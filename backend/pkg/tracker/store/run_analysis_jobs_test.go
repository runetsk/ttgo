package store

import (
	"testing"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

func TestMaybeEnqueueForRunCreatesNewJob(t *testing.T) {
	s := newTestStore(t)
	runID := seedRun(t, s)

	job, created, err := s.MaybeEnqueueForRun(runID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, models.RunAnalysisJobStatusQueued, job.Status)
}

func TestMaybeEnqueueForRunIdempotent(t *testing.T) {
	s := newTestStore(t)
	runID := seedRun(t, s)

	j1, created1, err := s.MaybeEnqueueForRun(runID, models.RunAnalysisJobTriggerAutoOnDone, "")
	require.NoError(t, err)
	require.True(t, created1)

	j2, created2, err := s.MaybeEnqueueForRun(runID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	require.False(t, created2)
	require.Equal(t, j1.ID, j2.ID, "second call should return the existing active job")
}

func TestMaybeEnqueueForRunAllowsNewAfterTerminal(t *testing.T) {
	s := newTestStore(t)
	runID := seedRun(t, s)

	j1, _, err := s.MaybeEnqueueForRun(runID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	require.NoError(t, s.UpdateAnalysisJobStatus(j1.ID, models.RunAnalysisJobStatusCompleted, ""))

	j2, created, err := s.MaybeEnqueueForRun(runID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	require.True(t, created)
	require.NotEqual(t, j1.ID, j2.ID)
}

func TestSweepRunningJobsMarksOrphansFailed(t *testing.T) {
	s := newTestStore(t)
	runID := seedRun(t, s)
	j, _, err := s.MaybeEnqueueForRun(runID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	require.NoError(t, s.MarkAnalysisJobRunning(j.ID))

	affected, err := s.SweepRunningAnalysisJobs()
	require.NoError(t, err)
	require.Equal(t, int64(1), affected)

	got, err := s.GetAnalysisJob(j.ID)
	require.NoError(t, err)
	require.Equal(t, models.RunAnalysisJobStatusFailed, got.Status)
	require.Contains(t, got.ErrorMessage, "interrupted")
}
