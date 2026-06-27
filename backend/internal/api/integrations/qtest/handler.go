package qtest

import (
	"ttgo/internal/api/websocket"
	"ttgo/pkg/tracker/store"
)

type Handler struct {
	store *store.Store
	hub   *websocket.Hub
}

func NewHandler(s *store.Store, hub *websocket.Hub) *Handler {
	return &Handler{
		store: s,
		hub:   hub,
	}
}
