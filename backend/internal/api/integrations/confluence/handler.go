package confluence

import (
	"ttgo/internal/api/websocket"
	"ttgo/pkg/tracker/store"

	"github.com/microcosm-cc/bluemonday"
)

const maxExternalResponseSize = 10 << 20 // 10 MB for external API responses.

type Handler struct {
	store     *store.Store
	hub       *websocket.Hub
	sanitizer *bluemonday.Policy
}

func NewHandler(s *store.Store, hub *websocket.Hub, sanitizer *bluemonday.Policy) *Handler {
	return &Handler{
		store:     s,
		hub:       hub,
		sanitizer: sanitizer,
	}
}
