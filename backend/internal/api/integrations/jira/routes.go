package jira

import (
	"ttgo/internal/api/routing"

	"github.com/go-pkgz/routegroup"
)

// Mount registers Jira routes. Writing the integration config (org-wide base URL
// + credentials, and the SSRF lever) is admin-only, matching qTest; reads stay at
// read scope (F-005).
func Mount(api *routegroup.Bundle, h *Handler, requireAuth routing.AuthMiddleware, requireAdmin routing.AdminMiddleware) {
	api.HandleFunc("GET /settings/jira", requireAuth("read", h.GetConfig))
	api.HandleFunc("PUT /settings/jira", requireAdmin(h.UpsertConfig))
	api.HandleFunc("GET /jira/ticket/{ticketId}", requireAuth("read", h.GetTicket))
	api.HandleFunc("POST /jira/search", requireAuth("read", h.Search))
}
