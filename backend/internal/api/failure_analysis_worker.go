package api

import (
	"context"
	"log/slog"
	"time"

	apiai "ttgo/internal/api/ai"
	apiruns "ttgo/internal/api/runs"
	apiws "ttgo/internal/api/websocket"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/failureanalysis/worker"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"
	"ttgo/pkg/tracker/typesafe"
)

// StartFailureAnalysisWorker constructs and runs the background worker for run_analysis_jobs.
func (s *Server) StartFailureAnalysisWorker(ctx context.Context) {
	resolver := s.analyzeDepsResolver()
	bc := &apiws.RunAnalysisBroadcaster{Hub: s.Hub}
	s.ensureAIHandler().SetFailureAnalysisDeps(resolver, bc)
	w := worker.NewWorker(s.store, resolver, bc, 3*time.Second)
	go w.Run(ctx)
}

// analyzeDepsResolver builds the failure-analysis resolver on the Server's shared TypeSafe
// client factory, so the worker, the run-completion gate, sync analyze and Explain all draw
// from one rate limiter.
func (s *Server) analyzeDepsResolver() failureanalysis.DepsResolver {
	return newAnalyzeDepsResolver(s.store, s.typesafeClients)
}

// ensureAIHandler creates the shared AI handler once, wired to the same TypeSafe client
// factory, so the settings connection test shares the limiter too.
func (s *Server) ensureAIHandler() *apiai.Handler {
	if s.aiHandler == nil {
		s.aiHandler = apiai.NewHandler(s.store, s.sanitizer)
		s.aiHandler.SetTypeSafeClientFactoryShared(s.typesafeClients)
	}
	return s.aiHandler
}

// newAnalyzeDepsResolver builds JobDeps from live settings on every call (spec §4 "Resolver"),
// enforcing consent at execution time rather than only when a job is queued:
//   - the AI master switch stops everything (ErrAIDisabled);
//   - TypeSafe is resolved first; any TypeSafe setup problem (missing or undecryptable key)
//     is logged and leaves Decider/Semantic nil, so the LLM decides as before;
//   - the LLM is attached only when a route needs it: no TypeSafe decider, explanations on,
//     a takeover threshold, or the fallback. With none of them, analysis is TypeSafe-only and
//     never touches the LLM provider, so a broken or missing LLM configuration cannot block it;
//   - an automatic job uses the LLM only when that provider is itself approved for automatic
//     analysis, checked now, since the default provider may have changed since the job was queued;
//   - a broken LLM configuration fails the job only when the LLM would have to decide;
//   - every TypeSafe client is built through tsf, the process-wide factory with the shared
//     rate limiter (nil = unlimited, for tests);
//   - the settings page draws these rules (frontend/src/utils/analysisFlow.js); change both together.
func newAnalyzeDepsResolver(st *store.Store, tsf *typesafe.ClientFactory) failureanalysis.DepsResolver {
	return func(trigger string) (failureanalysis.JobDeps, error) {
		var deps failureanalysis.JobDeps
		if fs, err := st.GetOrCreateAIFeatureSettings(); err != nil {
			return deps, err
		} else if !fs.Enabled {
			return deps, failureanalysis.ErrAIDisabled
		}

		// An explicit Explain is a manual action that exists to call the LLM.
		explain := trigger == failureanalysis.TriggerExplain
		if explain {
			trigger = models.RunAnalysisJobTriggerManual
		}
		ts := resolveTypeSafe(st, tsf, trigger, &deps)

		needLLM := explain || deps.Decider == nil ||
			(ts != nil && (ts.NarrativeEnabled || ts.EscalateBelowPct > 0 || ts.LLMFallbackEnabled))
		if needLLM {
			cfg, err := st.GetDefaultProviderConfig()
			switch {
			case err != nil:
				if deps.Decider == nil {
					return deps, err
				}
				slog.Warn("failure-analysis: default LLM provider could not be loaded; TypeSafe decides alone", "err", err)
				deps.LLMUnavailableReason = "the default LLM provider could not be loaded"
			case cfg == nil:
				deps.LLMUnavailableReason = "no default LLM provider is configured"
			case trigger != models.RunAnalysisJobTriggerManual && !cfg.AllowAutoFailureAnalysis:
				deps.LLMUnavailableReason = "the default LLM provider is not approved for automatic analysis"
			default:
				p, perr := llm.NewProvider(cfg)
				if perr != nil {
					if deps.Decider == nil {
						return deps, perr
					}
					slog.Warn("failure-analysis: default LLM provider is misconfigured; TypeSafe decides alone", "err", perr)
					deps.LLMUnavailableReason = "the default LLM provider is misconfigured"
				} else {
					deps.Narrative, deps.NarrativeModel = p, cfg.ModelName
				}
			}
		}

		if deps.Decider != nil && ts != nil {
			deps.NarrativeSkipped = !ts.NarrativeEnabled
			deps.NoLLMFallback = !ts.LLMFallbackEnabled
			// Below this verdict confidence the LLM decides instead (needs both engines).
			if deps.Narrative != nil && ts.EscalateBelowPct > 0 {
				deps.EscalateBelow = float64(ts.EscalateBelowPct) / 100
			}
		}
		return deps, nil
	}
}

// newAutoAnalyzeGate answers, at run completion, whether an automatic analysis could run now:
// the resolver the worker uses, asked for the completion trigger. It builds clients but makes no
// network call. The worker resolves again when the job runs, so later setting changes still apply.
func newAutoAnalyzeGate(resolve failureanalysis.DepsResolver) apiruns.AutoAnalyzeGate {
	return func() (bool, string) {
		deps, err := resolve(models.RunAnalysisJobTriggerAutoOnDone)
		if err != nil {
			return false, err.Error()
		}
		if !deps.CanAnalyze() {
			if deps.LLMUnavailableReason != "" {
				return false, deps.LLMUnavailableReason
			}
			return false, "nothing is configured to analyze"
		}
		return true, ""
	}
}

// resolveTypeSafe attaches the TypeSafe decider and semantic grouping when they are enabled,
// consented for this trigger and keyed. It returns the settings when TypeSafe can decide.
func resolveTypeSafe(st *store.Store, tsf *typesafe.ClientFactory, trigger string, deps *failureanalysis.JobDeps) *models.TypeSafeSettings {
	ts, err := st.GetTypeSafeSettings()
	if err != nil {
		slog.Warn("typesafe: settings unavailable, running without TypeSafe", "err", err)
		return nil
	}
	permitted := ts.Enabled && (trigger == models.RunAnalysisJobTriggerManual || ts.AllowAutoFailureAnalysis)
	if !permitted || (!ts.VerdictEngineEnabled && !ts.SemanticDedupEnabled) {
		return nil
	}
	key, err := st.TypeSafeAPIKey()
	if err != nil || key == "" {
		slog.Warn("typesafe: API key missing or undecryptable; re-enter it in Settings", "err", err)
		if !ts.VerdictEngineEnabled {
			return nil // semantic grouping only: skip it, the LLM decides as configured
		}
		// TypeSafe is the configured decider but cannot run. Treat it as unavailable rather
		// than as not configured, so the LLM-fallback switch decides whether the LLM may see
		// the failure (off: every attempt is recorded as failed with category configuration).
		deps.Decider = failureanalysis.NewUnavailableDecider(&typesafe.Error{
			Category: typesafe.CategoryConfiguration,
			Message:  "TypeSafe.ai API key missing or undecryptable; re-enter it in Settings",
		})
		deps.DeciderModel = ts.Model
		return ts
	}
	client := tsf.New(key, typesafe.Options{Timeout: time.Duration(ts.TimeoutSeconds) * time.Second})
	if ts.SemanticDedupEnabled {
		deps.Semantic = &failureanalysis.SemanticDeps{Client: client, Model: ts.Model}
	}
	if !ts.VerdictEngineEnabled {
		return nil
	}
	deps.Decider = failureanalysis.NewTypeSafeDecider(client, ts.Model)
	deps.DeciderModel = ts.Model
	return ts
}
