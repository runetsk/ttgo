package store

import (
	"testing"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

// seedFitGroup stores an explained group — representative, signature clone, a mismatched and a
// fitting semantic clone — under one job.
func seedFitGroup(t *testing.T, s *Store) (jobID string, rep, sig, low, high *models.RunResultAnalysis) {
	t.Helper()
	runID := seedRun(t, s)
	job, _, err := s.MaybeEnqueueForRun(runID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	jobID = job.ID
	score := 0.93
	mk := func(a *models.RunResultAnalysis) *models.RunResultAnalysis {
		a.RunResultID = addFailingResult(t, s, runID).ID
		a.JobID = &jobID
		a.Engine, a.Verdict, a.Confidence, a.ConfidenceScore = models.AnalysisEngineTypeSafe, models.VerdictFlakyTest, models.ConfidenceHigh, &score
		a.NarrativeStatus, a.Summary = models.NarrativeStatusOK, "Shared"
		out, err := s.CreateAnalysis(a)
		require.NoError(t, err)
		return out
	}
	lowFit, highFit := 0.2, 0.9
	rep = mk(&models.RunResultAnalysis{Rationale: "R0"})
	sig = mk(&models.RunResultAnalysis{SourceAnalysisID: &rep.ID, DedupMethod: models.DedupMethodSignature,
		Rationale: "[Grouped from representative analysis] R0"})
	low = mk(&models.RunResultAnalysis{SourceAnalysisID: &rep.ID, DedupMethod: models.DedupMethodSemantic, NarrativeFit: &lowFit,
		Rationale: "[Grouped semantically with representative analysis] R0"})
	high = mk(&models.RunResultAnalysis{SourceAnalysisID: &rep.ID, DedupMethod: models.DedupMethodSemantic, NarrativeFit: &highFit,
		Rationale: "[Grouped semantically with representative analysis] R0"})
	return
}

func TestApplyNarration_WritesFitsOnSemanticClonesOnly(t *testing.T) {
	s := newTestStore(t)
	rep, sig, sem, _ := seedGroup(t, s, models.NarrativeStatusPending)
	d := okDelta()
	d.Fits = map[string]float64{sem.RunResultID: 0.3, sig.RunResultID: 0.9, rep.RunResultID: 0.9}

	changed, err := s.ApplyNarration(rep.ID, d)
	require.NoError(t, err)
	require.Len(t, changed, 3)
	for _, a := range changed {
		switch a.ID {
		case sem.ID:
			require.NotNil(t, a.NarrativeFit)
			require.InDelta(t, 0.3, *a.NarrativeFit, 1e-12)
		default:
			require.Nil(t, a.NarrativeFit, "representatives and signature clones keep NULL: %s", a.ID)
		}
	}
}

func TestApplyNarration_ANewExplanationClearsOldFits(t *testing.T) {
	s := newTestStore(t)
	rep, _, sem, _ := seedGroup(t, s, models.NarrativeStatusPending)
	d := okDelta()
	d.Fits = map[string]float64{sem.RunResultID: 0.3}
	_, err := s.ApplyNarration(rep.ID, d)
	require.NoError(t, err)

	// The explanation is later lost and explained again without a check.
	require.NoError(t, s.db.Model(&models.RunResultAnalysis{}).Where("id = ?", rep.ID).
		Update("narrative_status", models.NarrativeStatusUnavailable).Error)
	claimed, err := s.ClaimExplanation(rep.ID)
	require.NoError(t, err)
	require.True(t, claimed)
	_, err = s.ApplyNarration(rep.ID, okDelta())
	require.NoError(t, err)

	got, err := s.GetAnalysisByID(sem.ID)
	require.NoError(t, err)
	require.Nil(t, got.NarrativeFit, "a fit describes the text it was checked against")
}

func TestApplyNarration_SkipsSplitClones(t *testing.T) {
	s := newTestStore(t)
	rep, sig, sem, _ := seedGroup(t, s, models.NarrativeStatusPending)
	require.NoError(t, s.db.Model(&models.RunResultAnalysis{}).Where("id = ?", sem.ID).
		Updates(map[string]interface{}{"narrative_split": true, "summary": "Own"}).Error)

	changed, err := s.ApplyNarration(rep.ID, okDelta())
	require.NoError(t, err)
	ids := []string{}
	for _, a := range changed {
		ids = append(ids, a.ID)
	}
	require.ElementsMatch(t, []string{rep.ID, sig.ID}, ids, "a split clone is neither written nor republished")
	got, err := s.GetAnalysisByID(sem.ID)
	require.NoError(t, err)
	require.Equal(t, "Own", got.Summary)
	require.Zero(t, got.NarrativeRevision)
}

func TestClaimOwnExplanation_OnlyAMismatchedCloneOnce(t *testing.T) {
	s := newTestStore(t)
	_, rep, sig, low, high := seedFitGroup(t, s)
	for _, a := range []*models.RunResultAnalysis{rep, sig, high} {
		ok, err := s.ClaimOwnExplanation(a.ID)
		require.NoError(t, err)
		require.False(t, ok, "not a mismatched clone: %s", a.ID)
	}
	ok, err := s.ClaimOwnExplanation(low.ID)
	require.NoError(t, err)
	require.True(t, ok)
	got, err := s.GetAnalysisByID(low.ID)
	require.NoError(t, err)
	require.Equal(t, models.NarrativeStatusPending, got.NarrativeStatus)
	require.True(t, got.NarrativeSplit, "split from the claim on, so a group apply cannot overwrite it")
	require.Equal(t, 1, got.NarrativeRevision)
	require.InDelta(t, 0.2, *got.NarrativeFit, 1e-12)

	ok, err = s.ClaimOwnExplanation(low.ID)
	require.NoError(t, err)
	require.False(t, ok, "already being explained")
}

func TestClaimOwnExplanation_RetriesOnlyAFailedOwnExplanation(t *testing.T) {
	s := newTestStore(t)
	_, _, _, low, _ := seedFitGroup(t, s)
	ok, err := s.ClaimOwnExplanation(low.ID)
	require.NoError(t, err)
	require.True(t, ok)
	failed := okDelta()
	failed.NarrativeStatus, failed.Summary = models.NarrativeStatusUnavailable, "AI narrative unavailable: timeout"
	got, err := s.ApplyOwnNarration(low.ID, failed)
	require.NoError(t, err)
	require.Equal(t, models.NarrativeStatusUnavailable, got.NarrativeStatus)
	require.True(t, got.NarrativeSplit)

	ok, err = s.ClaimOwnExplanation(low.ID)
	require.NoError(t, err)
	require.True(t, ok, "a failed own explanation can be retried (R9)")
	got, err = s.ApplyOwnNarration(low.ID, okDelta())
	require.NoError(t, err)
	require.Equal(t, models.NarrativeStatusOK, got.NarrativeStatus)

	ok, err = s.ClaimOwnExplanation(low.ID)
	require.NoError(t, err)
	require.False(t, ok, "a written own explanation is not claimed again")

	require.NoError(t, s.db.Model(&models.RunResultAnalysis{}).Where("id = ?", low.ID).
		Update("narrative_status", models.NarrativeStatusUnparseable).Error)
	ok, err = s.ClaimOwnExplanation(low.ID)
	require.NoError(t, err)
	require.True(t, ok, "unparseable counts as failed")
}

func TestApplyOwnNarration_WritesThatRowOnly(t *testing.T) {
	s := newTestStore(t)
	_, rep, _, low, high := seedFitGroup(t, s)
	ok, err := s.ClaimOwnExplanation(low.ID)
	require.NoError(t, err)
	require.True(t, ok)

	got, err := s.ApplyOwnNarration(low.ID, okDelta())
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "S", got.Summary)
	require.Equal(t, "N", got.NextAction)
	require.Equal(t, "[Grouped semantically with representative analysis] R", got.Rationale, "the decision is still the group's")
	require.Equal(t, models.NarrativeStatusOK, got.NarrativeStatus)
	require.Equal(t, 2, got.NarrativeRevision, "claimed, then applied")
	require.Equal(t, 300, got.TokenUsagePrompt, "its own call is billed to it")
	require.Equal(t, 1, got.LLMCalls)
	require.Equal(t, `{"summary":"S"}`, got.RawResponse)
	require.True(t, got.NarrativeSplit)
	require.InDelta(t, 0.2, *got.NarrativeFit, 1e-12, "the fit is unchanged")

	for _, a := range []*models.RunResultAnalysis{rep, high} {
		other, err := s.GetAnalysisByID(a.ID)
		require.NoError(t, err)
		require.Equal(t, "Shared", other.Summary)
		require.Zero(t, other.NarrativeRevision)
	}

	lost, err := s.ApplyOwnNarration(low.ID, okDelta())
	require.NoError(t, err)
	require.Nil(t, lost, "no longer pending: a lost apply changes nothing")
}

func TestJobSweep_LeavesAnOwnExplanationInFlight(t *testing.T) {
	s := newTestStore(t)
	jobID, _, _, low, _ := seedFitGroup(t, s)
	ok, err := s.ClaimOwnExplanation(low.ID)
	require.NoError(t, err)
	require.True(t, ok)

	ids, err := s.PendingNarrativeIDs(jobID)
	require.NoError(t, err)
	require.NotContains(t, ids, low.ID, "the Explain request settles its own claim")
	n, err := s.SweepPendingNarratives(jobID)
	require.NoError(t, err)
	require.Zero(t, n)

	n, err = s.SweepPendingNarratives("")
	require.NoError(t, err)
	require.Equal(t, int64(1), n, "the startup sweep still settles it")
}

func TestListSemanticCloneResults(t *testing.T) {
	s := newTestStore(t)
	_, rep, _, low, high := seedFitGroup(t, s)
	got, err := s.ListSemanticCloneResults(rep.ID)
	require.NoError(t, err)
	ids := []string{}
	for _, r := range got {
		ids = append(ids, r.ID)
	}
	require.ElementsMatch(t, []string{low.RunResultID, high.RunResultID}, ids, "semantic clones only")

	ok, err := s.ClaimOwnExplanation(low.ID)
	require.NoError(t, err)
	require.True(t, ok)
	got, err = s.ListSemanticCloneResults(rep.ID)
	require.NoError(t, err)
	require.Len(t, got, 1, "a split clone no longer follows the group")
	require.Equal(t, high.RunResultID, got[0].ID)
}
