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
		SuggestionSource:              models.SuggestionSourceVerdict,
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
		require.Equal(t, models.SuggestionSourceVerdict, ev.Data["suggestion_source"])
		require.Equal(t, "jev-1.13.0", ev.Data["model_name"])
		require.Equal(t, models.NarrativeStatusOK, ev.Data["narrative_status"])
		require.Equal(t, dedupGroupKey, ev.Data["dedup_group_key"])
		require.Equal(t, models.DedupMethodSemantic, ev.Data["dedup_method"])
		require.Equal(t, dedupPSame, ev.Data["dedup_p_same"])
	case <-time.After(time.Second):
		t.Fatal("expected run_result_analysis.created broadcast, timed out")
	}
}

// Spec §A5: both events carry the whole row the UI renders, from one builder, so a live merge
// never needs a refetch and never swaps a fuller row for a partial one.
func TestAnalysisPayload_CarriesTheFullUIRow(t *testing.T) {
	src := "an-rep"
	job := "job-1"
	created := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	a := &models.RunResultAnalysis{ID: "an-3", RunResultID: "rr-3", Version: 2, Verdict: models.VerdictFlakyTest,
		Confidence: models.ConfidenceHigh, Engine: models.AnalysisEngineTypeSafe, Summary: "S", NextAction: "N",
		Rationale: "[Grouped from representative analysis] R", NarrativeStatus: models.NarrativeStatusOK,
		NarrativeRevision: 3, SourceAnalysisID: &src, CreatedAt: created, PolicyVersion: "fa-verdict-v5",
		HistoryAvailable: true, JobID: &job, Signals: `{"injection":0.03}`}
	p := analysisPayload(a)
	for _, key := range []string{
		"id", "analysis_id", "run_result_id", "version", "verdict", "confidence", "confidence_score", "engine", "model_name",
		"narrative_status", "suggested_defect_type", "suggested_defect_type_confidence", "suggestion_source",
		"dedup_group_key", "dedup_method", "dedup_p_same", "decision_status", "error_category",
		"takeover_from_verdict", "takeover_from_confidence", "job_id",
		"summary", "next_action", "rationale", "narrative_revision", "source_analysis_id", "created_at",
		"policy_version", "history_available", "signals",
	} {
		require.Contains(t, p, key)
	}
	require.Equal(t, "an-3", p["id"])
	require.Equal(t, "S", p["summary"])
	require.Equal(t, "N", p["next_action"])
	require.Equal(t, "[Grouped from representative analysis] R", p["rationale"])
	require.Equal(t, 3, p["narrative_revision"])
	require.Equal(t, &src, p["source_analysis_id"])
	require.Equal(t, created, p["created_at"])
	require.Equal(t, "fa-verdict-v5", p["policy_version"])
	require.Equal(t, true, p["history_available"])
	require.Equal(t, `{"injection":0.03}`, p["signals"], "the stored JSON string, as REST rows carry it")
}

func TestBroadcastRunResultAnalysisUpdated_SamePayloadAsCreated(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	a := &models.RunResultAnalysis{ID: "an-4", RunResultID: "rr-4", Version: 1, Verdict: models.VerdictFlakyTest,
		Confidence: models.ConfidenceHigh, Summary: "Shared timing race", NarrativeStatus: models.NarrativeStatusOK, NarrativeRevision: 1}
	client := makeTestClient(hub, RoleMember, runTopic("run-4"))
	hub.register <- client
	time.Sleep(50 * time.Millisecond)
	drainAck(t, client)

	b := &RunAnalysisBroadcaster{Hub: hub}
	b.BroadcastRunResultAnalysisCreated(a, "run-4")
	b.BroadcastRunResultAnalysisUpdated(a, "run-4")

	read := func() (string, map[string]interface{}) {
		select {
		case msg := <-client.send:
			var ev struct {
				Type string                 `json:"type"`
				Data map[string]interface{} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(msg, &ev))
			return ev.Type, ev.Data
		case <-time.After(time.Second):
			t.Fatal("expected an analysis broadcast, timed out")
			return "", nil
		}
	}
	createdType, createdData := read()
	updatedType, updatedData := read()
	require.Equal(t, EventRunResultAnalysisCreated, createdType)
	require.Equal(t, EventRunResultAnalysisUpdated, updatedType)
	require.Equal(t, "Shared timing race", updatedData["summary"])
	require.Equal(t, float64(1), updatedData["narrative_revision"])
	require.Equal(t, createdData, updatedData, "one builder for both events")
}

func TestAnalysisPayload_CarriesTheTransferFit(t *testing.T) {
	fit := 0.31
	p := analysisPayload(&models.RunResultAnalysis{ID: "an-9", NarrativeFit: &fit, NarrativeSplit: true})
	require.Equal(t, &fit, p["narrative_fit"])
	require.Equal(t, true, p["narrative_split"])

	p = analysisPayload(&models.RunResultAnalysis{ID: "an-10"})
	require.Contains(t, p, "narrative_fit", "an unchecked row says so explicitly (null), so a live merge clears an old fit")
	require.Nil(t, p["narrative_fit"].(*float64))
	require.Equal(t, false, p["narrative_split"])
}
