package categories

import (
	"ttgo/internal/api/routing"

	"github.com/go-pkgz/routegroup"
)

func Mount(api *routegroup.Bundle, h *Handler, requireAuth routing.AuthMiddleware) {
	api.HandleFunc("GET /categories", requireAuth("read", h.List))
	api.HandleFunc("POST /categories", requireAuth("write", h.Create))
	api.HandleFunc("DELETE /categories/{id}", requireAuth("write", h.Delete))
	api.HandleFunc("POST /categories/bulk-delete", requireAuth("write", h.BulkDelete))
}
