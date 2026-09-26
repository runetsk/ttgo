package ai

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
	"ttgo/internal/api/authctx"
	"ttgo/internal/api/httpx"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"
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
		ParallelGroups      *int   `json:"parallel_groups"` // omitted = keep the current value
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
	parallel := 0
	if req.ParallelGroups != nil {
		parallel = *req.ParallelGroups
		if parallel < 1 || parallel > models.MaxParallelGroups {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("parallel_groups must be between 1 and %d", models.MaxParallelGroups)})
			return
		}
	}
	if req.PromptTemplate == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "prompt_template is required"})
		return
	}
	updated, err := h.store.UpdateFailureAnalysisSettings(&models.AIFailureAnalysisSettings{
		EnabledOnCompletion: req.EnabledOnCompletion,
		MaxAnalysesPerRun:   req.MaxAnalysesPerRun,
		ParallelGroups:      parallel,
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
	if errors.Is(err, failureanalysis.ErrAIDisabled) {
		httpx.JSON(w, http.StatusConflict, map[string]string{"error": "AI features are switched off"})
		return
	}
	if err != nil && row != nil {
		// The attempt was recorded as failed; say why and return the stored row with it.
		httpx.JSON(w, http.StatusBadGateway, map[string]interface{}{"error": row.Summary, "analysis": row})
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, err)
		return
	}
	// RawResponse is deliberately NOT stripped here — the F-068 omission applies to list
	// responses only. SuggestedDefectType is persisted (models.RunResultAnalysis), not derived.
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
	if !h.requireAI(w) {
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

// GetRunAnalysisJob returns the most recent job for a run, or null if the run has
// never been analyzed. 404 means the run itself is missing — a run with no job yet
// is a 200, matching ListCurrentAnalysesForRun.
func (h *Handler) GetRunAnalysisJob(w http.ResponseWriter, r *http.Request) {
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
	job, err := h.store.GetLatestAnalysisJobForRun(runID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	view, err := h.jobView(job)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, view)
}

// analysisJobView is a job with what its analyses produced, so a client can tell a clean
// run from one with failed attempts or missing explanations after the job has ended.
type analysisJobView struct {
	*models.RunAnalysisJob
	Outcomes *models.RunAnalysisJobOutcomes `json:"outcomes,omitempty"`
}

func (h *Handler) jobView(job *models.RunAnalysisJob) (*analysisJobView, error) {
	if job == nil {
		return nil, nil
	}
	o, err := h.store.AnalysisJobOutcomes(job.ID)
	if err != nil {
		return nil, err
	}
	return &analysisJobView{RunAnalysisJob: job, Outcomes: &o}, nil
}

// ListRunAnalysisJobs returns every analysis job of a run, newest first, each with its
// pipeline and outcomes. Used to grade one job as a whole (`ttgo ai compare --by-job`).
//
// @Summary      List a run's analysis jobs
// @Description  Every failure-analysis job of the run, newest first, with the pipeline it ran (decider, narrator, explanations, takeover threshold, fallback, reply cap) and the outcome counts of its analyses.
// @Tags         ai-failure-analysis
// @Produce      json
// @Param        id   path      string  true  "Run ID"
// @Success      200  {array}   map[string]interface{}
// @Failure      404  {object}  map[string]interface{}
// @Router       /runs/{id}/analysis-jobs [get]
// @Security     BearerAuth
func (h *Handler) ListRunAnalysisJobs(w http.ResponseWriter, r *http.Request) {
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
	jobs, err := h.store.ListAnalysisJobsForRun(runID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]*analysisJobView, 0, len(jobs))
	for _, j := range jobs {
		v, err := h.jobView(j)
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, err)
			return
		}
		out = append(out, v)
	}
	httpx.JSON(w, http.StatusOK, out)
}

// RetryFailedRunAnalysis queues a job that re-analyzes only the groups whose current
// analysis failed.
//
// @Summary      Retry failed analyses
// @Description  Queues a failure-analysis job limited to the groups whose current analysis is a failed attempt (provider error, reply cut off or unreadable twice, TypeSafe unavailable with the fallback off). 409 when nothing failed or a job is already active.
// @Tags         ai-failure-analysis
// @Produce      json
// @Param        id   path      string  true  "Run ID"
// @Success      201  {object}  models.RunAnalysisJob
// @Failure      404  {object}  map[string]interface{}
// @Failure      409  {object}  map[string]interface{}
// @Router       /runs/{id}/analysis-job/retry-failed [post]
// @Security     BearerAuth
func (h *Handler) RetryFailedRunAnalysis(w http.ResponseWriter, r *http.Request) {
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
	if !h.requireAI(w) {
		return
	}
	failed, err := h.store.FailedResultIDsForRun(runID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if len(failed) == 0 {
		httpx.JSON(w, http.StatusConflict, map[string]string{"error": "no failed analyses to retry"})
		return
	}
	job, created, err := h.store.EnqueueRetryFailedForRun(runID, authctx.ActorID(r.Context()))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if !created {
		httpx.JSON(w, http.StatusConflict, map[string]interface{}{"error": "analysis already running", "job": job})
		return
	}
	httpx.JSON(w, http.StatusCreated, job)
}

// ExplainAnalysis writes an explanation for a stored TypeSafe decision that has none, because
// explanations were off or the explanation call failed. The decision is not re-run.
//
// @Summary      Explain a stored TypeSafe decision
// @Description  Asks the default LLM to explain a TypeSafe decision whose explanation was skipped, unavailable or unreadable, and stores the explanation on the same analysis. The verdict, confidence and suggestion do not change. 409 when the analysis is not an unexplained TypeSafe decision, AI is switched off, or no LLM provider is available.
// @Tags         ai-failure-analysis
// @Produce      json
// @Param        id          path      string  true  "Run result ID"
// @Param        analysisId  path      string  true  "Analysis ID"
// @Success      200  {object}  models.RunResultAnalysis
// @Failure      404  {object}  map[string]interface{}
// @Failure      409  {object}  map[string]interface{}
// @Router       /run-results/{id}/analyses/{analysisId}/explain [post]
// @Security     BearerAuth
func (h *Handler) ExplainAnalysis(w http.ResponseWriter, r *http.Request) {
	result, err := h.store.GetRunResultByID(r.PathValue("id"))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if result == nil {
		httpx.Error(w, http.StatusNotFound, fmt.Errorf("run result not found"))
		return
	}
	a, err := h.store.GetAnalysisByID(r.PathValue("analysisId"))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if a == nil || a.RunResultID != result.ID {
		httpx.Error(w, http.StatusNotFound, fmt.Errorf("analysis not found"))
		return
	}
	if a.Engine != models.AnalysisEngineTypeSafe || a.Failed() {
		httpx.JSON(w, http.StatusConflict, map[string]string{"error": "only a stored TypeSafe decision can be explained; re-analyze instead"})
		return
	}
	if a.NarrativeStatus == models.NarrativeStatusOK {
		httpx.JSON(w, http.StatusConflict, map[string]string{"error": "this analysis already has an explanation"})
		return
	}
	if h.resolveDeps == nil {
		httpx.JSON(w, http.StatusConflict, map[string]string{"error": "failure analysis is not configured"})
		return
	}
	deps, err := h.resolveDeps(failureanalysis.TriggerExplain)
	if errors.Is(err, failureanalysis.ErrAIDisabled) {
		httpx.JSON(w, http.StatusConflict, map[string]string{"error": "AI features are switched off"})
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, err)
		return
	}
	if deps.Narrative == nil {
		reason := deps.LLMUnavailableReason
		if reason == "" {
			reason = "no default LLM provider is configured"
		}
		httpx.JSON(w, http.StatusConflict, map[string]string{"error": "cannot explain: " + reason})
		return
	}
	settings, err := h.store.GetFailureAnalysisSettings()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	actx := failureanalysis.BuildContext(h.store, result, time.Now())
	actx.RedactionEnabled = settings.RedactionEnabled
	actx.PromptTemplate = settings.PromptTemplate
	actx.ProviderModel = deps.NarrativeModel
	res, err := failureanalysis.Explain(r.Context(), deps.Analyze(), actx, a)
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, err)
		return
	}
	updated, err := h.store.UpdateAnalysisNarrative(a.ID, store.NarrativeUpdate{
		Summary: res.Summary, NextAction: res.NextAction, Rationale: res.Rationale, NarrativeStatus: res.NarrativeStatus,
		AddPrompt: res.TokenUsagePrompt, AddCompletion: res.TokenUsageCompletion,
		AddLLMMs: res.LLMMs, AddLLMCalls: res.LLMCalls, FinishReason: res.FinishReason,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if h.broadcaster != nil {
		h.broadcaster.BroadcastRunResultAnalysisCreated(updated, result.TestRunID)
	}
	httpx.JSON(w, http.StatusOK, updated)
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
	changed, err := h.store.UpdateAnalysisJobStatus(job.ID, models.RunAnalysisJobStatusCancelled, "")
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if !changed {
		// The job reached a terminal state (completed/failed) between our read above and the
		// conditional update — UpdateAnalysisJobStatus only ever writes "cancelled" over a
		// still-active job, so a false here means there is nothing left to cancel.
		httpx.Error(w, http.StatusConflict, fmt.Errorf("analysis job is no longer active"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Accuracy window bounds for GET /ai/failure-analysis/accuracy. The default matches the other
// reporting endpoints; the cap keeps a hand-typed ?days from widening the query to all history.
const (
	defaultAccuracyWindowDays = 30
	maxAccuracyWindowDays     = 365
)

// GetFailureAnalysisAccuracy reports how often the AI's suggested defect_type matched the
// human's triage decision.
//
// @Summary      AI failure-analysis accuracy
// @Description  Agreement between the AI's suggested defect_type and the human triage decision over a rolling window, overall and broken down by the snapshotted verdict and confidence. The window is measured on the moment the decision was recorded, not on when the row was last touched. Only explicitly triaged failing results (FAIL or ERROR) that carried a suggestion at the decision moment are counted — results still sitting at the "to_investigate" auto-default (i.e. untriaged) and results with no suggestion are excluded rather than counted as disagreements.
// @Tags         ai-failure-analysis
// @Produce      json
// @Param        days  query     int  false  "Rolling window in days, counted from the triage decision (1-365)"  default(30)
// @Success      200   {object}  map[string]interface{}  "{total, agreed, agreement_rate, by_verdict:[{verdict,total,agreed,rate}], by_confidence:[{confidence,total,agreed,rate}]}"
// @Failure      500   {object}  map[string]interface{}
// @Router       /ai/failure-analysis/accuracy [get]
// @Security     BearerAuth
func (h *Handler) GetFailureAnalysisAccuracy(w http.ResponseWriter, r *http.Request) {
	days := defaultAccuracyWindowDays
	if d := r.URL.Query().Get("days"); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v > 0 {
			days = v
		}
	}
	if days > maxAccuracyWindowDays {
		days = maxAccuracyWindowDays
	}
	rep, err := h.store.GetFailureAnalysisAccuracy(time.Now().UTC().AddDate(0, 0, -days))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, rep)
}
