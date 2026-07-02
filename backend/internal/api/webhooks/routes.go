package webhooks

import (
	"ttgo/internal/api/routing"

	"github.com/go-pkgz/routegroup"
)

func Mount(api *routegroup.Bundle, h *Handler, requireAuth routing.AuthMiddleware) {
	api.HandleFunc("POST /webhooks", requireAuth("write", h.Create))
	api.HandleFunc("GET /webhooks", requireAuth("read", h.List))
	api.HandleFunc("DELETE /webhooks/{id}", requireAuth("write", h.Delete))
	api.HandleFunc("POST /webhooks/{id}/rotate-secret", requireAuth("write", h.RotateSecret))
	api.HandleFunc("GET /webhooks/{id}/logs", requireAuth("read", h.Logs))
}
