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
	claimed, err := s.MarkAnalysisJobRunning(j1.ID)
	require.NoError(t, err)
	require.True(t, claimed)
	changed, err := s.UpdateAnalysisJobStatus(j1.ID, models.RunAnalysisJobStatusCompleted, "")
	require.NoError(t, err)
	require.True(t, changed)

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
	_, err = s.MarkAnalysisJobRunning(j.ID)
	require.NoError(t, err)

	affected, err := s.SweepRunningAnalysisJobs()
	require.NoError(t, err)
	require.Equal(t, int64(1), affected)

	got, err := s.GetAnalysisJob(j.ID)
	require.NoError(t, err)
	require.Equal(t, models.RunAnalysisJobStatusFailed, got.Status)
	require.Contains(t, got.ErrorMessage, "interrupted")
}

func TestMarkAnalysisJobRunning_DoesNotReviveCancelledJob(t *testing.T) {
	s := newTestStore(t)
	run := &models.TestRun{Name: "r"}
	require.NoError(t, s.CreateTestRun(run))
	job, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	// Cancel lands between the worker's pick-up and its claim.
	changed, err := s.UpdateAnalysisJobStatus(job.ID, models.RunAnalysisJobStatusCancelled, "")
	require.NoError(t, err)
	require.True(t, changed)
	claimed, err := s.MarkAnalysisJobRunning(job.ID)
	require.NoError(t, err)
	require.False(t, claimed, "a cancelled job must not be revived as running")
	got, _ := s.GetAnalysisJob(job.ID)
	require.Equal(t, models.RunAnalysisJobStatusCancelled, got.Status)
}

func TestUpdateAnalysisJobStatus_CompletedCannotOverwriteCancelled(t *testing.T) {
	s := newTestStore(t)
	run := &models.TestRun{Name: "r"}
	require.NoError(t, s.CreateTestRun(run))
	job, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	claimed, err := s.MarkAnalysisJobRunning(job.ID)
	require.NoError(t, err)
	require.True(t, claimed)

	changed, err := s.UpdateAnalysisJobStatus(job.ID, models.RunAnalysisJobStatusCancelled, "")
	require.NoError(t, err)
	require.True(t, changed)

	changed, err = s.UpdateAnalysisJobStatus(job.ID, models.RunAnalysisJobStatusCompleted, "")
	require.NoError(t, err)
	require.False(t, changed, "completed must not overwrite cancelled")

	got, err := s.GetAnalysisJob(job.ID)
	require.NoError(t, err)
	require.Equal(t, models.RunAnalysisJobStatusCancelled, got.Status)
}

func TestUpdateAnalysisJobStatus_CompletesRunningJob(t *testing.T) {
	s := newTestStore(t)
	run := &models.TestRun{Name: "r"}
	require.NoError(t, s.CreateTestRun(run))
	job, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	claimed, err := s.MarkAnalysisJobRunning(job.ID)
	require.NoError(t, err)
	require.True(t, claimed)
	changed, err := s.UpdateAnalysisJobStatus(job.ID, models.RunAnalysisJobStatusCompleted, "")
	require.NoError(t, err)
	require.True(t, changed)
	require.NoError(t, s.SetAnalysisJobSemanticTokens(job.ID, 1234))
	got, err := s.GetAnalysisJob(job.ID)
	require.NoError(t, err)
	require.Equal(t, 1234, got.SemanticInputTokens)
}
func TestCreateSkippedAnalysisJob_IsTerminalAndNeverBlocks(t *testing.T) {
	s := newTestStore(t)
	runID := seedRun(t, s)

	skipped, err := s.CreateSkippedAnalysisJob(runID, models.RunAnalysisJobSkipReasonBudget, 0.4, 9.8, 10)
	require.NoError(t, err)
	require.Equal(t, models.RunAnalysisJobStatusSkipped, skipped.Status)
	require.Equal(t, models.RunAnalysisJobTriggerAutoOnDone, skipped.Trigger)
	require.NotNil(t, skipped.CompletedAt, "terminal from the start")
	require.Equal(t, "budget", skipped.SkipReason)
	require.InDelta(t, 0.4, *skipped.SkipEstimateUSD, 1e-12)
	require.InDelta(t, 9.8, *skipped.SkipSpentUSD, 1e-12)
	require.InDelta(t, 10, *skipped.SkipBudgetUSD, 1e-12)

	latest, err := s.GetLatestAnalysisJobForRun(runID)
	require.NoError(t, err)
	require.Equal(t, skipped.ID, latest.ID, "the run page shows the skip")
	next, err := s.NextQueuedAnalysisJob()
	require.NoError(t, err)
	require.Nil(t, next, "the worker never picks it up")
	o, err := s.AnalysisJobOutcomes(skipped.ID)
	require.NoError(t, err)
	require.Zero(t, o.Groups)
	require.Zero(t, o.FailedRows)
	changed, err := s.UpdateAnalysisJobStatus(skipped.ID, models.RunAnalysisJobStatusCancelled, "")
	require.NoError(t, err)
	require.False(t, changed, "nothing to cancel")

	job, created, err := s.MaybeEnqueueForRun(runID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	require.True(t, created, "a skipped job is not active: Run anyway is not blocked")
	require.NotEqual(t, skipped.ID, job.ID)
}
