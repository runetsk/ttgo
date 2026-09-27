package runs_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// enableAutoAnalysis switches on "Auto-analyze on run completion", keeping the other settings.
func enableAutoAnalysis(t *testing.T, env *testEnv) {
	t.Helper()
	rr := doRequest(env, http.MethodGet, "/api/settings/ai-failure-analysis", nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var fa map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &fa))
	fa["enabled_on_completion"] = true
	rr = doRequest(env, http.MethodPut, "/api/settings/ai-failure-analysis", fa)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
}

// completeRunWithFailure records one FAIL result, completes the run and returns the run's
// latest analysis job (nil when none was queued).
func completeRunWithFailure(t *testing.T, env *testEnv) map[string]any {
	t.Helper()
	runID, tcIDs := createRunWithCases(t, env, 1)
	createJSON(t, env, "/api/runs/"+runID+"/results", map[string]any{"test_case_id": tcIDs[0], "status": "FAIL"})
	rr := doRequest(env, http.MethodPost, "/api/runs/"+runID+"/complete", nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	rr = doRequest(env, http.MethodGet, "/api/runs/"+runID+"/analysis-job", nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var job map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &job))
	return job
}

// A TypeSafe-only setup is analyzed on completion: no LLM provider exists at all, and TypeSafe
// is allowed on automatic analysis.
func TestCompleteRunQueuesTypeSafeOnlyAutoAnalysis(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	enableAutoAnalysis(t, env)
	rr := doRequest(env, http.MethodPut, "/api/settings/typesafe", map[string]any{
		"api_key": "ts-key-1234", "enabled": true, "allow_auto_failure_analysis": true,
	})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	job := completeRunWithFailure(t, env)
	require.NotNil(t, job, "TypeSafe may decide alone, so completion queues the analysis")
	assert.Equal(t, "auto_on_completion", job["trigger"])
}

// Without TypeSafe's automatic consent and without any LLM, nothing may analyze: nothing is queued.
func TestCompleteRunSkipsAutoAnalysisWhenNothingMayAnalyze(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	enableAutoAnalysis(t, env)
	rr := doRequest(env, http.MethodPut, "/api/settings/typesafe", map[string]any{
		"api_key": "ts-key-1234", "enabled": true, "allow_auto_failure_analysis": false,
	})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	assert.Nil(t, completeRunWithFailure(t, env))
}
func setMonthlyBudget(t *testing.T, env *testEnv, usd float64) {
	t.Helper()
	rr := doRequest(env, http.MethodPut, "/api/settings/ai-budgets", map[string]any{"monthly_usd": usd})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
}

func enableTypeSafeAuto(t *testing.T, env *testEnv) {
	t.Helper()
	rr := doRequest(env, http.MethodPut, "/api/settings/typesafe", map[string]any{
		"api_key": "ts-key-1234", "enabled": true, "allow_auto_failure_analysis": true,
	})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
}

// Spent + this run's estimate over the monthly budget: the automatic analysis is recorded as a
// skipped job instead of being queued, and "Run anyway" (an acknowledged manual start) works.
func TestCompleteRunSkipsAutoAnalysisOverMonthlyBudget(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	enableAutoAnalysis(t, env)
	enableTypeSafeAuto(t, env)
	setMonthlyBudget(t, env, 0.0001) // one TypeSafe group is estimated at ~$0.00067

	job := completeRunWithFailure(t, env)
	require.NotNil(t, job)
	assert.Equal(t, "skipped", job["status"])
	assert.Equal(t, "auto_on_completion", job["trigger"])
	assert.Equal(t, "budget", job["skip_reason"])
	assert.Greater(t, job["skip_estimate_usd"].(float64), 0.0001)
	assert.Equal(t, 0.0, job["skip_spent_usd"])
	assert.InDelta(t, 0.0001, job["skip_budget_usd"], 1e-12)
	assert.NotNil(t, job["completed_at"])

	// "Run anyway": the skipped job is not active, so an acknowledged manual start is queued.
	// (The manual 409 itself is covered in internal/api/ai/analysis_budget_test.go; this test
	// server does not wire the failure-analysis resolver into the AI handler.)
	runID := job["test_run_id"].(string)
	rr := doRequest(env, http.MethodPost, "/api/runs/"+runID+"/analyze-failures?acknowledge_budget=true", nil)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
}

func TestCompleteRunQueuesAutoAnalysisWithinBudget(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	enableAutoAnalysis(t, env)
	enableTypeSafeAuto(t, env)
	setMonthlyBudget(t, env, 5)

	job := completeRunWithFailure(t, env)
	require.NotNil(t, job)
	assert.Equal(t, "queued", job["status"])
}
