package qtest

import (
	"ttgo/internal/api/routing"

	"github.com/go-pkgz/routegroup"
)

func Mount(api *routegroup.Bundle, h *Handler, requireAuth routing.AuthMiddleware, requireAdmin routing.AdminMiddleware) {
	api.HandleFunc("GET /settings/qtest", requireAdmin(h.GetConfig))
	api.HandleFunc("PUT /settings/qtest", requireAdmin(h.UpsertConfig))
	api.HandleFunc("POST /settings/qtest/test-connection", requireAdmin(h.TestConnection))
	api.HandleFunc("GET /qtest/projects", requireAuth("read", h.ListProjects))
	api.HandleFunc("GET /qtest/enabled-projects", requireAuth("read", h.ListEnabledProjects))
	api.HandleFunc("POST /qtest/enabled-projects", requireAdmin(h.AddEnabledProject))
	api.HandleFunc("POST /qtest/enabled-projects/remove", requireAdmin(h.RemoveEnabledProject))
	api.HandleFunc("POST /qtest/enabled-projects/set-default", requireAdmin(h.SetDefaultProject))
	api.HandleFunc("GET /qtest/modules", requireAuth("read", h.ListModules))
	api.HandleFunc("GET /qtest/test-cases", requireAuth("read", h.ListTestCases))
	api.HandleFunc("POST /qtest/import", requireAuth("write", h.Import))
	api.HandleFunc("POST /qtest/upload", requireAuth("write", h.Upload))
	api.HandleFunc("POST /qtest/upload-folder", requireAuth("write", h.UploadFolder))
	api.HandleFunc("POST /qtest/unlink-folder", requireAuth("write", h.UnlinkFolder))
	api.HandleFunc("POST /qtest/bulk-unlink", requireAuth("write", h.BulkUnlink))
	api.HandleFunc("POST /qtest/sync", requireAuth("write", h.Sync))
	api.HandleFunc("POST /qtest/batch-mappings", requireAuth("read", h.BatchGetMappings))
}
