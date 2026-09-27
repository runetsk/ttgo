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
	for _, k := range []string{"suggested_verdict", "suggested_defect_type", "suggested_confidence", "suggested_engine", "suggested_policy_version"} {
		require.Equal(t, "", m[k], k)
	}
	require.Nil(t, m["suggested_confidence_score"])
	require.Contains(t, m, "suggested_is_clone", "the clone flag must be written, not left over from an earlier decision")
	require.Nil(t, m["suggested_is_clone"], "cleared to unknown (NULL), never to false, which would read as a direct prediction")
	require.Nil(t, m["decided_at"])
}

func TestSnapshotValues_RecordsPolicyAndCloneProvenance(t *testing.T) {
	src := "rep-1"
	direct := &models.RunResultAnalysis{Verdict: "flaky_test", Confidence: "high", Engine: models.AnalysisEngineTypeSafe,
		SuggestedDefectType: "automation_bug", SuggestedDefectTypeConfidence: f(0.91), PolicyVersion: "fa-verdict-v5"}
	clone := *direct
	clone.SourceAnalysisID = &src

	v := snapshotValues(direct, time.Now())
	require.Equal(t, "fa-verdict-v5", v["suggested_policy_version"])
	require.Equal(t, false, v["suggested_is_clone"])

	v = snapshotValues(&clone, time.Now())
	require.Equal(t, "fa-verdict-v5", v["suggested_policy_version"])
	require.Equal(t, true, v["suggested_is_clone"], "a clone carries its representative's answer, not a prediction of its own")

	gen := snapshotValues(&models.RunResultAnalysis{Verdict: "product_bug", Confidence: "high", SuggestedDefectType: "product_bug"}, time.Now())
	require.Equal(t, "", gen["suggested_policy_version"], "generative rows have no question-set policy")
	require.Equal(t, false, gen["suggested_is_clone"])
}

func TestSnapshotKey_SeparatesDirectFromCloneAndPolicies(t *testing.T) {
	src := "rep-1"
	base := models.RunResultAnalysis{Verdict: "flaky_test", Confidence: "high", Engine: "typesafe",
		SuggestedDefectType: "automation_bug", SuggestedDefectTypeConfidence: f(0.9), PolicyVersion: "fa-verdict-v5"}
	clone := base
	clone.SourceAnalysisID = &src
	older := base
	older.PolicyVersion = "fa-verdict-v4"
	require.NotEqual(t, snapshotKeyFor(&base), snapshotKeyFor(&clone), "the bulk path must not write a clone and a direct row in one statement")
	require.NotEqual(t, snapshotKeyFor(&base), snapshotKeyFor(&older))
	same := base
	require.Equal(t, snapshotKeyFor(&base), snapshotKeyFor(&same))
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
