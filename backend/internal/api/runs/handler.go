package runs

import (
	"context"
	apiws "ttgo/internal/api/websocket"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"
)

type RunCompletedNotifier func(context.Context, *models.TestRun)

type Handler struct {
	store              *store.Store
	hub                *apiws.Hub
	notifyRunCompleted RunCompletedNotifier
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
