package api

import (
	"context"
	"time"

	apiai "ttgo/internal/api/ai"
	apiws "ttgo/internal/api/websocket"
	"ttgo/pkg/tracker/failureanalysis/worker"
	"ttgo/pkg/tracker/llm"
)

// StartFailureAnalysisWorker constructs and runs a background worker that
// processes queued run_analysis_jobs. Returns immediately; the worker runs
// in its own goroutine until ctx is cancelled.
func (s *Server) StartFailureAnalysisWorker(ctx context.Context) {
	providerFn := func() (llm.Provider, string, error) {
		cfg, err := s.store.GetDefaultProviderConfig()
		if err != nil || cfg == nil {
			return nil, "", err
		}
		p, err := llm.NewProvider(cfg)
		return p, cfg.ModelName, err
	}
	bc := &apiws.RunAnalysisBroadcaster{Hub: s.Hub}

	// Ensure the AI handler exists and is wired with provider + broadcaster.
	if s.aiHandler == nil {
		s.aiHandler = apiai.NewHandler(s.store, s.sanitizer)
	}
	s.aiHandler.SetFailureAnalysisDeps(providerFn, bc)

	// Worker resolves its provider lazily each job so admin changes take effect
	// without a restart.
	w := worker.NewWorker(s.store, llmLazy{fn: providerFn}, bc, 3*time.Second)
	go w.Run(ctx)
}

// llmLazy wraps a provider factory as an llm.Provider. Each Chat call
// re-resolves the underlying provider so admins can swap the default at
// runtime.
type llmLazy struct {
	fn func() (llm.Provider, string, error)
}

func (l llmLazy) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	p, model, err := l.fn()
	if err != nil || p == nil {
		return nil, err
	}
	if req.Model == "" {
		req.Model = model
	}
	return p.Chat(ctx, req)
}
