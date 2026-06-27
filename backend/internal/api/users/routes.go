package users

import (
	"ttgo/internal/api/routing"

	"github.com/go-pkgz/routegroup"
)

func Mount(api *routegroup.Bundle, h *Handler, requireAdmin routing.AdminMiddleware) {
	api.HandleFunc("GET /users", requireAdmin(h.List))
	api.HandleFunc("POST /users", requireAdmin(h.Create))
	api.HandleFunc("PATCH /users/{id}", requireAdmin(h.Update))
	api.HandleFunc("DELETE /users/{id}", requireAdmin(h.Delete))
	api.HandleFunc("POST /users/{id}/restore", requireAdmin(h.Restore))
}
