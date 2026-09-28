package ai_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"

	"github.com/stretchr/testify/require"
)

// serveQuery drives a handler with a query string (the four starts read acknowledge_budget).
func serveQuery(handler http.HandlerFunc, query string, params map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/api/test"+query, nil)
	for k, v := range params {
		req.SetPathValue(k, v)
	}
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func setBudgets(t *testing.T, s *store.Store, perRequest, monthly float64) {
	t.Helper()
	_, err := s.UpdateAIBudgetSettings(map[string]interface{}{"per_request_usd": perRequest, "monthly_usd": monthly})
	require.NoError(t, err)
}

func requireBudget409(t *testing.T, rec *httptest.ResponseRecorder, scope string) map[string]any {
	t.Helper()
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "budget", body["category"])
	require.Equal(t, scope, body["scope"])
	require.Greater(t, body["estimated_cost_usd"].(float64), 0.0)
	return body
}

// One priced group is estimated at worst ~$0.32 (4 × (6,000 prompt + 2,048 reply) tokens at $10/M), over a
// $0.01 per-request budget: every start asks first and proceeds once acknowledged.
func TestAnalysisStarts_PerRequestBudgetNeedsAcknowledgement(t *testing.T) {
	prov := &fixedReplyProvider{reply: verdictReply}
	e := newQuickEnv(t, func(string) (failureanalysis.JobDeps, error) { return pricedDeps(prov), nil })
	setBudgets(t, e.s, 0.01, 0)
	result := map[string]string{"id": e.result.ID}

	body := requireBudget409(t, serveQuery(e.h.AnalyzeRunResult, "", result), "request")
	require.Contains(t, body["error"], "worst-case estimated cost")
	require.Zero(t, prov.calls, "nothing is sent before the acknowledgement")
	require.Equal(t, http.StatusCreated, serveQuery(e.h.AnalyzeRunResult, "?acknowledge_budget=true", result).Code)

	a := e.typeSafeRow(t, models.NarrativeStatusSkipped, models.DecisionStatusOK)
	explain := map[string]string{"id": e.result.ID, "analysisId": a.ID}
	requireBudget409(t, serveQuery(e.h.ExplainAnalysis, "", explain), "request")
	require.Equal(t, http.StatusOK, serveQuery(e.h.ExplainAnalysis, "?acknowledge_budget=true", explain).Code)

	run := map[string]string{"id": e.runID}
	requireBudget409(t, serveQuery(e.h.EnqueueRunAnalysis, "", run), "request")
	jobs, err := e.s.ListAnalysisJobsForRun(e.runID)
	require.NoError(t, err)
	require.Empty(t, jobs, "a refused start queues nothing")
	require.Equal(t, http.StatusCreated, serveQuery(e.h.EnqueueRunAnalysis, "?acknowledge_budget=true", run).Code)
}

func TestRetryFailed_BudgetNeedsAcknowledgement(t *testing.T) {
	e := newQuickEnv(t, func(string) (failureanalysis.JobDeps, error) {
		return pricedDeps(&fixedReplyProvider{reply: verdictReply}), nil
	})
	setBudgets(t, e.s, 0.01, 0)
	e.typeSafeRow(t, models.NarrativeStatusUnavailable, models.DecisionStatusFailed)
	run := map[string]string{"id": e.runID}

	requireBudget409(t, serveQuery(e.h.RetryFailedRunAnalysis, "", run), "request")
	rec := serveQuery(e.h.RetryFailedRunAnalysis, "?acknowledge_budget=true", run)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
}

// The monthly check is spent so far (generation + analysis) + this start's estimate.
func TestEnqueue_MonthlyBudgetCountsAnalysisSpend(t *testing.T) {
	e := newQuickEnv(t, func(string) (failureanalysis.JobDeps, error) {
		return pricedDeps(&fixedReplyProvider{reply: verdictReply}), nil
	})
	setBudgets(t, e.s, 0, 0.4)
	spent := 0.1 // the ~$0.32 worst case alone fits under $0.40, not on top of this
	require.NoError(t, e.s.RecordAnalysisCostEvent(&models.AIAnalysisCostEvent{
		Kind: models.AnalysisCostKindAnalysis, Engine: models.AnalysisCostEngineLLM, RunID: e.runID, EstimatedCost: &spent,
	}))

	body := requireBudget409(t, serveQuery(e.h.EnqueueRunAnalysis, "", map[string]string{"id": e.runID}), "month")
	require.InDelta(t, 0.1, body["month_spent_usd"], 1e-9)
	require.InDelta(t, 0.4, body["budget_usd"], 1e-9)
}

// An unpriced route has an unknown cost: budgets never hold it back.
func TestAnalysisStarts_UnpricedRouteIsNeverHeld(t *testing.T) {
	prov := &fixedReplyProvider{reply: verdictReply}
	e := newQuickEnv(t, func(string) (failureanalysis.JobDeps, error) {
		return failureanalysis.JobDeps{Narrative: prov, NarrativeModel: "mock"}, nil
	})
	setBudgets(t, e.s, 0.0001, 0.0001)
	require.Equal(t, http.StatusCreated, serveQuery(e.h.EnqueueRunAnalysis, "", map[string]string{"id": e.runID}).Code)
}
