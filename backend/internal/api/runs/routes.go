package runs

import (
	"ttgo/internal/api/routing"

	"github.com/go-pkgz/routegroup"
)

func Mount(api *routegroup.Bundle, h *Handler, requireAuth routing.AuthMiddleware) {
	api.HandleFunc("GET /runs", requireAuth("read", h.GetTestRuns))
	api.HandleFunc("POST /runs", requireAuth("write", h.CreateTestRun))
	api.HandleFunc("GET /runs/{id}", requireAuth("read", h.GetTestRun))
	api.HandleFunc("DELETE /runs/{id}", requireAuth("write", h.DeleteTestRun))
	api.HandleFunc("POST /runs/bulk-delete", requireAuth("write", h.BulkDeleteTestRuns))
	api.HandleFunc("PUT /runs/{id}", requireAuth("write", h.UpdateTestRun))
	api.HandleFunc("POST /runs/{id}/complete", requireAuth("write", h.CompleteRun))
	api.HandleFunc("POST /runs/{id}/reopen", requireAuth("write", h.ReopenRun))
	api.HandleFunc("POST /runs/{id}/copy", requireAuth("write", h.CopyTestRun))
	api.HandleFunc("PATCH /runs/{id}/folder", requireAuth("write", h.AssignRunToFolder))
	api.HandleFunc("POST /runs/{id}/results", requireAuth("write", h.AddRunResult))
	api.HandleFunc("POST /runs/{id}/results/bulk-update", requireAuth("write", h.BulkUpdateRunResults))
	api.HandleFunc("PUT /runs/{id}/results/{result_id}", requireAuth("write", h.UpdateRunResult))
	api.HandleFunc("DELETE /runs/{id}/results/{result_id}", requireAuth("write", h.DeleteRunResult))
	api.HandleFunc("POST /runs/{id}/results/{result_id}/screenshots", requireAuth("write", h.UploadScreenshots))
	api.HandleFunc("GET /uploads/screenshots/{result_id}/{filename}", requireAuth("read", h.ServeScreenshot))
	api.HandleFunc("POST /runs/{id}/results/{result_id}/retry", requireAuth("write", h.RetryRunResult))
	api.HandleFunc("GET /runs/{id}/comments", requireAuth("read", h.ListRunComments))
	api.HandleFunc("POST /runs/{id}/comments", requireAuth("write", h.AddRunComment))
	api.HandleFunc("GET /runs/{id}/results/{result_id}/comments", requireAuth("read", h.ListResultComments))
	api.HandleFunc("POST /runs/{id}/results/{result_id}/comments", requireAuth("write", h.AddResultComment))
	api.HandleFunc("PUT /comments/{comment_id}", requireAuth("write", h.UpdateComment))
	api.HandleFunc("DELETE /comments/{comment_id}", requireAuth("write", h.DeleteComment))
	api.HandleFunc("GET /runs/{id}/defect-links", requireAuth("read", h.ListRunDefectLinks))
	api.HandleFunc("GET /runs/{id}/results/{result_id}/defect-links", requireAuth("read", h.ListResultDefectLinks))
	api.HandleFunc("POST /runs/{id}/results/{result_id}/defect-links", requireAuth("write", h.LinkResultDefect))
	api.HandleFunc("DELETE /runs/{id}/results/{result_id}/defect-links/{jiraKey}", requireAuth("write", h.UnlinkResultDefect))
	api.HandleFunc("POST /run-folders", requireAuth("write", h.CreateRunFolder))
	api.HandleFunc("GET /run-folders", requireAuth("read", h.GetRunFolders))
	api.HandleFunc("PATCH /run-folders/{id}", requireAuth("write", h.UpdateRunFolder))
	api.HandleFunc("PATCH /run-folders/{id}/order", requireAuth("write", h.ReorderRunFolder))
	api.HandleFunc("PATCH /run-folders/{id}/parent", requireAuth("write", h.MoveRunFolder))
	api.HandleFunc("POST /run-folders/{id}/copy", requireAuth("write", h.CopyRunFolder))
	api.HandleFunc("DELETE /run-folders/{id}", requireAuth("write", h.DeleteRunFolder))
}
