package store

import (
	"testing"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

// seedGroup stores a pending TypeSafe representative with a signature and a semantic clone,
// plus an unrelated pending row, all in one run.
func seedGroup(t *testing.T, s *Store, status string) (rep, sig, sem, other *models.RunResultAnalysis) {
	t.Helper()
	runID := seedRun(t, s)
	score := 0.92
	mk := func(a *models.RunResultAnalysis) *models.RunResultAnalysis {
		a.RunResultID = addFailingResult(t, s, runID).ID
		a.Engine, a.Verdict, a.Confidence, a.ConfidenceScore = models.AnalysisEngineTypeSafe, models.VerdictFlakyTest, models.ConfidenceHigh, &score
		out, err := s.CreateAnalysis(a)
		require.NoError(t, err)
		return out
	}
	rep = mk(&models.RunResultAnalysis{NarrativeStatus: status, TypeSafeInputTokens: 500, DecisionMs: 700})
	sig = mk(&models.RunResultAnalysis{NarrativeStatus: status, SourceAnalysisID: &rep.ID,
		DedupMethod: models.DedupMethodSignature, Rationale: "[Grouped from representative analysis] "})
	sem = mk(&models.RunResultAnalysis{NarrativeStatus: status, SourceAnalysisID: &rep.ID,
		DedupMethod: models.DedupMethodSemantic, Rationale: "[Grouped semantically with representative analysis] "})
	other = mk(&models.RunResultAnalysis{NarrativeStatus: models.NarrativeStatusPending})
	return
}

func okDelta() failureanalysis.NarrationDelta {
	return failureanalysis.NarrationDelta{Summary: "S", NextAction: "N", Rationale: "R", RawResponse: `{"summary":"S"}`,
		FinishReason: "stop", NarrativeStatus: models.NarrativeStatusOK,
		PromptTokens: 300, CompletionTokens: 40, LLMMs: 900, LLMCalls: 1}
}

func TestApplyNarration_WritesTheWholeGroupInOneGo(t *testing.T) {
	s := newTestStore(t)
	rep, sig, sem, other := seedGroup(t, s, models.NarrativeStatusPending)

	changed, err := s.ApplyNarration(rep.ID, okDelta())
	require.NoError(t, err)
	require.Len(t, changed, 3)
	require.Equal(t, rep.ID, changed[0].ID, "the representative comes first")

	got, err := s.GetAnalysisByID(rep.ID)
	require.NoError(t, err)
	require.Equal(t, "S", got.Summary)
	require.Equal(t, "N", got.NextAction)
	require.Equal(t, "R", got.Rationale)
	require.Equal(t, `{"summary":"S"}`, got.RawResponse)
	require.Equal(t, "stop", got.FinishReason)
	require.Equal(t, models.NarrativeStatusOK, got.NarrativeStatus)
	require.Equal(t, 1, got.NarrativeRevision)
	require.Equal(t, 300, got.TokenUsagePrompt)
	require.Equal(t, 40, got.TokenUsageCompletion)
	require.Equal(t, 900, got.LLMMs)
	require.Equal(t, 1, got.LLMCalls)
	require.Equal(t, 500, got.TypeSafeInputTokens, "the decision's own accounting is kept")
	require.Equal(t, 700, got.DecisionMs)
	require.Equal(t, models.VerdictFlakyTest, got.Verdict, "the decision is untouched")
	require.Equal(t, rep.Version, got.Version, "an explanation is not a new version")

	for _, c := range []struct {
		row    *models.RunResultAnalysis
		prefix string
	}{{sig, "[Grouped from representative analysis] "}, {sem, "[Grouped semantically with representative analysis] "}} {
		got, err := s.GetAnalysisByID(c.row.ID)
		require.NoError(t, err)
		require.Equal(t, "S", got.Summary)
		require.Equal(t, "N", got.NextAction)
		require.Equal(t, c.prefix+"R", got.Rationale, "the clone keeps its grouping marker")
		require.Equal(t, models.NarrativeStatusOK, got.NarrativeStatus)
		require.Equal(t, 1, got.NarrativeRevision)
		require.Zero(t, got.TokenUsagePrompt+got.TokenUsageCompletion+got.LLMMs+got.LLMCalls, "usage stays on the representative")
		require.Empty(t, got.RawResponse)
		require.Empty(t, got.FinishReason)
	}

	untouched, err := s.GetAnalysisByID(other.ID)
	require.NoError(t, err)
	require.Equal(t, models.NarrativeStatusPending, untouched.NarrativeStatus)
	require.Empty(t, untouched.Summary)
}

func TestApplyNarration_LostApplyChangesNothing(t *testing.T) {
	s := newTestStore(t)
	rep, sig, _, _ := seedGroup(t, s, models.NarrativeStatusUnavailable) // e.g. swept while the call ran

	changed, err := s.ApplyNarration(rep.ID, okDelta())
	require.NoError(t, err)
	require.Empty(t, changed)
	for _, id := range []string{rep.ID, sig.ID} {
		got, err := s.GetAnalysisByID(id)
		require.NoError(t, err)
		require.Equal(t, models.NarrativeStatusUnavailable, got.NarrativeStatus)
		require.Empty(t, got.Summary)
		require.Zero(t, got.TokenUsagePrompt)
		require.Zero(t, got.NarrativeRevision)
	}
}

func TestClaimExplanation_OneNarrationInFlightPerGroup(t *testing.T) {
	s := newTestStore(t)
	rep, sig, _, _ := seedGroup(t, s, models.NarrativeStatusSkipped)

	ok, err := s.ClaimExplanation(rep.ID)
	require.NoError(t, err)
	require.True(t, ok)
	got, err := s.GetAnalysisByID(rep.ID)
	require.NoError(t, err)
	require.Equal(t, models.NarrativeStatusPending, got.NarrativeStatus)
	require.Equal(t, 1, got.NarrativeRevision)

	ok, err = s.ClaimExplanation(rep.ID)
	require.NoError(t, err)
	require.False(t, ok, "a second claim loses")

	ok, err = s.ClaimExplanation(sig.ID)
	require.NoError(t, err)
	require.False(t, ok, "a clone is explained through its representative, never claimed itself")

	_, err = s.ApplyNarration(rep.ID, okDelta())
	require.NoError(t, err)
	ok, err = s.ClaimExplanation(rep.ID)
	require.NoError(t, err)
	require.False(t, ok, "an explained group cannot be claimed again")

	runID := seedRun(t, s)
	failed, err := s.CreateAnalysis(&models.RunResultAnalysis{RunResultID: addFailingResult(t, s, runID).ID,
		Engine: models.AnalysisEngineTypeSafe, Verdict: models.VerdictUnknown, Confidence: models.ConfidenceLow,
		DecisionStatus: models.DecisionStatusFailed, NarrativeStatus: models.NarrativeStatusUnavailable})
	require.NoError(t, err)
	ok, err = s.ClaimExplanation(failed.ID)
	require.NoError(t, err)
	require.False(t, ok, "a failed attempt has no decision to explain")

	for _, status := range []string{models.NarrativeStatusUnavailable, models.NarrativeStatusUnparseable} {
		r, _, _, _ := seedGroup(t, s, status)
		ok, err := s.ClaimExplanation(r.ID)
		require.NoError(t, err)
		require.True(t, ok, status)
	}
}

func TestSweepPendingNarratives_ByJobAndAll(t *testing.T) {
	s := newTestStore(t)
	runID := seedRun(t, s)
	jobA, jobB := "job-a", "job-b"
	mk := func(jobID *string) *models.RunResultAnalysis {
		a, err := s.CreateAnalysis(&models.RunResultAnalysis{RunResultID: addFailingResult(t, s, runID).ID, JobID: jobID,
			Engine: models.AnalysisEngineTypeSafe, Verdict: models.VerdictFlakyTest, Confidence: models.ConfidenceHigh,
			NarrativeStatus: models.NarrativeStatusPending})
		require.NoError(t, err)
		return a
	}
	a1, a2, b1, lone := mk(&jobA), mk(&jobA), mk(&jobB), mk(nil)

	ids, err := s.PendingNarrativeIDs(jobA)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{a1.ID, a2.ID}, ids)

	n, err := s.SweepPendingNarratives(jobA)
	require.NoError(t, err)
	require.Equal(t, int64(2), n)
	got, err := s.GetAnalysisByID(a1.ID)
	require.NoError(t, err)
	require.Equal(t, models.NarrativeStatusUnavailable, got.NarrativeStatus)
	require.Equal(t, SweptNarrativeSummary, got.Summary)
	require.Equal(t, 1, got.NarrativeRevision)
	still, err := s.GetAnalysisByID(b1.ID)
	require.NoError(t, err)
	require.Equal(t, models.NarrativeStatusPending, still.NarrativeStatus, "another job's rows are left alone")

	n, err = s.SweepPendingNarratives("")
	require.NoError(t, err)
	require.Equal(t, int64(2), n, "startup sweep: every pending row, job or not")
	got, err = s.GetAnalysisByID(lone.ID)
	require.NoError(t, err)
	require.Equal(t, models.NarrativeStatusUnavailable, got.NarrativeStatus)
}

func TestListGroupMemberResults(t *testing.T) {
	s := newTestStore(t)
	rep, sig, sem, _ := seedGroup(t, s, models.NarrativeStatusPending)
	members, err := s.ListGroupMemberResults(rep.ID)
	require.NoError(t, err)
	var ids []string
	for _, m := range members {
		ids = append(ids, m.ID)
	}
	require.ElementsMatch(t, []string{sig.RunResultID, sem.RunResultID}, ids, "the clones' results, not the representative's")
}
