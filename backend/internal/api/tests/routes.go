package tests

import (
	"ttgo/internal/api/routing"

	"github.com/go-pkgz/routegroup"
)

func Mount(api *routegroup.Bundle, h *Handler, requireAuth routing.AuthMiddleware) {
	api.HandleFunc("GET /tests", requireAuth("read", h.GetTests))
	api.HandleFunc("GET /tests/by-custom-field", requireAuth("read", h.GetTestByCustomField))
	api.HandleFunc("GET /tests/{id}", requireAuth("read", h.GetTest))
	api.HandleFunc("POST /tests", requireAuth("write", h.CreateTest))
	api.HandleFunc("PUT /tests/{id}", requireAuth("write", h.UpdateTest))
	api.HandleFunc("DELETE /tests/{id}", requireAuth("write", h.DeleteTest))
	api.HandleFunc("POST /tests/bulk-delete", requireAuth("write", h.BulkDeleteTests))
	api.HandleFunc("POST /tests/export", requireAuth("read", h.ExportTests))
	api.HandleFunc("POST /tests/{id}/categories", requireAuth("write", h.AssignCategory))
	api.HandleFunc("GET /tests/{id}/versions", requireAuth("read", h.ListVersions))
	api.HandleFunc("GET /tests/{id}/versions/{vid}", requireAuth("read", h.GetVersion))
	api.HandleFunc("POST /tests/{id}/versions/{vid}/restore", requireAuth("write", h.RestoreVersion))
	api.HandleFunc("GET /tests/{id}/executions", requireAuth("read", h.ListTestExecutions))
	api.HandleFunc("DELETE /tests/{id}/reverification-flag", requireAuth("write", h.DismissReverification))
	api.HandleFunc("GET /tests/{id}/defect-links", requireAuth("read", h.ListTestDefects))
	api.HandleFunc("POST /tests/{id}/defect-links", requireAuth("write", h.LinkTestDefect))
	api.HandleFunc("DELETE /tests/{id}/defect-links/{defect_id}", requireAuth("write", h.UnlinkTestDefect))
}
