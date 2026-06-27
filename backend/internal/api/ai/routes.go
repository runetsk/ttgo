package ai

import (
	"ttgo/internal/api/routing"

	"github.com/go-pkgz/routegroup"
)

func Mount(api *routegroup.Bundle, h *Handler, requireAuth routing.AuthMiddleware, requireAdmin routing.AdminMiddleware) {
	api.HandleFunc("GET /settings/llm-providers", requireAuth("read", h.ListProviders))
	api.HandleFunc("POST /settings/llm-providers", requireAdmin(h.CreateProvider))
	api.HandleFunc("PUT /settings/llm-providers/{id}", requireAdmin(h.UpdateProvider))
	api.HandleFunc("DELETE /settings/llm-providers/{id}", requireAdmin(h.DeleteProvider))
	api.HandleFunc("POST /settings/llm-providers/{id}/test", requireAdmin(h.TestConnection))
	api.HandleFunc("POST /settings/llm-providers/{id}/set-default", requireAdmin(h.SetDefaultProvider))
	api.HandleFunc("GET /settings/ai-features", requireAuth("read", h.GetAIFeatureSettings))
	api.HandleFunc("PUT /settings/ai-features", requireAdmin(h.UpdateAIFeatureSettings))
	api.HandleFunc("GET /settings/ai-gen-coverage", requireAuth("read", h.GetCoverageConfig))
	api.HandleFunc("PUT /settings/ai-gen-coverage", requireAdmin(h.UpdateCoverageConfig))
	api.HandleFunc("GET /settings/ai-gen-template", requireAuth("read", h.GetTemplate))
	api.HandleFunc("PUT /settings/ai-gen-template", requireAdmin(h.UpdateTemplate))
	api.HandleFunc("POST /settings/ai-gen-template/reset", requireAdmin(h.ResetTemplate))
	api.HandleFunc("PUT /settings/ai-gen-parent-template", requireAdmin(h.UpdateParentTemplate))
	api.HandleFunc("POST /settings/ai-gen-parent-template/reset", requireAdmin(h.ResetParentTemplate))
	api.HandleFunc("POST /requirements/{id}/generate-tests", requireAuth("write", h.GenerateTests))
	api.HandleFunc("POST /requirements/{id}/accept-generated-tests", requireAuth("write", h.AcceptGeneratedTests))
	api.HandleFunc("POST /import/parse", requireAuth("write", h.ParseImport))
	api.HandleFunc("POST /import/accept", requireAuth("write", h.AcceptImport))

	// ai-failure-analysis endpoints
	api.HandleFunc("POST /run-results/{id}/analyze", requireAuth("write", h.AnalyzeRunResult))
	api.HandleFunc("GET /run-results/{id}/analyses", requireAuth("read", h.ListRunResultAnalyses))
	api.HandleFunc("GET /runs/{id}/analyses/current", requireAuth("read", h.ListCurrentAnalysesForRun))
	api.HandleFunc("POST /runs/{id}/analyze-failures", requireAuth("write", h.EnqueueRunAnalysis))
	api.HandleFunc("GET /runs/{id}/analysis-job", requireAuth("read", h.GetRunAnalysisJob))
	api.HandleFunc("POST /runs/{id}/analysis-job/cancel", requireAuth("write", h.CancelRunAnalysisJob))
	api.HandleFunc("GET /settings/ai-failure-analysis", requireAuth("read", h.GetFailureAnalysisSettings))
	api.HandleFunc("PUT /settings/ai-failure-analysis", requireAdmin(h.UpdateFailureAnalysisSettings))
	api.HandleFunc("POST /settings/ai-failure-analysis/prompt/reset", requireAdmin(h.ResetFailureAnalysisPrompt))
}
