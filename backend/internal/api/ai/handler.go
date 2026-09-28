package ai

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
	"ttgo/internal/api/httpx"
	"ttgo/internal/api/websocket"
	"ttgo/pkg/tracker/callstats"
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

	// typesafeClients is the process-wide TypeSafe client factory (one shared rate limiter),
	// wired by the Server through SetTypeSafeClientFactoryShared. Nil builds unlimited clients.
	typesafeClients *typesafe.ClientFactory
	// newTypeSafeClient constructs the TypeSafe.ai client used by TestTypeSafeConnection. It
	// builds through typesafeClients; tests replace it via SetTypeSafeClientFactory.
	newTypeSafeClient func(apiKey string, timeout time.Duration) typesafe.Client
}

func NewHandler(s *store.Store, sanitizer *bluemonday.Policy) *Handler {
	h := &Handler{
		store:     s,
		sanitizer: sanitizer,
		inflight:  newInflightRegistry(),
	}
	h.newTypeSafeClient = func(k string, to time.Duration) typesafe.Client {
		return h.typesafeClients.New(k, typesafe.Options{Timeout: to})
	}
	return h
}

// SetTypeSafeClientFactoryShared wires the Server's process-wide TypeSafe client factory, so
// the connection test draws from the same rate limiter as failure analysis.
func (h *Handler) SetTypeSafeClientFactoryShared(f *typesafe.ClientFactory) {
	h.typesafeClients = f
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
func (h *Handler) analyzeSync(ctx context.Context, result *models.RunResult, userID string, acknowledged bool) (*models.RunResultAnalysis, error) {
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
	if warn := h.checkAnalysisBudget(failureanalysis.EstimateCallUSD(deps), 1, acknowledged); warn != nil {
		return nil, &budgetExceededError{payload: warn}
	}
	settings, err := h.store.GetFailureAnalysisSettings()
	if err != nil {
		return nil, err
	}
	actx := failureanalysis.BuildContext(h.store, result, time.Now())
	actx.RedactionEnabled = settings.RedactionEnabled
	actx.PromptTemplate = settings.PromptTemplate
	actx.ProviderModel = deps.NarrativeModel
	// A counter of its own, so the hedges this analysis fires are billed with it.
	ctx, calls := callstats.WithCounter(ctx)
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
	h.recordCosts(failureanalysis.CostEvents(models.AnalysisCostKindAnalysis, res, deps,
		failureanalysis.RefsFor(result.TestRunID, nil, row)))
	h.recordCosts(failureanalysis.HedgeCostEvents(calls.HedgePromptTokens(), deps,
		failureanalysis.RefsFor(result.TestRunID, nil, row)))
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

// aiOffMessage is the one wording every AI endpoint answers with while the master switch is off
// (failureanalysis.ErrAIDisabled carries the same text for the analysis paths).
const aiOffMessage = "AI features are switched off"

// aiEnabled reports the global AI master switch.
func (h *Handler) aiEnabled() (bool, error) {
	fs, err := h.store.GetOrCreateAIFeatureSettings()
	if err != nil {
		return false, err
	}
	return fs.Enabled, nil
}

// requireAI enforces the AI master switch for a handler that calls an AI vendor. It writes
// 409 {"error":"AI features are switched off"} (500 when the setting cannot be read) and
// returns false when the request must stop. Connection tests and the prompt preview are not
// gated: admins configure providers before switching AI on, and the preview calls nothing.
func (h *Handler) requireAI(w http.ResponseWriter) bool {
	on, err := h.aiEnabled()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return false
	}
	if !on {
		httpx.JSON(w, http.StatusConflict, map[string]string{"error": aiOffMessage})
		return false
	}
	return true
}

// recordCosts appends cost events to the ledger. A failed write is logged, never surfaced:
// the analysis or explanation it describes succeeded.
func (h *Handler) recordCosts(events []*models.AIAnalysisCostEvent) {
	for _, ev := range events {
		if err := h.store.RecordAnalysisCostEvent(ev); err != nil {
			slog.Warn("failure-analysis: cost event not recorded", "kind", ev.Kind, "engine", ev.Engine, "err", err)
		}
	}
}

func ptrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
