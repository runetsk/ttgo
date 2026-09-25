package runs

import (
	"context"
	apiws "ttgo/internal/api/websocket"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"
)

type RunCompletedNotifier func(context.Context, *models.TestRun)

// AutoAnalyzeGate reports whether a completed run's failures may be queued for automatic
// analysis now, and why not. The api package implements it with the failure-analysis resolver.
type AutoAnalyzeGate func() (ok bool, reason string)

type Handler struct {
	store              *store.Store
	hub                *apiws.Hub
	notifyRunCompleted RunCompletedNotifier
	autoAnalyze        AutoAnalyzeGate
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

func (h *Handler) canAutoAnalyze() (bool, string) {
	if h.autoAnalyze == nil {
		return false, "no automatic-analysis gate is configured"
	}
	return h.autoAnalyze()
}
