package runs_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// addSingleResult attaches one test case to a run via the single-add endpoint.
func addSingleResult(t *testing.T, env *testEnv, runID, testCaseID string) {
	t.Helper()
	rr := doRequest(env, http.MethodPost, "/api/runs/"+runID+"/results", map[string]any{"test_case_id": testCaseID})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
}

func bulkAdd(env *testEnv, runID string, ids []string) *httptest.ResponseRecorder {
	return doRequest(env, http.MethodPost, "/api/runs/"+runID+"/results/bulk", map[string]any{"test_case_ids": ids})
}

func TestBulkAddResultsAddsNewTests(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	ids := seedTestCases(t, env, 3)
	run := createJSON(t, env, "/api/runs", map[string]any{"name": "Bulk Add Run"})
	runID := run["id"].(string)

	rr := bulkAdd(env, runID, []string{ids[0], ids[1]})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	var created []map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &created))
	require.Len(t, created, 2)
	for _, row := range created {
		assert.Equal(t, "PENDING", row["status"])
		assert.EqualValues(t, 1, row["attempt_number"])
	}

	full := doRequest(env, http.MethodGet, "/api/runs/"+runID, nil)
	var run2 map[string]any
	require.NoError(t, json.Unmarshal(full.Body.Bytes(), &run2))
	assert.Len(t, run2["run_results"].([]any), 2)
}

func TestBulkAddResultsSkipsAlreadyPresent(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	ids := seedTestCases(t, env, 2)
	run := createJSON(t, env, "/api/runs", map[string]any{"name": "Skip Run"})
	runID := run["id"].(string)
	addSingleResult(t, env, runID, ids[0]) // ids[0] already in the run

	rr := bulkAdd(env, runID, []string{ids[0], ids[1]})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	var created []map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &created))
	require.Len(t, created, 1) // only ids[1] is new; ids[0] skipped, no duplicate attempt

	full := doRequest(env, http.MethodGet, "/api/runs/"+runID, nil)
	var run2 map[string]any
	require.NoError(t, json.Unmarshal(full.Body.Bytes(), &run2))
	assert.Len(t, run2["run_results"].([]any), 2)
}

func TestBulkAddResultsDedupesInput(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	ids := seedTestCases(t, env, 1)
	run := createJSON(t, env, "/api/runs", map[string]any{"name": "Dedup Run"})
	runID := run["id"].(string)

	rr := bulkAdd(env, runID, []string{ids[0], ids[0], ids[0]})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	var created []map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &created))
	assert.Len(t, created, 1)
}

func TestBulkAddResultsRejectsUnknownTestCase(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	ids := seedTestCases(t, env, 1)
	run := createJSON(t, env, "/api/runs", map[string]any{"name": "Unknown TC Run"})
	runID := run["id"].(string)

	rr := bulkAdd(env, runID, []string{ids[0], "no-such-id"})
	assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "test_case_ids")
}

func TestBulkAddResultsRejectsEmpty(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	run := createJSON(t, env, "/api/runs", map[string]any{"name": "Empty Run"})
	runID := run["id"].(string)

	rr := bulkAdd(env, runID, []string{})
	assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "test_case_ids is required")
}

func TestBulkAddResultsRejectsUnknownRun(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	ids := seedTestCases(t, env, 1)

	rr := bulkAdd(env, "no-such-run", []string{ids[0]})
	assert.Equal(t, http.StatusNotFound, rr.Code, rr.Body.String())
}

func TestBulkAddResultsRejectsAllEmptyStrings(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	run := createJSON(t, env, "/api/runs", map[string]any{"name": "Empty Strings Run"})
	runID := run["id"].(string)

	rr := bulkAdd(env, runID, []string{"", ""})
	assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "test_case_ids is required")
}

func TestBulkAddResultsRejectsTooMany(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	run := createJSON(t, env, "/api/runs", map[string]any{"name": "Too Many Run"})
	runID := run["id"].(string)

	ids := make([]string, 501)
	for i := range ids {
		ids[i] = "id-" + strconv.Itoa(i)
	}
	rr := bulkAdd(env, runID, ids)
	assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "too many")
}

// All requested test cases already in the run → 201 with an empty array and no
// new rows (the handler skips the websocket broadcast when nothing was created).
func TestBulkAddResultsAllPresentReturnsEmpty(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	ids := seedTestCases(t, env, 1)
	run := createJSON(t, env, "/api/runs", map[string]any{"name": "All Present Run"})
	runID := run["id"].(string)
	addSingleResult(t, env, runID, ids[0])

	rr := bulkAdd(env, runID, []string{ids[0]})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	var created []map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &created))
	assert.Empty(t, created)

	// No duplicate attempt was created — the run still holds exactly the one row.
	full := doRequest(env, http.MethodGet, "/api/runs/"+runID, nil)
	var run2 map[string]any
	require.NoError(t, json.Unmarshal(full.Body.Bytes(), &run2))
	assert.Len(t, run2["run_results"].([]any), 1)
}

func TestBulkAddResultsSnapshotsTestNames(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	ids := seedTestCases(t, env, 2) // seedTestCases names them "Pick Case A", "Pick Case B"
	run := createJSON(t, env, "/api/runs", map[string]any{"name": "Snapshot Run"})
	runID := run["id"].(string)

	rr := bulkAdd(env, runID, []string{ids[0], ids[1]})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	var created []map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &created))
	require.Len(t, created, 2)

	names := map[string]bool{}
	for _, row := range created {
		names[row["test_name_snapshot"].(string)] = true
	}
	assert.True(t, names["Pick Case A"], "created rows should snapshot the test case names")
	assert.True(t, names["Pick Case B"], "created rows should snapshot the test case names")
}
