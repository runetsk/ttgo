package store

import (
	"testing"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

func TestAnalysisJobOutcomes_CopiesAutoApply(t *testing.T) {
	s := newTestStore(t)
	runID := seedRun(t, s)
	job, _, err := s.MaybeEnqueueForRun(runID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)

	o, err := s.AnalysisJobOutcomes(job.ID)
	require.NoError(t, err)
	require.Equal(t, "", o.AutoApplyState, "a queued job has not resolved it yet")
	require.Zero(t, o.AutoApplied)

	require.NoError(t, s.SetAnalysisJobAutoApplyState(job.ID, models.AutoApplyStateOn))
	require.NoError(t, s.AddAnalysisJobAutoApplied(job.ID, 3))
	require.NoError(t, s.AddAnalysisJobAutoApplied(job.ID, 2))
	require.NoError(t, s.AddAnalysisJobAutoApplied(job.ID, 0))

	o, err = s.AnalysisJobOutcomes(job.ID)
	require.NoError(t, err)
	require.Equal(t, 5, o.AutoApplied)
	require.Equal(t, models.AutoApplyStateOn, o.AutoApplyState)
	got, err := s.GetAnalysisJob(job.ID)
	require.NoError(t, err)
	require.Equal(t, 5, got.AutoApplied)
	require.Equal(t, models.AutoApplyStateOn, got.AutoApplyState)
}
