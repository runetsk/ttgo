package websocket

import (
	"encoding/json"
	"testing"
	"time"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

// TestBroadcastRunResultAnalysisCreated_IncludesSuggestedDefectType asserts the
// live payload carries the verdict-derived suggestion next to the raw verdict, so
// a client applying a WS event lands on the same suggestion the REST endpoints
// return. The mapping is lossy (flaky_test → automation_bug), so echoing the
// verdict alone would not be equivalent.
func TestBroadcastRunResultAnalysisCreated_IncludesSuggestedDefectType(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	a := &models.RunResultAnalysis{
		ID:          "an-1",
		RunResultID: "rr-1",
		Version:     1,
		Verdict:     models.VerdictFlakyTest,
		Confidence:  models.ConfidenceHigh,
	}

	client := makeTestClient(hub, RoleMember, runResultTopic(a.RunResultID))
	hub.register <- client
	time.Sleep(50 * time.Millisecond)
	drainAck(t, client)

	(&RunAnalysisBroadcaster{Hub: hub}).BroadcastRunResultAnalysisCreated(a, "")

	select {
	case msg := <-client.send:
		var ev struct {
			Type string                 `json:"type"`
			Data map[string]interface{} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(msg, &ev))
		require.Equal(t, EventRunResultAnalysisCreated, ev.Type)
		require.Equal(t, models.VerdictFlakyTest, ev.Data["verdict"])
		require.Equal(t, "automation_bug", ev.Data["suggested_defect_type"])
	case <-time.After(time.Second):
		t.Fatal("expected run_result_analysis.created broadcast, timed out")
	}
}
