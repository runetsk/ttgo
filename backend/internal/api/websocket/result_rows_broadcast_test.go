package websocket

import (
	"encoding/json"
	"testing"
	"time"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

// Auto-apply republishes the changed rows as the result_updated delta the runs handlers send,
// so the run grid merges them with applyResultDelta and shows the AI badge live.
func TestBroadcastRunResultsUpdated_SendsAResultDelta(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	client := makeTestClient(hub, RoleMember, runTopic("run-9"))
	hub.register <- client
	time.Sleep(50 * time.Millisecond)
	drainAck(t, client)

	run := &models.TestRun{ID: "run-9", Name: "nightly"}
	rows := []*models.RunResult{{ID: "rr-1", TestRunID: "run-9", Status: models.StatusFail,
		DefectType: "product_bug", DefectTypeSource: models.DefectTypeSourceAI}}
	(&RunAnalysisBroadcaster{Hub: hub}).BroadcastRunResultsUpdated(run, rows)

	select {
	case msg := <-client.send:
		var ev struct {
			Type string `json:"type"`
			Data struct {
				RunID   string                   `json:"run_id"`
				Run     map[string]interface{}   `json:"run"`
				Results []map[string]interface{} `json:"results"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(msg, &ev))
		require.Equal(t, EventResultUpdated, ev.Type)
		require.Equal(t, "run-9", ev.Data.RunID)
		require.Equal(t, "run-9", ev.Data.Run["id"])
		require.Len(t, ev.Data.Results, 1)
		require.Equal(t, "product_bug", ev.Data.Results[0]["defect_type"])
		require.Equal(t, models.DefectTypeSourceAI, ev.Data.Results[0]["defect_type_source"])
	case <-time.After(time.Second):
		t.Fatal("expected a result_updated broadcast, timed out")
	}
}
