package runs_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Completing a run must notify the open detail page: a run_updated event on
// the exact "run:{id}" topic (runs:* subscribers already receive it via the
// wildcard mapping, but exact-topic subscribers must too — parity with ReopenRun).
func TestCompleteRunBroadcastsRunTopic(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	conn, done := dialRunsWS(t, env) // subscribes runs:*, which also matches run:{id}
	defer done()

	runID, tcIDs := createRunWithCases(t, env, 1)
	createJSON(t, env, "/api/runs/"+runID+"/results",
		map[string]any{"test_case_id": tcIDs[0], "status": "PASS"})

	rr := doRequest(env, http.MethodPost, "/api/runs/"+runID+"/complete", nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	// Read frames until a run_updated arrives on the exact run:{id} topic.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		require.NoError(t, conn.SetReadDeadline(deadline))
		_, raw, err := conn.ReadMessage()
		require.NoError(t, err)
		var msg map[string]any
		require.NoError(t, json.Unmarshal(raw, &msg))
		if msg["type"] == "run_updated" && msg["topic"] == "run:"+runID {
			data := msg["data"].(map[string]any)
			assert.Equal(t, "PASS", data["status"])
			return
		}
	}
	t.Fatal("no run_updated event on topic run:{id} before deadline")
}
