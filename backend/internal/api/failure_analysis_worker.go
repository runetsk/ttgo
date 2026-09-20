package api

import (
	"context"
	"log/slog"
	"time"

	apiai "ttgo/internal/api/ai"
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
	resolver := newAnalyzeDepsResolver(s.store)
	bc := &apiws.RunAnalysisBroadcaster{Hub: s.Hub}
	if s.aiHandler == nil {
		s.aiHandler = apiai.NewHandler(s.store, s.sanitizer)
	}
	s.aiHandler.SetFailureAnalysisDeps(resolver, bc)
	w := worker.NewWorker(s.store, resolver, bc, 3*time.Second)
	go w.Run(ctx)
}

// newAnalyzeDepsResolver builds JobDeps from live settings on every call (spec §4 "Resolver").
// A generative-provider error is fatal for the job; any TypeSafe setup problem (missing or
// undecryptable key included) is logged and leaves Decider/Semantic nil so the job runs
// generatively — the promised fallback.
func newAnalyzeDepsResolver(st *store.Store) failureanalysis.DepsResolver {
	return func(trigger string) (failureanalysis.JobDeps, error) {
		var deps failureanalysis.JobDeps
		cfg, err := st.GetDefaultProviderConfig()
		if err != nil {
			return deps, err
		}
		if cfg != nil {
			p, perr := llm.NewProvider(cfg)
			if perr != nil {
				return deps, perr
			}
			deps.Narrative, deps.NarrativeModel = p, cfg.ModelName
		}

		ts, err := st.GetTypeSafeSettings()
		if err != nil {
			slog.Warn("typesafe: settings unavailable, running without TypeSafe", "err", err)
			return deps, nil
		}
		permitted := ts.Enabled && (trigger == models.RunAnalysisJobTriggerManual || ts.AllowAutoFailureAnalysis)
		if !permitted || (!ts.VerdictEngineEnabled && !ts.SemanticDedupEnabled) {
			return deps, nil
		}
		key, err := st.TypeSafeAPIKey()
		if err != nil || key == "" {
			slog.Warn("typesafe: API key missing or undecryptable; re-enter it in Settings, running without TypeSafe", "err", err)
			return deps, nil
		}
		client := typesafe.NewHTTPClient(key, typesafe.Options{Timeout: time.Duration(ts.TimeoutSeconds) * time.Second})
		if ts.VerdictEngineEnabled {
			deps.Decider = failureanalysis.NewTypeSafeDecider(client, ts.Model)
		}
		if ts.SemanticDedupEnabled {
			deps.Semantic = &failureanalysis.SemanticDeps{Client: client, Model: ts.Model}
		}
		return deps, nil
	}
}
