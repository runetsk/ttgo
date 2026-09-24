package ai_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"ttgo/internal/api/ai"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"

	"github.com/microcosm-cc/bluemonday"
	"github.com/stretchr/testify/require"
)

// fixedReplyProvider answers every call with the same content, or fails with err.
type fixedReplyProvider struct {
	reply string
	err   error
	calls int
}

func (p *fixedReplyProvider) Chat(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	p.calls++
	if p.err != nil {
		return nil, p.err
	}
	return &llm.ChatResponse{Content: p.reply, Model: "mock", FinishReason: "stop",
		Usage: &llm.ChatUsage{PromptTokens: 200, CompletionTokens: 30, TotalTokens: 230}}, nil
}

type quickEnv struct {
	s      *store.Store
	h      *ai.Handler
	runID  string
	result *models.RunResult
}

func newQuickEnv(t *testing.T, resolve func(string) (failureanalysis.JobDeps, error)) *quickEnv {
	t.Helper()
	t.Chdir(t.TempDir())
	s, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	run := &models.TestRun{Name: "quick"}
	require.NoError(t, s.CreateTestRun(run))
	rr := &models.RunResult{TestRunID: run.ID, TestNameSnapshot: "t", AttemptNumber: 1,
		Status: models.StatusFail, FailureType: "assertion", ErrorMessage: "boom"}
	require.NoError(t, s.AddRunResult(rr))
	h := ai.NewHandler(s, bluemonday.UGCPolicy())
	h.SetFailureAnalysisDeps(resolve, nil)
	return &quickEnv{s: s, h: h, runID: run.ID, result: rr}
}

func serve(handler http.HandlerFunc, method string, params map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/api/test", nil)
	for k, v := range params {
		req.SetPathValue(k, v)
	}
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func (e *quickEnv) typeSafeRow(t *testing.T, narrative, status string) *models.RunResultAnalysis {
	t.Helper()
	score := 0.95
	a, err := e.s.CreateAnalysis(&models.RunResultAnalysis{RunResultID: e.result.ID, Engine: models.AnalysisEngineTypeSafe,
		ModelName: "jev", Verdict: models.VerdictFlakyTest, Confidence: models.ConfidenceHigh, ConfidenceScore: &score,
		SuggestedDefectType: "automation_bug", NarrativeStatus: narrative, DecisionStatus: status})
	require.NoError(t, err)
	return a
}

func (e *quickEnv) explain(a *models.RunResultAnalysis) *httptest.ResponseRecorder {
	return serve(e.h.ExplainAnalysis, "POST", map[string]string{"id": e.result.ID, "analysisId": a.ID})
}

func TestExplainAnalysis_FillsTheExplanationOfAStoredDecision(t *testing.T) {
	prov := &fixedReplyProvider{reply: `{"summary":"Timing race","next_action":"Wait for the button","rationale":"R"}`}
	var trigger string
	e := newQuickEnv(t, func(tr string) (failureanalysis.JobDeps, error) {
		trigger = tr
		return failureanalysis.JobDeps{Narrative: prov, NarrativeModel: "mock"}, nil
	})
	a := e.typeSafeRow(t, models.NarrativeStatusSkipped, models.DecisionStatusOK)

	rec := e.explain(a)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, failureanalysis.TriggerExplain, trigger)
	var got models.RunResultAnalysis
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, a.ID, got.ID, "the same analysis, not a new version")
	require.Equal(t, "Timing race", got.Summary)
	require.Equal(t, models.NarrativeStatusOK, got.NarrativeStatus)
	require.Equal(t, models.VerdictFlakyTest, got.Verdict)
	require.Equal(t, 200, got.TokenUsagePrompt)
	require.Equal(t, 1, got.LLMCalls)

	require.Equal(t, http.StatusConflict, e.explain(a).Code, "already explained")
}

func TestExplainAnalysis_Refusals(t *testing.T) {
	e := newQuickEnv(t, func(string) (failureanalysis.JobDeps, error) {
		return failureanalysis.JobDeps{LLMUnavailableReason: "the default LLM provider is misconfigured"}, nil
	})
	failed := e.typeSafeRow(t, models.NarrativeStatusUnavailable, models.DecisionStatusFailed)
	require.Equal(t, http.StatusConflict, e.explain(failed).Code, "a failed attempt is re-analyzed, not explained")

	gen, err := e.s.CreateAnalysis(&models.RunResultAnalysis{RunResultID: e.result.ID, Engine: models.AnalysisEngineGenerative,
		Verdict: models.VerdictProductBug, Confidence: models.ConfidenceHigh, NarrativeStatus: models.NarrativeStatusOK})
	require.NoError(t, err)
	require.Equal(t, http.StatusConflict, e.explain(gen).Code)

	skipped := e.typeSafeRow(t, models.NarrativeStatusSkipped, models.DecisionStatusOK)
	rec := e.explain(skipped)
	require.Equal(t, http.StatusConflict, rec.Code)
	require.Contains(t, rec.Body.String(), "misconfigured", "says why no LLM is available")

	missing := serve(e.h.ExplainAnalysis, "POST", map[string]string{"id": e.result.ID, "analysisId": "nope"})
	require.Equal(t, http.StatusNotFound, missing.Code)
}

func TestAnalyzeRunResult_FailedAttemptIsStoredAnd502(t *testing.T) {
	prov := &fixedReplyProvider{err: &llm.ProviderError{Category: llm.ErrCatProvider, StatusCode: 502, Message: "upstream"}}
	e := newQuickEnv(t, func(string) (failureanalysis.JobDeps, error) {
		return failureanalysis.JobDeps{Narrative: prov, NarrativeModel: "mock"}, nil
	})
	rec := serve(e.h.AnalyzeRunResult, "POST", map[string]string{"id": e.result.ID})
	require.Equal(t, http.StatusBadGateway, rec.Code, rec.Body.String())
	var body struct {
		Error    string                   `json:"error"`
		Analysis models.RunResultAnalysis `json:"analysis"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Contains(t, body.Error, "analysis failed")
	require.Equal(t, models.DecisionStatusFailed, body.Analysis.DecisionStatus)
	require.Equal(t, string(llm.ErrCatProvider), body.Analysis.ErrorCategory)
	require.Equal(t, "mock", body.Analysis.ModelName)

	rows, err := e.s.ListAnalysesForResult(e.result.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1, "the attempt is in the version history")
}

func TestFailureAnalysis_AIMasterSwitchIs409(t *testing.T) {
	e := newQuickEnv(t, func(string) (failureanalysis.JobDeps, error) {
		return failureanalysis.JobDeps{}, failureanalysis.ErrAIDisabled
	})
	_, err := e.s.UpdateAIFeatureSettings(false)
	require.NoError(t, err)

	params := map[string]string{"id": e.runID}
	require.Equal(t, http.StatusConflict, serve(e.h.EnqueueRunAnalysis, "POST", params).Code)
	require.Equal(t, http.StatusConflict, serve(e.h.RetryFailedRunAnalysis, "POST", params).Code)
	require.Equal(t, http.StatusConflict, serve(e.h.AnalyzeRunResult, "POST", map[string]string{"id": e.result.ID}).Code)
	a := e.typeSafeRow(t, models.NarrativeStatusSkipped, models.DecisionStatusOK)
	require.Equal(t, http.StatusConflict, e.explain(a).Code)

	jobs, err := e.s.ListAnalysisJobsForRun(e.runID)
	require.NoError(t, err)
	require.Empty(t, jobs, "nothing was queued")
}

func TestRetryFailedRunAnalysis(t *testing.T) {
	e := newQuickEnv(t, func(string) (failureanalysis.JobDeps, error) { return failureanalysis.JobDeps{}, errors.New("unused") })
	params := map[string]string{"id": e.runID}
	rec := serve(e.h.RetryFailedRunAnalysis, "POST", params)
	require.Equal(t, http.StatusConflict, rec.Code)
	require.Contains(t, rec.Body.String(), "no failed analyses")

	e.typeSafeRow(t, models.NarrativeStatusUnavailable, models.DecisionStatusFailed)
	rec = serve(e.h.RetryFailedRunAnalysis, "POST", params)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var job models.RunAnalysisJob
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &job))
	require.True(t, job.RetryFailedOnly)

	require.Equal(t, http.StatusConflict, serve(e.h.RetryFailedRunAnalysis, "POST", params).Code, "a job is already active")
	require.Equal(t, http.StatusNotFound, serve(e.h.RetryFailedRunAnalysis, "POST", map[string]string{"id": "nope"}).Code)
}

func TestRunAnalysisJobs_CarryOutcomesAndPipeline(t *testing.T) {
	e := newQuickEnv(t, nil)
	job, _, err := e.s.MaybeEnqueueForRun(e.runID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	require.NoError(t, e.s.SetAnalysisJobPipeline(job.ID, `{"decider":"jev"}`, "TypeSafe jev, no LLM"))
	_, err = e.s.CreateAnalysis(&models.RunResultAnalysis{RunResultID: e.result.ID, JobID: &job.ID, Verdict: models.VerdictUnknown,
		Confidence: models.ConfidenceLow, DecisionStatus: models.DecisionStatusFailed, ErrorCategory: "timeout"})
	require.NoError(t, err)

	params := map[string]string{"id": e.runID}
	rec := serve(e.h.GetRunAnalysisJob, "GET", params)
	require.Equal(t, http.StatusOK, rec.Code)
	var latest struct {
		ID            string                         `json:"id"`
		PipelineLabel string                         `json:"pipeline_label"`
		Outcomes      *models.RunAnalysisJobOutcomes `json:"outcomes"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &latest))
	require.Equal(t, job.ID, latest.ID)
	require.Equal(t, "TypeSafe jev, no LLM", latest.PipelineLabel)
	require.NotNil(t, latest.Outcomes)
	require.Equal(t, 1, latest.Outcomes.Failed)

	rec = serve(e.h.ListRunAnalysisJobs, "GET", params)
	require.Equal(t, http.StatusOK, rec.Code)
	var list []json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	require.Len(t, list, 1)
	require.Contains(t, string(list[0]), `"failed_rows":1`)
	require.Equal(t, http.StatusNotFound, serve(e.h.ListRunAnalysisJobs, "GET", map[string]string{"id": "nope"}).Code)
}
