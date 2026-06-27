package requirements

import (
	"ttgo/internal/api/routing"

	"github.com/go-pkgz/routegroup"
)

func Mount(api *routegroup.Bundle, h *Handler, requireAuth routing.AuthMiddleware) {
	api.HandleFunc("GET /requirements", requireAuth("read", h.ListRequirements))
	api.HandleFunc("POST /requirements", requireAuth("write", h.CreateRequirement))
	api.HandleFunc("GET /requirements/{id}", requireAuth("read", h.GetRequirement))
	api.HandleFunc("PUT /requirements/{id}", requireAuth("write", h.UpdateRequirement))
	api.HandleFunc("DELETE /requirements/{id}", requireAuth("write", h.DeleteRequirement))
	api.HandleFunc("POST /requirements/bulk-delete", requireAuth("write", h.BulkDeleteRequirements))
	api.HandleFunc("GET /requirements/{id}/children", requireAuth("read", h.ListChildren))
	api.HandleFunc("POST /requirements/{id}/links", requireAuth("write", h.CreateLink))
	api.HandleFunc("DELETE /requirements/{id}/links/{testCaseId}", requireAuth("write", h.DeleteLink))
	api.HandleFunc("GET /traceability", requireAuth("read", h.TraceabilityMatrix))
	api.HandleFunc("POST /requirements/import", requireAuth("write", h.ImportRequirement))
	api.HandleFunc("POST /requirements/bulk-import", requireAuth("write", h.BulkImport))
	api.HandleFunc("POST /requirements/{id}/resync", requireAuth("write", h.Resync))
	api.HandleFunc("POST /requirements/{id}/resync/resolve", requireAuth("write", h.ResyncResolve))
	api.HandleFunc("POST /requirements/{id}/unlink", requireAuth("write", h.Unlink))
	api.HandleFunc("POST /requirements/{id}/post-to-jira", requireAuth("write", h.PostToJira))
	api.HandleFunc("GET /tests/{id}/requirements", requireAuth("read", h.ListTestCaseRequirements))
}
