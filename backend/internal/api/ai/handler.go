package ai

import (
	"context"
	"fmt"
	"time"
	"ttgo/internal/api/websocket"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"

	"github.com/microcosm-cc/bluemonday"
)

type Handler struct {
	store     *store.Store
	sanitizer *bluemonday.Policy

	// ai-failure-analysis dependencies (optional — nil when no LLM provider configured).
	currentProvider func() (llm.Provider, string, error)
	broadcaster     *websocket.RunAnalysisBroadcaster

	// inflight tracks in-process generation runs so they can be cancelled
	// from another session (POST /ai-generations/{id}/cancel).
	inflight *inflightRegistry
}

func NewHandler(s *store.Store, sanitizer *bluemonday.Policy) *Handler {
	return &Handler{store: s, sanitizer: sanitizer, inflight: newInflightRegistry()}
}

// SetFailureAnalysisDeps wires in the provider resolver and broadcaster.
func (h *Handler) SetFailureAnalysisDeps(
	provider func() (llm.Provider, string, error),
	bc *websocket.RunAnalysisBroadcaster,
) {
	h.currentProvider = provider
	h.broadcaster = bc
}

// analyzeSync runs Analyze directly and persists the result.
func (h *Handler) analyzeSync(ctx context.Context, result *models.RunResult, userID string) (*models.RunResultAnalysis, error) {
	if h.currentProvider == nil {
		return nil, fmt.Errorf("no LLM provider configured")
	}
	provider, model, err := h.currentProvider()
	if err != nil || provider == nil {
		return nil, fmt.Errorf("llm provider unavailable: %w", err)
	}
	settings, err := h.store.GetFailureAnalysisSettings()
	if err != nil {
		return nil, err
	}
	// Assemble enrichment (history, linked defects/requirements, steps, env) from
	// the shared builder, then layer the active settings + resolved model onto it.
	// BuildContext does not set these fields, so applying them here is required or
	// redaction and the admin prompt template silently break.
	actx := failureanalysis.BuildContext(h.store, result, time.Now())
	actx.RedactionEnabled = settings.RedactionEnabled
	actx.PromptTemplate = settings.PromptTemplate
	actx.ProviderModel = model
	res, err := failureanalysis.Analyze(ctx, provider, actx)
	if err != nil {
		return nil, err
	}
	row, err := h.store.CreateAnalysis(&models.RunResultAnalysis{
		RunResultID:          result.ID,
		Verdict:              res.Verdict,
		Confidence:           res.Confidence,
		Summary:              res.Summary,
		NextAction:           res.NextAction,
		Rationale:            res.Rationale,
		RawResponse:          res.RawResponse,
		ModelName:            res.ModelName,
		TokenUsagePrompt:     res.TokenUsagePrompt,
		TokenUsageCompletion: res.TokenUsageCompletion,
		CreatedBy:            ptrOrNil(userID),
	})
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
