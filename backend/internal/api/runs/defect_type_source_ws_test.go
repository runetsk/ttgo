package runs_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// R2: the single and bulk live result payloads carry defect_type_source, so a grid showing an
// "AI" badge drops it the moment a person's write lands. The bulk patch says "human" for a triage
// mode and "" for the status-change default.
func TestResultDeltasCarryDefectTypeSource(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	conn, done := dialRunsWS(t, env)
	defer done()

	runID, tcIDs := createRunWithCases(t, env, 1)
	created := createJSON(t, env, "/api/runs/"+runID+"/results", map[string]any{"test_case_id": tcIDs[0], "status": "FAIL"})
	id := created["id"].(string)

	single := readEventOfType(t, conn, "result_updated")
	rows := single["data"].(map[string]any)["results"].([]any)
	require.Len(t, rows, 1)
	assert.Contains(t, rows[0].(map[string]any), "defect_type_source", "full rows carry the source")

	rr := doRequest(env, http.MethodPost, "/api/runs/"+runID+"/results/bulk-update",
		map[string]any{"result_ids": []string{id}, "defect_type": "product_bug"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	bulk := readEventOfType(t, conn, "result_bulk_updated")
	patch := bulk["data"].(map[string]any)["patch"].(map[string]any)
	assert.Equal(t, "product_bug", patch["defect_type"])
	assert.Equal(t, "human", patch["defect_type_source"], "a bulk triage is a person's choice")
	assert.NotContains(t, patch, "suggested_auto_applied", "no SQL expression reaches the wire")

	rr = doRequest(env, http.MethodPost, "/api/runs/"+runID+"/results/bulk-update",
		map[string]any{"result_ids": []string{id}, "status": "ERROR"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	bulk = readEventOfType(t, conn, "result_bulk_updated")
	patch = bulk["data"].(map[string]any)["patch"].(map[string]any)
	assert.Equal(t, "to_investigate", patch["defect_type"])
	require.Contains(t, patch, "defect_type_source")
	assert.Equal(t, "", patch["defect_type_source"], "the status-change default is nobody's choice")
}
