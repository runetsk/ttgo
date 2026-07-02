package defects

import (
	"ttgo/internal/api/routing"

	"github.com/go-pkgz/routegroup"
)

func Mount(api *routegroup.Bundle, h *Handler, requireAuth routing.AuthMiddleware) {
	api.HandleFunc("GET /defects", requireAuth("read", h.List))
	api.HandleFunc("GET /defects/{id}/tests", requireAuth("read", h.AffectedTests))
	api.HandleFunc("POST /defects", requireAuth("write", h.Create))
	api.HandleFunc("PATCH /defects/{id}", requireAuth("write", h.Update))
	api.HandleFunc("DELETE /defects/{id}", requireAuth("write", h.Delete))
}
