package store

import (
	"testing"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

func TestAnalysisJobOutcomes_TransferCounts(t *testing.T) {
	s := newTestStore(t)
	runID := seedRun(t, s)
	job, _, err := s.MaybeEnqueueForRun(runID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	jobID := job.ID
	create := func(a *models.RunResultAnalysis) *models.RunResultAnalysis {
		a.RunResultID = addFailingResult(t, s, runID).ID
		a.JobID = &jobID
		a.Engine, a.Verdict, a.Confidence = models.AnalysisEngineTypeSafe, models.VerdictFlakyTest, models.ConfidenceHigh
		out, err := s.CreateAnalysis(a)
		require.NoError(t, err)
		return out
	}
	low, high, edge := 0.2, 0.7, 0.5
	rep := create(&models.RunResultAnalysis{})
	create(&models.RunResultAnalysis{SourceAnalysisID: &rep.ID, DedupMethod: models.DedupMethodSemantic, NarrativeFit: &low})
	create(&models.RunResultAnalysis{SourceAnalysisID: &rep.ID, DedupMethod: models.DedupMethodSemantic, NarrativeFit: &high})
	create(&models.RunResultAnalysis{SourceAnalysisID: &rep.ID, DedupMethod: models.DedupMethodSemantic, NarrativeFit: &edge})
	create(&models.RunResultAnalysis{SourceAnalysisID: &rep.ID, DedupMethod: models.DedupMethodSignature})

	require.NoError(t, s.AddAnalysisJobTransferStats(job.ID, 1, 3))
	require.NoError(t, s.AddAnalysisJobTransferStats(job.ID, 1, 2))
	require.NoError(t, s.AddAnalysisJobTransferStats(job.ID, 0, 0))

	stored, err := s.GetAnalysisJob(job.ID)
	require.NoError(t, err)
	require.Equal(t, 2, stored.TransferCheckFailed)
	require.Equal(t, 5, stored.TransferUnchecked)

	o, err := s.AnalysisJobOutcomes(job.ID)
	require.NoError(t, err)
	require.Equal(t, 1, o.TransferMismatch, "only a fit below 0.50 is a mismatch")
	require.Equal(t, 2, o.TransferCheckFailed)
	require.Equal(t, 5, o.TransferUnchecked)
	require.Equal(t, 1, o.Groups, "clones are not groups")

	got, err := s.GetAnalysisByID(rep.ID)
	require.NoError(t, err)
	require.Nil(t, got.NarrativeFit, "a representative has no fit")
	require.False(t, got.NarrativeSplit)
}
