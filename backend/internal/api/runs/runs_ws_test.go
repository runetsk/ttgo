package runs_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dialRunsWS opens an authenticated WS on a live httptest server wrapping
// env.srv, consumes the "connected" ack, and subscribes to runs:*. The same
// srv instance backs doRequest calls, so REST writes broadcast to this socket.
func dialRunsWS(t *testing.T, env *testEnv) (*websocket.Conn, func()) {
	t.Helper()
	ts := httptest.NewServer(env.srv)
	url := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/ws"
	header := http.Header{}
	header.Set("Origin", "http://localhost:5173")
	header.Set("Cookie", "session_token="+env.sessionToken)
	conn, _, err := websocket.DefaultDialer.Dial(url, header)
	require.NoError(t, err)

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(3*time.Second)))
	_, _, err = conn.ReadMessage() // "connected" ack
	require.NoError(t, err)
	require.NoError(t, conn.WriteJSON(map[string]string{"action": "subscribe", "topic": "runs:*"}))
	// Subscription is registered by the client's read pump; give it a beat
	// before the first broadcast-triggering request.
	time.Sleep(200 * time.Millisecond)

	return conn, func() { conn.Close(); ts.Close() }
}

// readEventOfType reads frames until one with the wanted type arrives.
func readEventOfType(t *testing.T, conn *websocket.Conn, eventType string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		require.NoError(t, conn.SetReadDeadline(deadline))
		_, raw, err := conn.ReadMessage()
		require.NoError(t, err)
		var msg map[string]any
		require.NoError(t, json.Unmarshal(raw, &msg))
		if msg["type"] == eventType {
			return msg
		}
	}
	t.Fatalf("no %s event before deadline", eventType)
	return nil
}

func createJSON(t *testing.T, env *testEnv, path string, body map[string]any) map[string]any {
	t.Helper()
	rr := doRequest(env, http.MethodPost, path, body)
	require.Equal(t, http.StatusCreated, rr.Code, "%s: %s", path, rr.Body.String())
	var out map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	return out
}

// createRunWithCases seeds a folder, n test cases, and one run via the API.
func createRunWithCases(t *testing.T, env *testEnv, n int) (runID string, tcIDs []string) {
	t.Helper()
	folder := createJSON(t, env, "/api/folders", map[string]any{"name": "WS Folder"})
	for i := 0; i < n; i++ {
		tc := createJSON(t, env, "/api/tests", map[string]any{
			"name":      "WS Case " + string(rune('A'+i)),
			"folder_id": folder["id"],
		})
		tcIDs = append(tcIDs, tc["id"].(string))
	}
	run := createJSON(t, env, "/api/runs", map[string]any{"name": "WS Run"})
	return run["id"].(string), tcIDs
}

func TestAddResultBroadcastsDelta(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	conn, done := dialRunsWS(t, env)
	defer done()

	runID, tcIDs := createRunWithCases(t, env, 1)
	created := createJSON(t, env, "/api/runs/"+runID+"/results",
		map[string]any{"test_case_id": tcIDs[0], "status": "PASS"})

	msg := readEventOfType(t, conn, "result_updated")
	data := msg["data"].(map[string]any)
	assert.Equal(t, runID, data["run_id"])

	run := data["run"].(map[string]any)
	assert.Equal(t, runID, run["id"])
	_, hasResults := run["run_results"]
	assert.False(t, hasResults, "summary must not embed result rows")
	assert.EqualValues(t, 1, run["total_results"])
	assert.EqualValues(t, 1, run["passed_results"])

	results := data["results"].([]any)
	require.Len(t, results, 1)
	row := results[0].(map[string]any)
	assert.Equal(t, created["id"], row["id"])
	assert.Equal(t, "PASS", row["status"])
	assert.Equal(t, "WS Case A", row["test_name_snapshot"])
	require.NotNil(t, row["test_case"], "row must carry its preloaded test case")
	assert.Equal(t, tcIDs[0], row["test_case"].(map[string]any)["id"])
}

func TestBulkUpdateBroadcastsPatch(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	conn, done := dialRunsWS(t, env)
	defer done()

	runID, tcIDs := createRunWithCases(t, env, 2)
	var ids []string
	for _, tcID := range tcIDs {
		created := createJSON(t, env, "/api/runs/"+runID+"/results",
			map[string]any{"test_case_id": tcID, "status": "PENDING"})
		ids = append(ids, created["id"].(string))
	}

	rr := doRequest(env, http.MethodPost, "/api/runs/"+runID+"/results/bulk-update",
		map[string]any{"result_ids": ids, "status": "PASS"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	msg := readEventOfType(t, conn, "result_bulk_updated")
	data := msg["data"].(map[string]any)
	assert.Equal(t, runID, data["run_id"])
	assert.ElementsMatch(t, []any{ids[0], ids[1]}, data["result_ids"].([]any))
	patch := data["patch"].(map[string]any)
	assert.Equal(t, "PASS", patch["status"])
	assert.Equal(t, "", patch["defect_type"])
	assert.Contains(t, patch, "updated_at")
	_, hasRows := data["results"]
	assert.False(t, hasRows, "bulk delta carries ids+patch, not rows")
	run := data["run"].(map[string]any)
	assert.EqualValues(t, 2, run["passed_results"])
}

func TestRetryBroadcastsNewRow(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	conn, done := dialRunsWS(t, env)
	defer done()

	runID, tcIDs := createRunWithCases(t, env, 1)
	created := createJSON(t, env, "/api/runs/"+runID+"/results",
		map[string]any{"test_case_id": tcIDs[0], "status": "FAIL"})

	rr := doRequest(env, http.MethodPost,
		"/api/runs/"+runID+"/results/"+created["id"].(string)+"/retry", nil)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	msg := readEventOfType(t, conn, "result_retried")
	data := msg["data"].(map[string]any)
	results := data["results"].([]any)
	require.Len(t, results, 1)
	row := results[0].(map[string]any)
	assert.EqualValues(t, 2, row["attempt_number"])
	assert.Equal(t, "PENDING", row["status"])
	run := data["run"].(map[string]any)
	assert.Equal(t, "RUNNING", run["status"])
	assert.EqualValues(t, 1, run["retried_count"])
	assert.EqualValues(t, 2, run["total_attempts"])
}

func TestDeleteResultBroadcastsDeletedIDs(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	conn, done := dialRunsWS(t, env)
	defer done()

	runID, tcIDs := createRunWithCases(t, env, 1)
	created := createJSON(t, env, "/api/runs/"+runID+"/results",
		map[string]any{"test_case_id": tcIDs[0], "status": "PASS"})

	rr := doRequest(env, http.MethodDelete,
		"/api/runs/"+runID+"/results/"+created["id"].(string), nil)
	require.Equal(t, http.StatusNoContent, rr.Code, rr.Body.String())

	msg := readEventOfType(t, conn, "result_deleted")
	data := msg["data"].(map[string]any)
	assert.Equal(t, []any{created["id"]}, data["deleted_result_ids"].([]any))
	run := data["run"].(map[string]any)
	assert.EqualValues(t, 0, run["total_results"])
}

// TestDeltaFramesStayBounded pins the S4 fix: frame size must not grow with
// the number of results already in the run (the old payload was the full
// re-fetched run, so frame N carried all N results).
func TestDeltaFramesStayBounded(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	conn, done := dialRunsWS(t, env)
	defer done()

	runID, tcIDs := createRunWithCases(t, env, 1)
	const n = 30
	for i := 0; i < n; i++ {
		// Same test case: attempt_number auto-increments, so the run
		// accumulates n rows.
		createJSON(t, env, "/api/runs/"+runID+"/results",
			map[string]any{"test_case_id": tcIDs[0], "status": "PASS"})
	}

	var first, last, seen int
	for seen < n {
		require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
		_, raw, err := conn.ReadMessage()
		require.NoError(t, err)
		var msg map[string]any
		require.NoError(t, json.Unmarshal(raw, &msg))
		if msg["type"] != "result_updated" {
			continue
		}
		seen++
		if first == 0 {
			first = len(raw)
		}
		last = len(raw)
	}
	assert.Less(t, last, 4096, "delta frame must stay ~O(1)")
	assert.Less(t, last, first*2, "frame size must not scale with run size")
}
