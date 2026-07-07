package runs_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateRunResultPersistsSteps(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	runID, tcIDs := createRunWithCases(t, env, 1)
	created := createJSON(t, env, "/api/runs/"+runID+"/results",
		map[string]any{"test_case_id": tcIDs[0]})
	resultID := created["id"].(string)

	steps := []map[string]any{
		{"order_index": 0, "action": "<p>Open login</p>", "expected_result": "<p>Form shows</p>", "status": "PASS", "note": ""},
		{"order_index": 1, "action": "<p>Submit</p>", "expected_result": "<p>Dashboard</p>", "status": "FAIL", "note": "500 page"},
	}
	rr := doRequest(env, http.MethodPut, "/api/runs/"+runID+"/results/"+resultID,
		map[string]any{"status": "FAIL", "steps": steps})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	got := doRequest(env, http.MethodGet, "/api/runs/"+runID, nil)
	require.Equal(t, http.StatusOK, got.Code)
	var run map[string]any
	require.NoError(t, json.Unmarshal(got.Body.Bytes(), &run))
	results := run["run_results"].([]any)
	var target map[string]any
	for _, r := range results {
		if r.(map[string]any)["id"] == resultID {
			target = r.(map[string]any)
		}
	}
	require.NotNil(t, target)
	assert.Equal(t, "FAIL", target["status"])
	gotSteps := target["steps"].([]any)
	require.Len(t, gotSteps, 2)
	assert.Equal(t, "PASS", gotSteps[0].(map[string]any)["status"])
	assert.Equal(t, "FAIL", gotSteps[1].(map[string]any)["status"])
	assert.Equal(t, "500 page", gotSteps[1].(map[string]any)["note"])
}

func TestUpdateRunResultRejectsNonArraySteps(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	runID, tcIDs := createRunWithCases(t, env, 1)
	created := createJSON(t, env, "/api/runs/"+runID+"/results",
		map[string]any{"test_case_id": tcIDs[0]})
	resultID := created["id"].(string)

	rr := doRequest(env, http.MethodPut, "/api/runs/"+runID+"/results/"+resultID,
		map[string]any{"steps": map[string]any{"not": "an array"}})
	assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "steps")
}
