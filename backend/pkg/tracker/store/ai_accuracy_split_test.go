package store

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

// seedTriaged adds one triaged calibration row. clone nil = provenance unknown.
func seedTriaged(t *testing.T, s *Store, runID string, n int, engine, policy string, clone *bool, suggested, human string) {
	t.Helper()
	decided := time.Now().UTC().Add(-time.Hour)
	require.NoError(t, s.AddRunResult(&models.RunResult{
		TestRunID: runID, TestNameSnapshot: fmt.Sprintf("p%d", n), AttemptNumber: n,
		Status: models.StatusFail, ErrorMessage: "boom", DefectType: human,
		SuggestedVerdict: models.VerdictProductBug, SuggestedDefectType: suggested,
		SuggestedConfidence: models.ConfidenceHigh, SuggestedEngine: engine,
		SuggestedPolicyVersion: policy, SuggestedIsClone: clone, DecidedAt: &decided,
	}))
}

func engineBucket(t *testing.T, rep *AIFailureAnalysisAccuracy, engine string) AIAccuracyEngineBucket {
	t.Helper()
	for _, b := range rep.ByEngine {
		if b.Engine == engine {
			return b
		}
	}
	t.Fatalf("no %s bucket in %+v", engine, rep.ByEngine)
	return AIAccuracyEngineBucket{}
}

func TestAccuracy_SplitsDirectPredictionsFromClones(t *testing.T) {
	s := newTestStore(t)
	runID := seedRun(t, s)
	direct, clone := tsBool(false), tsBool(true)
	seedTriaged(t, s, runID, 1, "typesafe", "fa-verdict-v5", direct, "product_bug", "product_bug")
	seedTriaged(t, s, runID, 2, "typesafe", "fa-verdict-v5", direct, "product_bug", "automation_bug")
	seedTriaged(t, s, runID, 3, "typesafe", "fa-verdict-v5", clone, "product_bug", "product_bug")
	seedTriaged(t, s, runID, 4, "typesafe", "fa-verdict-v5", clone, "product_bug", "product_bug")
	seedTriaged(t, s, runID, 5, "generative", "", direct, "product_bug", "product_bug")
	seedTriaged(t, s, runID, 6, "generative", "", nil, "product_bug", "system_issue") // provenance unknown

	rep, err := s.GetFailureAnalysisAccuracy(last30d())
	require.NoError(t, err)
	require.Equal(t, 6, rep.Total, "unknown rows stay in the overall totals")
	require.Equal(t, 4, rep.Agreed)
	require.Equal(t, 3, rep.DirectTotal)
	require.Equal(t, 2, rep.DirectAgreed)
	require.InDelta(t, 2.0/3, rep.DirectRate, 1e-9)
	require.Equal(t, 2, rep.CloneTotal)
	require.Equal(t, 2, rep.CloneAgreed)
	require.Equal(t, 1, rep.UnknownProvenance)

	ts := engineBucket(t, rep, "typesafe")
	require.Equal(t, 4, ts.Total)
	require.Equal(t, 2, ts.DirectTotal)
	require.Equal(t, 1, ts.DirectAgreed)
	require.InDelta(t, 0.5, ts.DirectRate, 1e-9)
	require.Equal(t, 2, ts.CloneTotal)
	require.Equal(t, 2, ts.CloneAgreed)
	gen := engineBucket(t, rep, "generative")
	require.Equal(t, 2, gen.Total)
	require.Equal(t, 1, gen.DirectTotal)
	require.Equal(t, 0, gen.CloneTotal)

	raw, err := json.Marshal(rep)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"direct_rate":`, "the split is flattened into the JSON object")
	require.Contains(t, string(raw), `"unknown_provenance":1`)
}

func TestAccuracy_PolicyVersionFilter(t *testing.T) {
	s := newTestStore(t)
	runID := seedRun(t, s)
	direct := tsBool(false)
	seedTriaged(t, s, runID, 1, "typesafe", "fa-verdict-v4", direct, "product_bug", "product_bug")
	seedTriaged(t, s, runID, 2, "typesafe", "fa-verdict-v5", direct, "product_bug", "automation_bug")
	seedTriaged(t, s, runID, 3, "typesafe-derived", "fa-verdict-v5", direct, "product_bug", "product_bug")
	seedTriaged(t, s, runID, 4, "generative", "", nil, "product_bug", "product_bug")

	all, err := s.GetFailureAnalysisAccuracyFor(AccuracyFilter{Since: last30d()})
	require.NoError(t, err)
	require.Equal(t, 4, all.Total)
	require.Equal(t, "", all.PolicyVersion)
	require.Equal(t, []string{"fa-verdict-v5", "fa-verdict-v4"}, all.PolicyVersions)

	v5, err := s.GetFailureAnalysisAccuracyFor(AccuracyFilter{Since: last30d(), PolicyVersion: "fa-verdict-v5"})
	require.NoError(t, err)
	require.Equal(t, 2, v5.Total)
	require.Equal(t, 1, v5.Agreed)
	require.Equal(t, 0, v5.UnknownProvenance, "unknown rows never match a specific version")
	require.Len(t, v5.ByEngine, 2)
	require.Equal(t, "fa-verdict-v5", v5.PolicyVersion)
	require.Equal(t, []string{"fa-verdict-v5", "fa-verdict-v4"}, v5.PolicyVersions, "the dropdown keeps every version while one is selected")

	none, err := s.GetFailureAnalysisAccuracyFor(AccuracyFilter{Since: last30d(), PolicyVersion: "fa-verdict-v9"})
	require.NoError(t, err)
	require.Equal(t, 0, none.Total)
	require.Empty(t, none.ByEngine)
	require.NotNil(t, none.Coverage)
	require.Len(t, none.PolicyVersions, 2)
}

func TestAccuracy_CoveragePerEngineOverRepresentatives(t *testing.T) {
	s := newTestStore(t)
	add := func(a models.RunResultAnalysis) *models.RunResultAnalysis {
		t.Helper()
		a.RunResultID = seedFailingResult(t, s).ID
		row, err := s.CreateAnalysis(&a)
		require.NoError(t, err)
		return row
	}
	ts := func(policy, verdict, suggested, narrative string) models.RunResultAnalysis {
		return models.RunResultAnalysis{Engine: models.AnalysisEngineTypeSafe, ModelName: "jev", PolicyVersion: policy,
			Verdict: verdict, Confidence: models.ConfidenceHigh, SuggestedDefectType: suggested, NarrativeStatus: narrative}
	}
	repRow := add(ts("fa-verdict-v5", "product_bug", "product_bug", models.NarrativeStatusOK))
	add(ts("fa-verdict-v5", models.VerdictUnknown, "", models.NarrativeStatusOK))           // abstained
	add(ts("fa-verdict-v5", "flaky_test", "automation_bug", models.NarrativeStatusSkipped)) // decided, no explanation
	add(ts("fa-verdict-v4", "environment", "system_issue", models.NarrativeStatusOK))
	clone := ts("fa-verdict-v5", "product_bug", "product_bug", models.NarrativeStatusOK)
	clone.SourceAnalysisID = &repRow.ID
	add(clone) // a clone repeats its representative; it is not an analysis of its own
	add(models.RunResultAnalysis{Engine: models.AnalysisEngineGenerative, Verdict: models.VerdictUnknown, Confidence: models.ConfidenceLow,
		DecisionStatus: models.DecisionStatusFailed, ErrorCategory: "timeout"})
	add(models.RunResultAnalysis{Engine: models.AnalysisEngineGenerative, ModelName: "minimax", Verdict: "product_bug", Confidence: models.ConfidenceMedium})
	old := add(ts("fa-verdict-v3", "product_bug", "product_bug", models.NarrativeStatusOK))
	require.NoError(t, s.db.Exec(`UPDATE run_result_analyses SET created_at = ? WHERE id = ?`, time.Now().AddDate(0, 0, -40), old.ID).Error)

	all, err := s.GetFailureAnalysisAccuracyFor(AccuracyFilter{Since: last30d()})
	require.NoError(t, err)
	require.Equal(t, []AIAccuracyCoverage{
		{Engine: "generative", Analyses: 2, Decided: 1, Abstained: 0, Failed: 1, NoExplanation: 0},
		{Engine: "typesafe", Analyses: 4, Decided: 3, Abstained: 1, Failed: 0, NoExplanation: 1},
	}, all.Coverage)
	require.Equal(t, []string{"fa-verdict-v5", "fa-verdict-v4"}, all.PolicyVersions, "the 40-day-old v3 analysis is outside the window")

	v5, err := s.GetFailureAnalysisAccuracyFor(AccuracyFilter{Since: last30d(), PolicyVersion: "fa-verdict-v5"})
	require.NoError(t, err)
	require.Equal(t, []AIAccuracyCoverage{{Engine: "typesafe", Analyses: 3, Decided: 2, Abstained: 1, NoExplanation: 1}}, v5.Coverage)
}
