package backups

import (
	"ttgo/internal/api/routing"

	"github.com/go-pkgz/routegroup"
)

func Mount(api *routegroup.Bundle, m *Manager, requireAdmin routing.AdminMiddleware) {
	api.HandleFunc("GET /backups", requireAdmin(m.List))
	api.HandleFunc("POST /backups", requireAdmin(m.Create))
	api.HandleFunc("GET /backups/{id}", requireAdmin(m.Get))
	api.HandleFunc("DELETE /backups/{id}", requireAdmin(m.Delete))
	api.HandleFunc("GET /backups/{id}/download", requireAdmin(m.Download))
	api.HandleFunc("POST /backups/{id}/restore", requireAdmin(m.Restore))
	api.HandleFunc("POST /backups/upload-restore", requireAdmin(m.UploadRestore))
	api.HandleFunc("GET /backup-schedule", requireAdmin(m.GetSchedule))
	api.HandleFunc("PUT /backup-schedule", requireAdmin(m.UpdateSchedule))
	api.HandleFunc("GET /maintenance-status", m.MaintenanceStatus)
}
