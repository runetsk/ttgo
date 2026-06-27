package tokens

import (
	"ttgo/internal/api/routing"

	"github.com/go-pkgz/routegroup"
)

// Mount registers token routes. Token lifecycle (create/list/delete) is
// admin-only: a write-scoped Bearer token must not be able to mint or revoke
// API credentials, nor enumerate the credential inventory (F-004).
func Mount(api *routegroup.Bundle, h *Handler, requireAdmin routing.AdminMiddleware) {
	api.HandleFunc("POST /tokens", requireAdmin(h.Create))
	api.HandleFunc("GET /tokens", requireAdmin(h.List))
	api.HandleFunc("DELETE /tokens/{id}", requireAdmin(h.Delete))
}
