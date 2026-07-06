package runs_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedTestCases creates a folder with n test cases and returns their IDs.
func seedTestCases(t *testing.T, env *testEnv, n int) []string {
	t.Helper()
	folder := createJSON(t, env, "/api/folders", map[string]any{"name": "Pick Folder"})
	var ids []string
	for i := 0; i < n; i++ {
		tc := createJSON(t, env, "/api/tests", map[string]any{
			"name":      "Pick Case " + string(rune('A'+i)),
			"folder_id": folder["id"],
		})
		ids = append(ids, tc["id"].(string))
	}
	return ids
}

func TestCreateRunWithPickedTests(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	ids := seedTestCases(t, env, 3)

	run := createJSON(t, env, "/api/runs", map[string]any{
		"name":          "Picked Run",
		"test_case_ids": []string{ids[0], ids[2]},
	})

	rr := doRequest(env, http.MethodGet, "/api/runs/"+run["id"].(string), nil)
	require.Equal(t, http.StatusOK, rr.Code)
	var full map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &full))

	results := full["run_results"].([]any)
	require.Len(t, results, 2)
	names := map[string]bool{}
	for _, r := range results {
		row := r.(map[string]any)
		assert.Equal(t, "PENDING", row["status"])
		names[row["test_name_snapshot"].(string)] = true
	}
	assert.True(t, names["Pick Case A"])
	assert.True(t, names["Pick Case C"])
}

func TestCreateRunWithPickedTestsDedupes(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	ids := seedTestCases(t, env, 1)

	run := createJSON(t, env, "/api/runs", map[string]any{
		"name":          "Dupe Run",
		"test_case_ids": []string{ids[0], ids[0], ids[0]},
	})

	rr := doRequest(env, http.MethodGet, "/api/runs/"+run["id"].(string), nil)
	require.Equal(t, http.StatusOK, rr.Code)
	var full map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &full))
	assert.Len(t, full["run_results"].([]any), 1)
}

func TestCreateRunRejectsUnknownTestCase(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	ids := seedTestCases(t, env, 1)

	rr := doRequest(env, http.MethodPost, "/api/runs", map[string]any{
		"name":          "Bad Run",
		"test_case_ids": []string{ids[0], "no-such-id"},
	})
	assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "test_case_ids")
}

func TestCreateRunRejectsCategoryPlusPickedTests(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	ids := seedTestCases(t, env, 1)
	cat := createJSON(t, env, "/api/categories", map[string]any{"name": "Pick Cat"})

	rr := doRequest(env, http.MethodPost, "/api/runs", map[string]any{
		"name":          "Conflicted Run",
		"category_id":   cat["id"],
		"test_case_ids": []string{ids[0]},
	})
	assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "not both")
}
