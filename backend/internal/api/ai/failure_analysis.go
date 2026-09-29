package ai

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"ttgo/internal/api/authctx"
	"ttgo/internal/api/httpx"
	"ttgo/pkg/tracker/callstats"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"
)

// GetFailureAnalysisSettings returns the failure-analysis settings.
//
// @Summary      Get failure-analysis settings
// @Description  The singleton settings: automatic analysis, caps, parallel groups, dedup, redaction, prompt template, and the LLM latency settings llm_call_timeout_seconds (10–120, default 45) and hedge_after_seconds (0 = off).
// @Tags         ai-failure-analysis
// @Produce      json
// @Success      200  {object}  models.AIFailureAnalysisSettings
// @Router       /settings/ai-failure-analysis [get]
// @Security     BearerAuth
func (h *Handler) GetFailureAnalysisSettings(w http.ResponseWriter, r *http.Request) {
	s, err := h.store.GetFailureAnalysisSettings()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, s)
}

// UpdateFailureAnalysisSettings replaces the failure-analysis settings.
//
// @Summary      Update failure-analysis settings
// @Description  parallel_groups, llm_call_timeout_seconds and hedge_after_seconds may be omitted to keep their stored values. llm_call_timeout_seconds bounds each LLM request (10–120 s; a call cut by it is retried once); hedge_after_seconds sends an identical second request after that many seconds without an answer (0 = off, else at least 3 and below the call timeout). few_shot_examples (0..8, 0 = off: how many past human triage decisions accompany each analyzed failure) also keeps its stored value when omitted. 400 with the reason when a value is out of range.
// @Tags         ai-failure-analysis
// @Accept       json
// @Produce      json
// @Param        body  body  object  true  "enabled_on_completion, max_analyses_per_run (1–500), parallel_groups, dedup_enabled, redaction_enabled, prompt_template, llm_call_timeout_seconds, hedge_after_seconds, few_shot_examples"
// @Success      200  {object}  models.AIFailureAnalysisSettings
// @Failure      400  {object}  map[string]interface{}
// @Router       /settings/ai-failure-analysis [put]
// @Security     BearerAuth
func (h *Handler) UpdateFailureAnalysisSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EnabledOnCompletion   bool   `json:"enabled_on_completion"`
		MaxAnalysesPerRun     int    `json:"max_analyses_per_run"`
		ParallelGroups        *int   `json:"parallel_groups"` // omitted = keep the current value
		DedupEnabled          bool   `json:"dedup_enabled"`
		RedactionEnabled      bool   `json:"redaction_enabled"`
		PromptTemplate        string `json:"prompt_template"`
		LLMCallTimeoutSeconds *int   `json:"llm_call_timeout_seconds"` // omitted = keep
		HedgeAfterSeconds     *int   `json:"hedge_after_seconds"`      // omitted = keep; 0 = off
		FewShotExamples       *int   `json:"few_shot_examples"`        // omitted = keep; 0 = off
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
	current, err := h.store.GetFailureAnalysisSettings()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	callTimeout, hedgeAfter := current.LLMCallTimeoutSeconds, current.HedgeAfterSeconds
	if callTimeout == 0 {
		callTimeout = models.DefaultLLMCallTimeoutSeconds
	}
	if req.LLMCallTimeoutSeconds != nil {
		callTimeout = *req.LLMCallTimeoutSeconds
	}
	if req.HedgeAfterSeconds != nil {
		hedgeAfter = *req.HedgeAfterSeconds
	}
	if err := models.ValidateLLMLatency(callTimeout, hedgeAfter); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	// 0 is a value (examples off), so an omitted field carries the stored one through.
	fewShot := current.FewShotExamples
	if req.FewShotExamples != nil {
		fewShot = *req.FewShotExamples
		if fewShot < 0 || fewShot > models.MaxFewShotExamples {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("few_shot_examples must be between 0 and %d", models.MaxFewShotExamples)})
			return
		}
	}
	updated, err := h.store.UpdateFailureAnalysisSettings(&models.AIFailureAnalysisSettings{
		EnabledOnCompletion:   req.EnabledOnCompletion,
		MaxAnalysesPerRun:     req.MaxAnalysesPerRun,
		ParallelGroups:        parallel,
		DedupEnabled:          req.DedupEnabled,
		RedactionEnabled:      req.RedactionEnabled,
		PromptTemplate:        req.PromptTemplate,
		LLMCallTimeoutSeconds: callTimeout,
		HedgeAfterSeconds:     hedgeAfter,
		FewShotExamples:       fewShot,
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
//
// @Summary      Analyze one failing result
// @Description  Runs failure analysis on one result now and stores it as a new version. A failed attempt is stored too and returned with 502. 409 when AI is switched off, or when the estimated cost exceeds a soft AI budget and acknowledge_budget is not true (payload: category "budget", scope, estimated_cost_usd, budget_usd, month_spent_usd).
// @Tags         ai-failure-analysis
// @Produce      json
// @Param        id                  path   string  true   "Run result ID"
// @Param        acknowledge_budget  query  bool    false  "Proceed although a soft AI budget would be exceeded"
// @Success      201  {object}  models.RunResultAnalysis
// @Failure      404  {object}  map[string]interface{}
// @Failure      409  {object}  map[string]interface{}
// @Failure      502  {object}  map[string]interface{}
// @Router       /run-results/{id}/analyze [post]
// @Security     BearerAuth
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
	row, err := h.analyzeSync(r.Context(), result, userID, acknowledgedBudget(r))
	if errors.Is(err, failureanalysis.ErrAIDisabled) {
		httpx.JSON(w, http.StatusConflict, map[string]string{"error": "AI features are switched off"})
		return
	}
	var over *budgetExceededError
	if errors.As(err, &over) {
		httpx.JSON(w, http.StatusConflict, over.payload)
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
//
// @Summary      Analyze a run's failures
// @Description  Queues a failure-analysis job for the run's failing results. 409 when a job is already active, AI is switched off, or the job's estimated cost (failing groups × the per-call estimate) exceeds a soft AI budget and acknowledge_budget is not true.
// @Tags         ai-failure-analysis
// @Produce      json
// @Param        id                  path   string  true   "Run ID"
// @Param        acknowledge_budget  query  bool    false  "Proceed although a soft AI budget would be exceeded"
// @Success      201  {object}  models.RunAnalysisJob
// @Failure      400  {object}  map[string]interface{}
// @Failure      404  {object}  map[string]interface{}
// @Failure      409  {object}  map[string]interface{}
// @Router       /runs/{id}/analyze-failures [post]
// @Security     BearerAuth
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
	settings, err := h.store.GetFailureAnalysisSettings()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	groups := failureanalysis.PlannedGroups(failures, settings.DedupEnabled, settings.MaxAnalysesPerRun)
	if warn := h.jobBudgetWarning(r, groups); warn != nil {
		httpx.JSON(w, http.StatusConflict, warn)
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
// @Description  Every failure-analysis job of the run, newest first, with the pipeline it ran (decider, narrator, explanations, takeover threshold, fallback, reply cap) and the outcome counts of its analyses. Outcomes include stage timing (decision_ms_avg/p50/max, llm_ms_avg/p50/max over representatives), rate_limit_hits, and the LLM latency counts call_timeouts, hedges_fired and hedges_won.
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
// @Description  Queues a failure-analysis job limited to the groups whose current analysis is a failed attempt (provider error, reply cut off or unreadable twice, TypeSafe unavailable with the fallback off). 409 when nothing failed or a job is already active. 409 also when the estimated cost exceeds a soft AI budget and acknowledge_budget is not true.
// @Tags         ai-failure-analysis
// @Produce      json
// @Param        id   path      string  true  "Run ID"
// @Param        acknowledge_budget  query  bool  false  "Proceed although a soft AI budget would be exceeded"
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
	settings, err := h.store.GetFailureAnalysisSettings()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	groups := len(failed) // an upper bound: failed results, before regrouping
	if settings.MaxAnalysesPerRun > 0 && groups > settings.MaxAnalysesPerRun {
		groups = settings.MaxAnalysesPerRun
	}
	if warn := h.jobBudgetWarning(r, groups); warn != nil {
		httpx.JSON(w, http.StatusConflict, warn)
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
// explanations were off or the explanation call failed. The decision is not re-run. A group is
// explained once, through its representative, and every clone receives the same explanation.
//
// @Summary      Explain a stored TypeSafe decision
// @Description  Asks the default LLM to explain a TypeSafe decision whose explanation was skipped, unavailable or unreadable, and stores the explanation on the same analysis and on every analysis grouped with it. On a grouped (clone) analysis the group's representative is explained, from the representative's evidence, and the clicked analysis is returned refreshed. The verdict, confidence and suggestion do not change. 409 when the analysis is not an unexplained TypeSafe decision, when its group is already being explained or explained, when AI is switched off, or no LLM provider is available. 409 also when the explanation's estimated cost exceeds a soft AI budget and acknowledge_budget is not true, and when TypeSafe flagged the failure as a possible prompt injection (signals.injection at or above 0.80) and override_injection is not true. A failed LLM call leaves the explanation unavailable with its reason (200). With scope=result on a semantically grouped result whose narrative_fit is below 0.5 (the group's explanation may not apply to it), or whose own explanation failed, only that result is explained, from its own evidence and without the group's members: TypeSafe.ai first checks that evidence for prompt injection (409 on a hit, or when TypeSafe.ai cannot check it, unless override_injection=true), and only the checked evidence is sent to the LLM. The row gets narrative_split=true, keeps its narrative_fit and its decision, and later group explanations leave it alone. 409 when the row is not such a result, is already being explained or already has its own explanation. After a group is explained, TypeSafe.ai checks the explanation against each semantic clone and stores narrative_fit on it.
// @Tags         ai-failure-analysis
// @Produce      json
// @Param        id          path      string  true  "Run result ID"
// @Param        analysisId  path      string  true  "Analysis ID"
// @Param        acknowledge_budget  query  bool  false  "Proceed although a soft AI budget would be exceeded"
// @Param        override_injection  query  bool  false  "Send the failure to the LLM although prompt injection is suspected or could not be checked"
// @Param        scope  query  string  false  "result: explain only this grouped result, from its own evidence"
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
	// Before the group's injection guard: the clone's own evidence is checked on that path.
	if r.URL.Query().Get("scope") == "result" {
		h.explainOwnResult(w, r, result, a)
		return
	}
	// A clone carries its group's decision: the group is explained through its representative —
	// its evidence, its members, its row — never from the clicked sibling.
	rep, repResult := a, result
	if a.SourceAnalysisID != nil {
		src, err := h.store.GetAnalysisByID(*a.SourceAnalysisID)
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, err)
			return
		}
		var srcResult *models.RunResult
		if src != nil {
			if srcResult, err = h.store.GetRunResultByID(src.RunResultID); err != nil {
				httpx.Error(w, http.StatusInternalServerError, err)
				return
			}
		}
		if src == nil || srcResult == nil {
			httpx.JSON(w, http.StatusConflict, map[string]string{"error": "this analysis's group representative no longer exists; re-analyze instead"})
			return
		}
		rep, repResult = src, srcResult
	}
	if rep.NarrativeStatus == models.NarrativeStatusOK {
		httpx.JSON(w, http.StatusConflict, map[string]string{"error": "this analysis already has an explanation"})
		return
	}
	// Injection guard (spec Wave 3 §1.4): a flagged failure goes to the LLM only on an explicit
	// confirmation. Checked before anything is claimed or spent; clones carry the group's flag.
	if override, _ := strconv.ParseBool(r.URL.Query().Get("override_injection")); !override &&
		failureanalysis.ParseSignals(rep.Signals).InjectionFlagged() {
		httpx.JSON(w, http.StatusConflict, map[string]string{"error": failureanalysis.ErrInjectionOverrideRequired.Error()})
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
	if warn := h.checkAnalysisBudget(failureanalysis.EstimateExplainUSD(deps), 1, acknowledgedBudget(r)); warn != nil {
		httpx.JSON(w, http.StatusConflict, warn)
		return
	}
	settings, err := h.store.GetFailureAnalysisSettings()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	members, err := h.store.ListGroupMemberResults(rep.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	actx := failureanalysis.BuildContext(h.store, repResult, time.Now(), deps.FewShotExamples)
	actx.RedactionEnabled = settings.RedactionEnabled
	actx.PromptTemplate = settings.PromptTemplate
	actx.ProviderModel = deps.NarrativeModel
	actx.GroupMembers = failureanalysis.GroupMemberErrors(repResult, members)

	// Claim before calling the LLM: at most one explanation is in flight per group, and a
	// request that loses the claim never spends anything.
	claimed, err := h.store.ClaimExplanation(rep.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if !claimed {
		httpx.JSON(w, http.StatusConflict, map[string]string{"error": "this analysis is already being explained or has been explained"})
		return
	}
	if pending, _ := h.store.GetAnalysisByID(rep.ID); pending != nil {
		h.broadcastAnalysisUpdated([]*models.RunResultAnalysis{pending}, repResult.TestRunID)
	}

	// P1: the call runs under its own call-stats counter so fired hedges are billed below.
	callCtx, calls := callstats.WithCounter(r.Context())
	// R8: only evidence the decision's injection question covered, and that is unchanged since,
	// reaches the LLM.
	delta, ok := failureanalysis.Narrate(callCtx, deps.Analyze(), actx, failureanalysis.DecidedForExplain(rep, actx))
	if !ok { // cannot happen for a claimed decision; never leave the claim pending
		delta = failureanalysis.NarrationDelta{NarrativeStatus: models.NarrativeStatusUnavailable, Reason: "nothing to explain",
			Summary: "AI narrative unavailable: nothing to explain"}
	}
	if delta.Reason == "cancelled" { // P2's internal marker: the request was dropped mid-call
		const why = "the request was cancelled before the explanation was written"
		delta.NarrativeStatus, delta.Reason, delta.Summary, delta.NextAction = models.NarrativeStatusUnavailable, why, "AI narrative unavailable: "+why, ""
	}
	// The narrative transfer check (spec §2): does this explanation also describe each semantic
	// clone's failure? On the request's context, so a dropped request leaves the fits unset.
	if delta.NarrativeStatus == models.NarrativeStatusOK && deps.Transfer != nil {
		clones, lerr := h.store.ListSemanticCloneResults(rep.ID)
		if lerr != nil {
			slog.Warn("failure-analysis: semantic clones not loaded; no transfer check", "analysis_id", rep.ID, "err", lerr)
		} else if len(clones) > 0 {
			in := failureanalysis.TransferInputFor(delta, repResult, clones, settings.RedactionEnabled)
			delta = failureanalysis.RunTransferCheck(callCtx, *deps.Transfer, in, delta)
		}
	}
	// Dated now, not on the analysis: explaining an old decision is this month's spend. Recorded
	// whether or not the apply below wins: the call was billed.
	refs := failureanalysis.RefsFor(repResult.TestRunID, rep.JobID, rep)
	h.recordCosts(failureanalysis.NarrationCostEvents(delta, deps, refs, models.AnalysisCostKindExplain))
	h.recordCosts(failureanalysis.HedgeCostEvents(calls.HedgePromptTokens(), deps, refs))
	h.recordCosts(failureanalysis.TransferCostEvents(delta.TransferTokens, delta.TransferModel, deps, refs))
	changed, err := h.store.ApplyNarration(rep.ID, delta)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if len(changed) == 0 {
		slog.Info("failure-analysis: explanation not applied; its group was settled while it was written", "analysis_id", rep.ID)
	}
	h.broadcastAnalysisUpdated(changed, repResult.TestRunID)

	clicked, err := h.store.GetAnalysisByID(a.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if clicked == nil {
		httpx.Error(w, http.StatusNotFound, fmt.Errorf("analysis not found"))
		return
	}
	httpx.JSON(w, http.StatusOK, clicked)
}

// ownConflict is why a row cannot be explained on its own now, or "".
func ownConflict(a *models.RunResultAnalysis) string {
	mismatched := a.SourceAnalysisID != nil && a.NarrativeFit != nil && *a.NarrativeFit < failureanalysis.TransferFitMin
	switch {
	case !mismatched:
		return "only a grouped result whose group explanation may not apply can be explained on its own"
	case a.NarrativeStatus == models.NarrativeStatusPending:
		return "this result is already being explained"
	case a.NarrativeSplit && a.NarrativeStatus == models.NarrativeStatusOK:
		return "this result already has its own explanation"
	}
	return ""
}

// explainOwnResult writes an explanation for one semantic clone from its own evidence, when the
// group's explanation may not apply to it (narrative_fit below failureanalysis.TransferFitMin), or
// retries a failed own explanation (spec §2.1, R9). TypeSafe first checks the clone's own evidence
// for prompt injection; a hit, a failed check or no TypeSafe needs override_injection=true. Then the
// row alone is claimed, narrated without group members from the checked blocks, and applied.
func (h *Handler) explainOwnResult(w http.ResponseWriter, r *http.Request, result *models.RunResult, a *models.RunResultAnalysis) {
	override := false
	if v := r.URL.Query().Get("override_injection"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, fmt.Errorf("override_injection must be true or false"))
			return
		}
		override = b
	}
	if msg := ownConflict(a); msg != "" {
		httpx.JSON(w, http.StatusConflict, map[string]string{"error": msg})
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
	if warn := h.checkAnalysisBudget(failureanalysis.EstimateOwnExplainUSD(deps), 1, acknowledgedBudget(r)); warn != nil {
		httpx.JSON(w, http.StatusConflict, warn)
		return
	}
	settings, err := h.store.GetFailureAnalysisSettings()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	actx := failureanalysis.BuildContext(h.store, result, time.Now(), deps.FewShotExamples)
	actx.RedactionEnabled = settings.RedactionEnabled
	actx.PromptTemplate = settings.PromptTemplate
	actx.ProviderModel = deps.NarrativeModel

	// R9: the clone's own evidence was never checked (the group's decision checked the
	// representative's). Ask TypeSafe first; it is billed to this row whatever happens next.
	refs := failureanalysis.RefsFor(result.TestRunID, a.JobID, a)
	check, cerr := failureanalysis.OwnEvidenceCheck{}, failureanalysis.ErrOwnCheckUnavailable
	if deps.Transfer != nil {
		check, cerr = failureanalysis.CheckOwnEvidence(r.Context(), *deps.Transfer, actx)
	}
	h.recordCosts(failureanalysis.OwnCheckCostEvents(check, deps, refs))
	switch {
	case cerr != nil && !override:
		slog.Info("failure-analysis: own evidence not checked for prompt injection", "analysis_id", a.ID, "err", cerr)
		httpx.JSON(w, http.StatusConflict, map[string]string{"error": "TypeSafe.ai could not check this result's evidence for prompt injection: confirm to send it to the LLM unchecked"})
		return
	case cerr == nil && check.Signals.InjectionFlagged() && !override:
		httpx.JSON(w, http.StatusConflict, map[string]string{"error": "possible prompt injection: confirm to send this failure to the LLM"})
		return
	}

	claimed, err := h.store.ClaimOwnExplanation(a.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if !claimed {
		httpx.JSON(w, http.StatusConflict, map[string]string{"error": "this result is already being explained"})
		return
	}
	if pending, _ := h.store.GetAnalysisByID(a.ID); pending != nil {
		h.broadcastAnalysisUpdated([]*models.RunResultAnalysis{pending}, result.TestRunID)
	}

	decided := failureanalysis.DecidedFromRow(a)
	// Narrate sends only the blocks the check covered (R8 filter on decided.Signals). With an
	// override and no check there are no signals: the clone's own evidence goes unfiltered, and
	// this path never sends group members.
	decided.Signals = ""
	if cerr == nil {
		decided.Signals = failureanalysis.SignalsJSON(check.Signals)
	}
	callCtx, calls := callstats.WithCounter(r.Context())
	delta, ok := failureanalysis.Narrate(callCtx, deps.Analyze(), actx, decided)
	if !ok { // cannot happen for a claimed decision; never leave the claim pending
		delta = failureanalysis.NarrationDelta{NarrativeStatus: models.NarrativeStatusUnavailable, Reason: "nothing to explain",
			Summary: "AI narrative unavailable: nothing to explain"}
	}
	if delta.Reason == "cancelled" {
		const why = "the request was cancelled before the explanation was written"
		delta.NarrativeStatus, delta.Reason, delta.Summary, delta.NextAction = models.NarrativeStatusUnavailable, why, "AI narrative unavailable: "+why, ""
	}
	h.recordCosts(failureanalysis.NarrationCostEvents(delta, deps, refs, models.AnalysisCostKindExplain))
	h.recordCosts(failureanalysis.HedgeCostEvents(calls.HedgePromptTokens(), deps, refs))
	row, err := h.store.ApplyOwnNarration(a.ID, delta)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if row == nil {
		slog.Info("failure-analysis: own explanation not applied; the row was settled while it was written", "analysis_id", a.ID)
		if row, err = h.store.GetAnalysisByID(a.ID); err != nil {
			httpx.Error(w, http.StatusInternalServerError, err)
			return
		}
		if row == nil {
			httpx.Error(w, http.StatusNotFound, fmt.Errorf("analysis not found"))
			return
		}
	} else {
		h.broadcastAnalysisUpdated([]*models.RunResultAnalysis{row}, result.TestRunID)
	}
	httpx.JSON(w, http.StatusOK, row)
}

// broadcastAnalysisUpdated republishes analyses whose explanation changed.
func (h *Handler) broadcastAnalysisUpdated(rows []*models.RunResultAnalysis, runID string) {
	if h.broadcaster == nil {
		return
	}
	for _, a := range rows {
		h.broadcaster.BroadcastRunResultAnalysisUpdated(a, runID)
	}
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
	maxPolicyVersionLen       = 64
)

// GetFailureAnalysisAccuracy reports how often the AI's suggested defect_type matched the
// human's triage decision.
//
// @Summary      AI failure-analysis accuracy
// @Description  Agreement between the AI's suggested defect_type and the human triage decision over a rolling window, overall and broken down by the snapshotted verdict, confidence and engine. Each engine and the totals are split into direct predictions and dedup clones (a group representative's answer copied onto a sibling); decisions recorded before that split existed count in the totals only (unknown_provenance). coverage reports, per engine, the representative analyses created in the window and how many decided, abstained, failed or lack an explanation. policy_version narrows everything to suggestions made under one TypeSafe policy version; policy_versions lists every version in the window. The window is measured on the moment the decision was recorded. Only explicitly triaged failing results (FAIL or ERROR) that carried a suggestion at the decision moment are counted; results still at the "to_investigate" auto-default and results with no suggestion are excluded rather than counted as disagreements.
// @Tags         ai-failure-analysis
// @Produce      json
// @Param        days            query     int     false  "Rolling window in days, counted from the triage decision (1-365)"  default(30)
// @Param        policy_version  query     string  false  "Only suggestions made under this TypeSafe policy version (empty = all versions)"
// @Success      200   {object}  store.AIFailureAnalysisAccuracy
// @Failure      400   {object}  map[string]interface{}
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
	policy := strings.TrimSpace(r.URL.Query().Get("policy_version"))
	if len(policy) > maxPolicyVersionLen {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "policy_version is too long"})
		return
	}
	rep, err := h.store.GetFailureAnalysisAccuracyFor(store.AccuracyFilter{
		Since:         time.Now().UTC().AddDate(0, 0, -days),
		PolicyVersion: policy,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, rep)
}
