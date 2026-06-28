package confluence

import (
	"ttgo/internal/api/routing"

	"github.com/go-pkgz/routegroup"
)

// Mount registers Confluence routes. Writing the integration config (org-wide
// base URL + credentials) is admin-only, matching Jira; reads stay at read
// scope (F-005).
func Mount(api *routegroup.Bundle, h *Handler, requireAuth routing.AuthMiddleware, requireAdmin routing.AdminMiddleware) {
	api.HandleFunc("GET /settings/confluence", requireAuth("read", h.GetConfig))
	api.HandleFunc("PUT /settings/confluence", requireAdmin(h.UpsertConfig))
	api.HandleFunc("GET /confluence/spaces", requireAuth("read", h.ListSpaces))
	api.HandleFunc("GET /confluence/pages", requireAuth("read", h.ListPages))
	api.HandleFunc("GET /confluence/pages/{pageId}", requireAuth("read", h.GetPage))
	api.HandleFunc("GET /confluence/pages/{pageId}/children", requireAuth("read", h.ListChildPages))
}
