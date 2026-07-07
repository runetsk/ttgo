package auth

import (
	"ttgo/internal/api/routing"

	"github.com/go-pkgz/routegroup"
)

func Mount(api *routegroup.Bundle, h *Handler, requireAuth routing.AuthMiddleware) {
	api.HandleFunc("POST /auth/login", h.Login)
	api.HandleFunc("POST /auth/logout", h.Logout)
	api.HandleFunc("GET /auth/me", h.Me)
	api.HandleFunc("POST /auth/change-password", requireAuth("write", h.ChangePassword))
	api.HandleFunc("GET /auth/needs-setup", h.NeedsSetup)
	api.HandleFunc("POST /auth/setup", h.Setup)
}
