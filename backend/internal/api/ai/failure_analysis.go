package ai

import (
	"encoding/json"
	"fmt"
	"net/http"
	"ttgo/internal/api/authctx"
	"ttgo/internal/api/httpx"
	"ttgo/pkg/tracker/models"
)

func (h *Handler) GetFailureAnalysisSettings(w http.ResponseWriter, r *http.Request) {
	s, err := h.store.GetFailureAnalysisSettings()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, s)
}

func (h *Handler) UpdateFailureAnalysisSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EnabledOnCompletion bool   `json:"enabled_on_completion"`
		MaxAnalysesPerRun   int    `json:"max_analyses_per_run"`
		DedupEnabled        bool   `json:"dedup_enabled"`
		RedactionEnabled    bool   `json:"redaction_enabled"`
		PromptTemplate      string `json:"prompt_template"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if req.MaxAnalysesPerRun < 1 || req.MaxAnalysesPerRun > 500 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "max_analyses_per_run must be between 1 and 500"})
		return
	}
	if req.PromptTemplate == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "prompt_template is required"})
		return
	}
	updated, err := h.store.UpdateFailureAnalysisSettings(&models.AIFailureAnalysisSettings{
		EnabledOnCompletion: req.EnabledOnCompletion,
		MaxAnalysesPerRun:   req.MaxAnalysesPerRun,
		DedupEnabled:        req.DedupEnabled,
		RedactionEnabled:    req.RedactionEnabled,
		PromptTemplate:      req.PromptTemplate,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, updated)
}

func (h *Handler) ResetFailureAnalysisPrompt(w http.ResponseWriter, r *http.Request) {
	if err := h.store.ResetFailureAnalysisPrompt(); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	s, _ := h.store.GetFailureAnalysisSettings()
	httpx.JSON(w, http.StatusOK, s)
}

// AnalyzeRunResult runs synchronous analysis on a single RunResult.
func (h *Handler) AnalyzeRunResult(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	result, err := h.store.GetRunResultByID(id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if result == nil {
		httpx.Error(w, http.StatusNotFound, fmt.Errorf("run result not found"))
		return
	}
	userID := authctx.ActorID(r.Context())
	row, err := h.analyzeSync(r.Context(), result, userID)
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, row)
}

// ListRunResultAnalyses returns all versions for a single result, newest first.
func (h *Handler) ListRunResultAnalyses(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	result, err := h.store.GetRunResultByID(id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if result == nil {
		httpx.Error(w, http.StatusNotFound, fmt.Errorf("run result not found"))
		return
	}
	rows, err := h.store.ListAnalysesForResult(id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	// Omit the raw LLM output (which can reflect failure-log text) from list
	// responses; it stays available in the stored record (F-068).
	for _, a := range rows {
		a.RawResponse = ""
	}
	httpx.JSON(w, http.StatusOK, rows)
}

// ListCurrentAnalysesForRun returns a map of run_result_id → newest analysis.
func (h *Handler) ListCurrentAnalysesForRun(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")
	run, err := h.store.GetTestRun(runID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if run == nil {
		httpx.Error(w, http.StatusNotFound, fmt.Errorf("test run not found"))
		return
	}
	m, err := h.store.GetCurrentAnalysesByRun(runID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	for _, a := range m {
		a.RawResponse = "" // omit raw LLM output from list responses (F-068)
	}
	httpx.JSON(w, http.StatusOK, m)
}

// EnqueueRunAnalysis creates (or returns) a batch job for analyzing a run's failures.
func (h *Handler) EnqueueRunAnalysis(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")
	run, err := h.store.GetTestRun(runID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if run == nil {
		httpx.Error(w, http.StatusNotFound, fmt.Errorf("test run not found"))
		return
	}
	failures, err := h.store.ListLatestFailingResults(runID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if len(failures) == 0 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "no failing results in this run"})
		return
	}
	userID := authctx.ActorID(r.Context())
	job, created, err := h.store.MaybeEnqueueForRun(runID, models.RunAnalysisJobTriggerManual, userID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if !created {
		httpx.JSON(w, http.StatusConflict, map[string]interface{}{
			"error": "analysis already running",
			"job":   job,
		})
		return
	}
	httpx.JSON(w, http.StatusCreated, job)
}

// GetRunAnalysisJob returns the most recent job for a run.
func (h *Handler) GetRunAnalysisJob(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")
	job, err := h.store.GetLatestAnalysisJobForRun(runID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if job == nil {
		httpx.Error(w, http.StatusNotFound, fmt.Errorf("no analysis job for this run"))
		return
	}
	httpx.JSON(w, http.StatusOK, job)
}

// CancelRunAnalysisJob marks the most recent active job as cancelled.
func (h *Handler) CancelRunAnalysisJob(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")
	job, err := h.store.GetLatestAnalysisJobForRun(runID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if job == nil || (job.Status != models.RunAnalysisJobStatusQueued && job.Status != models.RunAnalysisJobStatusRunning) {
		httpx.Error(w, http.StatusNotFound, fmt.Errorf("no active analysis job for this run"))
		return
	}
	if err := h.store.UpdateAnalysisJobStatus(job.ID, models.RunAnalysisJobStatusCancelled, ""); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
