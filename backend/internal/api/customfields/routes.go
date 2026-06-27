package customfields

import (
	"ttgo/internal/api/routing"

	"github.com/go-pkgz/routegroup"
)

// Mount registers custom-field routes. Custom-field definitions are global
// schema (mandatory flags + SELECT option sets applied across all test cases),
// so create/delete are admin-only — the closest analogue to a migration (F-014).
func Mount(api *routegroup.Bundle, h *Handler, requireAuth routing.AuthMiddleware, requireAdmin routing.AdminMiddleware) {
	api.HandleFunc("GET /custom-fields", requireAuth("read", h.List))
	api.HandleFunc("POST /custom-fields", requireAdmin(h.Create))
	api.HandleFunc("DELETE /custom-fields/{id}", requireAdmin(h.Delete))
}
