package runs

import (
	"context"
	"log/slog"
	"time"

	apiws "ttgo/internal/api/websocket"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"
)

type RunCompletedNotifier func(context.Context, *models.TestRun)

// AutoAnalyzeGate reports whether a completed run's failures may be queued for automatic
// analysis now, and why not. The api package implements it with the failure-analysis resolver.
type AutoAnalyzeGate func() (ok bool, reason string)

// AutoAnalyzeEstimate is the estimated cost (USD) of automatically analyzing these failures with
// the route an automatic job would run now; nil when nothing on that route is priced.
type AutoAnalyzeEstimate func(failures []*models.RunResult) *float64

// AutoAnalysis is what run completion consults before queueing automatic analysis.
type AutoAnalysis struct {
	Gate     AutoAnalyzeGate
	Estimate AutoAnalyzeEstimate
}

type Handler struct {
	store              *store.Store
	hub                *apiws.Hub
	notifyRunCompleted RunCompletedNotifier
	autoAnalyze        AutoAnalyzeGate
	autoEstimate       AutoAnalyzeEstimate
}

func NewHandler(s *store.Store, hub *apiws.Hub) *Handler {
	return &Handler{
		store: s,
		hub:   hub,
	}
}

func NewHandlerWithNotifier(s *store.Store, hub *apiws.Hub, notifyRunCompleted RunCompletedNotifier) *Handler {
	return &Handler{
		store:              s,
		hub:                hub,
		notifyRunCompleted: notifyRunCompleted,
	}
}

// WithAutoAnalyzeGate sets the check run completion makes before queueing automatic analysis.
func (h *Handler) WithAutoAnalyzeGate(g AutoAnalyzeGate) *Handler {
	h.autoAnalyze = g
	return h
}

// WithAutoAnalysis sets the gate and the cost estimate run completion uses.
func (h *Handler) WithAutoAnalysis(a AutoAnalysis) *Handler {
	h.autoAnalyze, h.autoEstimate = a.Gate, a.Estimate
	return h
}

// autoAnalysisOverBudget reports whether automatically analyzing failures would take the month
// over the monthly AI budget (spent so far + this run's estimate, the same threshold a manual
// start uses), with the figures for the skip record. Budgets are soft: no monthly budget, an
// unpriced route or a read error lets the analysis be queued.
func (h *Handler) autoAnalysisOverBudget(failures []*models.RunResult) (over bool, estimate, spent, budget float64) {
	if h.autoEstimate == nil {
		return false, 0, 0, 0
	}
	b, err := h.store.GetOrCreateAIBudgetSettings()
	if err != nil || b.MonthlyUSD <= 0 {
		return false, 0, 0, 0
	}
	est := h.autoEstimate(failures)
	if est == nil {
		return false, 0, 0, 0
	}
	spent, err = h.store.SumEstimatedCostSince(store.MonthStartUTC(time.Now()))
	if err != nil {
		return false, 0, 0, 0
	}
	return spent+*est > b.MonthlyUSD, *est, spent, b.MonthlyUSD
}

// recordSkippedAutoAnalysis stores the skip as a terminal job and tells an open run page, whose
// banner then offers "Run anyway".
func (h *Handler) recordSkippedAutoAnalysis(ctx context.Context, runID string, estimate, spent, budget float64) {
	job, err := h.store.CreateSkippedAnalysisJob(runID, models.RunAnalysisJobSkipReasonBudget, estimate, spent, budget)
	if err != nil {
		slog.WarnContext(ctx, "ai-failure-analysis: auto skip not recorded", "run_id", runID, "err", err)
		return
	}
	slog.InfoContext(ctx, "ai-failure-analysis: auto skipped — over the monthly budget", "run_id", runID,
		"estimate_usd", estimate, "spent_usd", spent, "budget_usd", budget)
	if h.hub != nil {
		(&apiws.RunAnalysisBroadcaster{Hub: h.hub}).BroadcastRunAnalysisCompleted(job, 0)
	}
}

func (h *Handler) canAutoAnalyze() (bool, string) {
	if h.autoAnalyze == nil {
		return false, "no automatic-analysis gate is configured"
	}
	return h.autoAnalyze()
}
