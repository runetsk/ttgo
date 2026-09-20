package websocket

import (
	"encoding/json"
	"testing"
	"time"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

// TestBroadcastRunResultAnalysisCreated_IncludesSuggestedDefectType asserts the
// live payload carries the PERSISTED suggestion (models.RunResultAnalysis.SuggestedDefectType)
// next to the raw verdict, so a client applying a WS event lands on the same suggestion the REST
// endpoints return. The broadcaster no longer derives it from the verdict — CreateAnalysis writes
// it at analysis time (spec §5) — so this row is seeded with the value that write-time default
// would have produced, the same way a persisted row reaches this code.
func TestBroadcastRunResultAnalysisCreated_IncludesSuggestedDefectType(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	a := &models.RunResultAnalysis{
		ID:                  "an-1",
		RunResultID:         "rr-1",
		Version:             1,
		Verdict:             models.VerdictFlakyTest,
		Confidence:          models.ConfidenceHigh,
		SuggestedDefectType: "automation_bug",
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

// TestBroadcastRunResultAnalysisCreated_FullTypeSafePayload covers F2: the frontend reads the
// live event by exact key name and REPLACES its in-memory analysis object with it (no merge
// with whatever the REST fetch returned), so every key the TypeSafe engine populates —
// engine, model_name, confidence_score, suggested_defect_type_confidence, narrative_status,
// dedup_method, dedup_p_same — has to be on the wire, not just verdict/suggested_defect_type.
func TestBroadcastRunResultAnalysisCreated_FullTypeSafePayload(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	confidenceScore := 0.87
	defectTypeConfidence := 0.91
	dedupGroupKey := "grp-1"
	dedupPSame := 0.93

	a := &models.RunResultAnalysis{
		ID:                            "an-2",
		RunResultID:                   "rr-2",
		Version:                       1,
		Verdict:                       models.VerdictFlakyTest,
		Confidence:                    models.ConfidenceHigh,
		Engine:                        models.AnalysisEngineTypeSafe,
		ModelName:                     "jev-1.13.0",
		ConfidenceScore:               &confidenceScore,
		SuggestedDefectType:           "automation_bug",
		SuggestedDefectTypeConfidence: &defectTypeConfidence,
		NarrativeStatus:               models.NarrativeStatusOK,
		DedupGroupKey:                 &dedupGroupKey,
		DedupMethod:                   models.DedupMethodSemantic,
		DedupPSame:                    &dedupPSame,
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
		require.Equal(t, a.RunResultID, ev.Data["run_result_id"])
		require.Equal(t, a.ID, ev.Data["analysis_id"])
		require.Equal(t, float64(a.Version), ev.Data["version"])
		require.Equal(t, models.VerdictFlakyTest, ev.Data["verdict"])
		require.Equal(t, "automation_bug", ev.Data["suggested_defect_type"])
		require.Equal(t, defectTypeConfidence, ev.Data["suggested_defect_type_confidence"])
		require.Equal(t, models.ConfidenceHigh, ev.Data["confidence"])
		require.Equal(t, confidenceScore, ev.Data["confidence_score"])
		require.Equal(t, models.AnalysisEngineTypeSafe, ev.Data["engine"])
		require.Equal(t, "jev-1.13.0", ev.Data["model_name"])
		require.Equal(t, models.NarrativeStatusOK, ev.Data["narrative_status"])
		require.Equal(t, dedupGroupKey, ev.Data["dedup_group_key"])
		require.Equal(t, models.DedupMethodSemantic, ev.Data["dedup_method"])
		require.Equal(t, dedupPSame, ev.Data["dedup_p_same"])
	case <-time.After(time.Second):
		t.Fatal("expected run_result_analysis.created broadcast, timed out")
	}
}
