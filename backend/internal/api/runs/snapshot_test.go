package runs

import (
	"testing"
	"time"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

func f(v float64) *float64 { return &v }

func TestSnapshotValues_TypeSafeUsesDefectTypeConfidence(t *testing.T) {
	a := &models.RunResultAnalysis{Verdict: "unknown", Confidence: "low", Engine: models.AnalysisEngineTypeSafe,
		ConfidenceScore: f(0.3), SuggestedDefectType: "automation_bug", SuggestedDefectTypeConfidence: f(0.93)}
	now := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
	v := snapshotValues(a, now)
	require.Equal(t, "unknown", v["suggested_verdict"])
	require.Equal(t, "automation_bug", v["suggested_defect_type"])
	require.Equal(t, "high", v["suggested_confidence"], "bucket of the DEFECT-TYPE confidence, not the verdict's")
	require.Equal(t, models.AnalysisEngineTypeSafe, v["suggested_engine"])
	require.InDelta(t, 0.93, *(v["suggested_confidence_score"].(*float64)), 1e-9)
	require.Equal(t, now, v["decided_at"])
}

func TestSnapshotValues_GenerativeKeepsLegacyShape(t *testing.T) {
	a := &models.RunResultAnalysis{Verdict: "flaky_test", Confidence: "medium", Engine: models.AnalysisEngineGenerative, SuggestedDefectType: "automation_bug"}
	v := snapshotValues(a, time.Now())
	require.Equal(t, "medium", v["suggested_confidence"])
	require.Equal(t, models.AnalysisEngineGenerative, v["suggested_engine"])
	require.Nil(t, v["suggested_confidence_score"])
}

func TestSnapshotValues_LegacyRowWithEmptyEngineIsGenerative(t *testing.T) {
	a := &models.RunResultAnalysis{Verdict: "product_bug", Confidence: "high", SuggestedDefectType: "product_bug"}
	v := snapshotValues(a, time.Now())
	require.Equal(t, models.AnalysisEngineGenerative, v["suggested_engine"])
}

func TestClearAISuggestion_ClearsEveryColumn(t *testing.T) {
	m := map[string]interface{}{}
	clearAISuggestion(m)
	for _, k := range []string{"suggested_verdict", "suggested_defect_type", "suggested_confidence", "suggested_engine"} {
		require.Equal(t, "", m[k], k)
	}
	require.Nil(t, m["suggested_confidence_score"])
	require.Nil(t, m["decided_at"])
}

func TestSnapshotKey_SeparatesSameVerdictDifferentSuggestion(t *testing.T) {
	a := &models.RunResultAnalysis{Verdict: "unknown", Confidence: "low", Engine: "typesafe", SuggestedDefectType: "automation_bug", SuggestedDefectTypeConfidence: f(0.9)}
	b := &models.RunResultAnalysis{Verdict: "unknown", Confidence: "low", Engine: "typesafe", SuggestedDefectType: "", SuggestedDefectTypeConfidence: f(0.4)}
	c := &models.RunResultAnalysis{Verdict: "unknown", Confidence: "low", Engine: "generative"}
	require.NotEqual(t, snapshotKeyFor(a), snapshotKeyFor(b))
	require.NotEqual(t, snapshotKeyFor(a), snapshotKeyFor(c))
	require.Equal(t, snapshotKeyFor(a), snapshotKeyFor(&models.RunResultAnalysis{Verdict: "unknown", Confidence: "low", Engine: "typesafe", SuggestedDefectType: "automation_bug", SuggestedDefectTypeConfidence: f(0.9)}))
}

func TestSnapshotValues_DerivedSuggestionIsItsOwnEngine(t *testing.T) {
	a := &models.RunResultAnalysis{Verdict: "product_bug", Confidence: "high", Engine: models.AnalysisEngineTypeSafe,
		ConfidenceScore: f(0.96), SuggestedDefectType: "product_bug", SuggestedDefectTypeConfidence: f(0.96),
		SuggestionSource: models.SuggestionSourceVerdict}
	v := snapshotValues(a, time.Date(2026, 9, 22, 9, 0, 0, 0, time.UTC))
	require.Equal(t, models.AnalysisEngineTypeSafeDerived, v["suggested_engine"], "graded apart from question-answered suggestions")
	require.Equal(t, "product_bug", v["suggested_defect_type"])
	require.Equal(t, "high", v["suggested_confidence"])
	require.InDelta(t, 0.96, *(v["suggested_confidence_score"].(*float64)), 1e-9)
}
