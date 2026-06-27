package folders

import (
	"ttgo/internal/api/routing"

	"github.com/go-pkgz/routegroup"
)

func Mount(api *routegroup.Bundle, h *Handler, requireAuth routing.AuthMiddleware) {
	api.HandleFunc("GET /folders/tree", requireAuth("read", h.GetTree))
	api.HandleFunc("GET /folders/{id}", requireAuth("read", h.Get))
	api.HandleFunc("POST /folders", requireAuth("write", h.Create))
	api.HandleFunc("DELETE /folders/{id}", requireAuth("write", h.Delete))
	api.HandleFunc("POST /folders/bulk-delete", requireAuth("write", h.BulkDelete))
	api.HandleFunc("POST /folders/bulk-move", requireAuth("write", h.BulkMove))
	api.HandleFunc("PATCH /folders/{id}", requireAuth("write", h.Rename))
	api.HandleFunc("PATCH /folders/{id}/parent", requireAuth("write", h.Move))
}
