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

// llmKeyUnreadableReason is why the LLM is left out when the default provider's stored key can't
// be decrypted (frontend/src/utils/analysisFlow.js shows the same words).
const llmKeyUnreadableReason = "the default LLM provider's stored key can't be decrypted — re-enter it"

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
//   - a default LLM key that can't be decrypted: a manual run the LLM must decide still runs and
//     records each attempt as failed with category configuration; Explain, and runs TypeSafe
//     decides, leave the LLM out with llmKeyUnreadableReason; an automatic run with nothing else
//     to decide fails resolution with the key error;
//   - every TypeSafe client is built through tsf, the process-wide factory with the shared
//     rate limiter (nil = unlimited, for tests);
//   - the narrative transfer check (JobDeps.Transfer) shares semantic grouping's client and is on
//     only with semantic grouping and dedup;
//   - auto-apply (spec §3.4) is resolved once per job (and per Explain): on only with the setting on,
//     a usable TypeSafe client deciding and the accuracy gate open at the configured threshold;
//     paused when the gate is closed;
//   - an attached LLM is wrapped with the per-call timeout and optional hedge (applyLLMLatency);
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
				keyUnreadable := llm.Classify(perr) == llm.ErrCatConfiguration
				switch {
				case perr == nil:
					deps.Narrative, deps.NarrativeModel = p, cfg.ModelName
					// The prices in force now price every call this job makes (cost ledger).
					deps.Pricing.LLMProviderID = cfg.ID
					deps.Pricing.LLMPromptPerMTok, deps.Pricing.LLMCompletionPerMTok = cfg.PromptPricePerMTok, cfg.CompletionPricePerMTok
				case keyUnreadable && deps.Decider == nil && !explain && trigger == models.RunAnalysisJobTriggerManual:
					// The LLM must decide but its stored key can't be decrypted. Attach a provider that
					// fails every call, so each attempt is recorded as failed with category configuration
					// and the card says what to fix, instead of the job failing as a whole.
					deps.Narrative, deps.NarrativeModel = llm.NewUnavailableProvider(perr), cfg.ModelName
				case keyUnreadable && (deps.Decider != nil || explain):
					slog.Warn("failure-analysis: default LLM provider key can't be decrypted; re-enter it in Settings", "err", perr)
					deps.LLMUnavailableReason = llmKeyUnreadableReason
				case deps.Decider == nil:
					return deps, perr
				default:
					slog.Warn("failure-analysis: default LLM provider is misconfigured; TypeSafe decides alone", "err", perr)
					deps.LLMUnavailableReason = "the default LLM provider is misconfigured"
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
		if deps.Narrative != nil {
			applyLLMLatency(st, &deps)
		}
		// Few-shot examples go to both engines. They are optional context, so a settings read
		// failure leaves them off instead of failing the job. The transfer check only has
		// semantic clones to look at when dedup is on. Auto-apply is resolved for every trigger,
		// Explain included: a group Explain maintains the semantic clones' labels from the fits it
		// computes (R10).
		deps.AutoApplyState = models.AutoApplyStateOff
		if fa, err := st.GetFailureAnalysisSettings(); err != nil {
			slog.Warn("failure-analysis: settings could not be loaded; no few-shot examples, no transfer check and no auto-apply", "err", err)
			deps.Transfer = nil
		} else {
			deps.FewShotExamples = fa.FewShotExamples
			if !fa.DedupEnabled {
				deps.Transfer = nil
			}
			resolveAutoApply(st, fa, &deps)
		}
		return deps, nil
	}
}

// applyLLMLatency bounds every failure-analysis LLM call — generative decisions, fallback,
// takeover, narration and Explain all go through deps.Narrative — with the per-call timeout
// and, when switched on, a hedged second request inside it (spec §B). Settings that cannot be
// read fall back to the defaults: 45 s, no hedging.
func applyLLMLatency(st *store.Store, deps *failureanalysis.JobDeps) {
	timeoutS, hedgeS := models.DefaultLLMCallTimeoutSeconds, 0
	if fas, err := st.GetFailureAnalysisSettings(); err != nil {
		slog.Warn("failure-analysis: settings unavailable; using the default LLM call timeout", "err", err)
	} else {
		timeoutS, hedgeS = fas.LLMCallTimeoutSeconds, fas.HedgeAfterSeconds
	}
	timeout := failureanalysis.LLMCallTimeoutFor(timeoutS)
	hedge := failureanalysis.HedgeAfterFor(hedgeS, timeout)
	deps.LLMCallTimeout, deps.HedgingOn = timeout, hedge > 0
	deps.Narrative = llm.WithCallTimeout(llm.WithHedge(deps.Narrative, hedge), timeout)
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

// newAutoAnalysis is what run completion consults: the gate, and the estimated cost of the
// automatic job at the route it would run now (planned groups × the per-call estimate).
func newAutoAnalysis(st *store.Store, resolve failureanalysis.DepsResolver) apiruns.AutoAnalysis {
	return apiruns.AutoAnalysis{
		Gate: newAutoAnalyzeGate(resolve),
		Estimate: func(failures []*models.RunResult) *float64 {
			deps, err := resolve(models.RunAnalysisJobTriggerAutoOnDone)
			if err != nil {
				return nil
			}
			settings, err := st.GetFailureAnalysisSettings()
			if err != nil {
				return nil
			}
			return failureanalysis.EstimateJobUSD(deps,
				failureanalysis.PlannedGroups(failures, settings.DedupEnabled, settings.MaxAnalysesPerRun))
		},
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
	deps.Pricing.TypeSafePerMTok = ts.PricePerMTok
	if ts.SemanticDedupEnabled {
		deps.Semantic = &failureanalysis.SemanticDeps{Client: client, Model: ts.Model}
		// The narrative transfer check asks the same client about the semantic clones grouping
		// creates; newAnalyzeDepsResolver drops it again when dedup is off.
		deps.Transfer = &failureanalysis.TransferDeps{Client: client, Model: ts.Model}
	}
	if !ts.VerdictEngineEnabled {
		return nil
	}
	deps.TypeSafeTimeout = time.Duration(ts.TimeoutSeconds) * time.Second
	if deps.TypeSafeTimeout <= 0 {
		deps.TypeSafeTimeout = 30 * time.Second // the TypeSafe client's own default
	}
	deps.Decider = failureanalysis.NewTypeSafeDecider(client, ts.Model)
	deps.DeciderModel = ts.Model
	return ts
}

// resolveAutoApply decides, once per job, whether results may be labelled by auto-apply
// (spec §3.4): the setting is on, a usable TypeSafe client decides, and the accuracy gate is open
// at the configured threshold. "Usable" is TypeSafeTimeout > 0: resolveTypeSafe sets it only on
// the path that built a real client, so a missing or undecryptable key (NewUnavailableDecider)
// records off, not on or paused. A gate that cannot be read pauses auto-apply rather than failing
// the job.
func resolveAutoApply(st *store.Store, fa *models.AIFailureAnalysisSettings, deps *failureanalysis.JobDeps) {
	deps.AutoApply = nil
	deps.AutoApplyState = models.AutoApplyStateOff
	if !fa.AutoApplyDefectType || deps.Decider == nil || deps.TypeSafeTimeout <= 0 {
		return
	}
	pct := fa.AutoApplyMinConfidence
	if models.ValidateAutoApplyMinConfidence(pct) != nil {
		pct = models.DefaultAutoApplyMinConfidence
	}
	minConfidence := float64(pct) / 100
	gate, err := st.AutoApplyGate(minConfidence)
	if err != nil {
		slog.Warn("failure-analysis: accuracy gate unavailable; auto-apply paused", "err", err)
		deps.AutoApplyState = models.AutoApplyStatePaused
		return
	}
	if !gate.Open {
		deps.AutoApplyState = models.AutoApplyStatePaused
		return
	}
	deps.AutoApply = &failureanalysis.AutoApplyDeps{MinConfidence: minConfidence}
	deps.AutoApplyState = models.AutoApplyStateOn
}
