package ai

import (
	"context"
	"errors"
	"fmt"
	"time"
	"ttgo/internal/api/websocket"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"
	"ttgo/pkg/tracker/typesafe"

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

	// newTypeSafeClient constructs the TypeSafe.ai client used by TestTypeSafeConnection;
	// overridable in tests via SetTypeSafeClientFactory.
	newTypeSafeClient func(apiKey string, timeout time.Duration) typesafe.Client
}

func NewHandler(s *store.Store, sanitizer *bluemonday.Policy) *Handler {
	return &Handler{
		store:     s,
		sanitizer: sanitizer,
		inflight:  newInflightRegistry(),
		newTypeSafeClient: func(k string, to time.Duration) typesafe.Client {
			return typesafe.NewHTTPClient(k, typesafe.Options{Timeout: to})
		},
	}
}

// SetTypeSafeClientFactory replaces the client constructor (tests only).
func (h *Handler) SetTypeSafeClientFactory(f func(apiKey string, timeout time.Duration) typesafe.Client) {
	h.newTypeSafeClient = f
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
	if errors.Is(err, failureanalysis.ErrAIDisabled) {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("llm provider unavailable: %w", err)
	}
	if !deps.CanAnalyze() {
		if deps.LLMUnavailableReason != "" {
			return nil, fmt.Errorf("cannot analyze: %s", deps.LLMUnavailableReason)
		}
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
	var attemptErr error
	if err != nil {
		if ctx.Err() != nil {
			return nil, err // the request was abandoned; nothing was attempted to completion
		}
		// Record the failed attempt like the batch worker does, so the version history shows
		// it with the engine, model, category and tokens it cost; then report the failure.
		attemptErr = err
		res = failureanalysis.FailedResult(err, deps.Analyze())
	}
	row := failureanalysis.AnalysisRowFrom(res, result.ID)
	row.CreatedBy = ptrOrNil(userID)
	row, err = h.store.CreateAnalysis(row)
	if err != nil {
		return nil, err
	}
	if h.broadcaster != nil {
		h.broadcaster.BroadcastRunResultAnalysisCreated(row, result.TestRunID)
	}
	if attemptErr != nil {
		return row, attemptErr
	}
	return row, nil
}

// aiEnabled reports the global AI master switch; failure analysis enforces it server-side.
func (h *Handler) aiEnabled() (bool, error) {
	fs, err := h.store.GetOrCreateAIFeatureSettings()
	if err != nil {
		return false, err
	}
	return fs.Enabled, nil
}

func ptrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
