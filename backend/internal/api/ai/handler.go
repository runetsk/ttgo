package ai

import (
	"context"
	"fmt"
	"time"
	"ttgo/internal/api/websocket"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/failureanalysis/worker"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"

	"github.com/microcosm-cc/bluemonday"
)

type Handler struct {
	store     *store.Store
	sanitizer *bluemonday.Policy

	// ai-failure-analysis dependencies (nil until StartFailureAnalysisWorker wires them).
	resolveDeps failureanalysis.DepsResolver
	broadcaster *websocket.RunAnalysisBroadcaster

	// inflight tracks in-process generation runs so they can be cancelled
	// from another session (POST /ai-generations/{id}/cancel).
	inflight *inflightRegistry
}

func NewHandler(s *store.Store, sanitizer *bluemonday.Policy) *Handler {
	return &Handler{store: s, sanitizer: sanitizer, inflight: newInflightRegistry()}
}

// SetFailureAnalysisDeps wires in the per-job dependency resolver and broadcaster.
func (h *Handler) SetFailureAnalysisDeps(resolver failureanalysis.DepsResolver, bc *websocket.RunAnalysisBroadcaster) {
	h.resolveDeps = resolver
	h.broadcaster = bc
}

// analyzeSync runs Analyze directly for one result (manual trigger) and persists the row.
func (h *Handler) analyzeSync(ctx context.Context, result *models.RunResult, userID string) (*models.RunResultAnalysis, error) {
	if h.resolveDeps == nil {
		return nil, fmt.Errorf("no LLM provider configured")
	}
	deps, err := h.resolveDeps(models.RunAnalysisJobTriggerManual)
	if err != nil {
		return nil, fmt.Errorf("llm provider unavailable: %w", err)
	}
	if deps.Narrative == nil {
		return nil, fmt.Errorf("no LLM provider configured")
	}
	settings, err := h.store.GetFailureAnalysisSettings()
	if err != nil {
		return nil, err
	}
	actx := failureanalysis.BuildContext(h.store, result, time.Now())
	actx.RedactionEnabled = settings.RedactionEnabled
	actx.PromptTemplate = settings.PromptTemplate
	actx.ProviderModel = deps.NarrativeModel
	res, err := failureanalysis.Analyze(ctx, deps.Analyze(), actx)
	if err != nil {
		return nil, err
	}
	row := worker.AnalysisRowFrom(res, result.ID)
	row.CreatedBy = ptrOrNil(userID)
	row, err = h.store.CreateAnalysis(row)
	if err != nil {
		return nil, err
	}
	if h.broadcaster != nil {
		h.broadcaster.BroadcastRunResultAnalysisCreated(row, result.TestRunID)
	}
	return row, nil
}

func ptrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
