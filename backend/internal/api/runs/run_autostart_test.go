package runs_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getRunStatus(t *testing.T, env *testEnv, runID string) string {
	t.Helper()
	rr := doRequest(env, http.MethodGet, "/api/runs/"+runID, nil)
	require.Equal(t, http.StatusOK, rr.Code)
	var run map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &run))
	return run["status"].(string)
}

func TestManualResultUpdateAutoStartsRun(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	runID, tcIDs := createRunWithCases(t, env, 1)
	created := createJSON(t, env, "/api/runs/"+runID+"/results",
		map[string]any{"test_case_id": tcIDs[0]}) // defaults to PENDING
	require.Equal(t, "PENDING", getRunStatus(t, env, runID))

	rr := doRequest(env, http.MethodPut,
		"/api/runs/"+runID+"/results/"+created["id"].(string),
		map[string]any{"status": "PASS"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	assert.Equal(t, "RUNNING", getRunStatus(t, env, runID))
}

func TestBulkResultUpdateAutoStartsRun(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	runID, tcIDs := createRunWithCases(t, env, 2)
	var ids []string
	for _, tcID := range tcIDs {
		created := createJSON(t, env, "/api/runs/"+runID+"/results",
			map[string]any{"test_case_id": tcID})
		ids = append(ids, created["id"].(string))
	}

	rr := doRequest(env, http.MethodPost, "/api/runs/"+runID+"/results/bulk-update",
		map[string]any{"result_ids": ids, "status": "SKIP"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	assert.Equal(t, "RUNNING", getRunStatus(t, env, runID))
}

func TestResultUpdateDoesNotReopenCompletedRun(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	runID, tcIDs := createRunWithCases(t, env, 1)
	created := createJSON(t, env, "/api/runs/"+runID+"/results",
		map[string]any{"test_case_id": tcIDs[0], "status": "PASS"})

	rr := doRequest(env, http.MethodPost, "/api/runs/"+runID+"/complete", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, "PASS", getRunStatus(t, env, runID))

	// Editing a result of a completed run must not flip it back to RUNNING.
	rr = doRequest(env, http.MethodPut,
		"/api/runs/"+runID+"/results/"+created["id"].(string),
		map[string]any{"status": "FAIL"})
	require.Equal(t, http.StatusOK, rr.Code)

	assert.Equal(t, "PASS", getRunStatus(t, env, runID))
}
