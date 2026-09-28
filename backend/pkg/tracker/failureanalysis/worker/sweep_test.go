package worker

import (
	"context"
	"testing"
	"time"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"

	"github.com/stretchr/testify/require"
)

func pendingRow(t *testing.T, s *store.Store, runID string, jobID *string) *models.RunResultAnalysis {
	t.Helper()
	rr := &models.RunResult{TestRunID: runID, TestNameSnapshot: "t", AttemptNumber: 1, Status: models.StatusFail, ErrorMessage: "boom"}
	require.NoError(t, s.AddRunResult(rr))
	a, err := s.CreateAnalysis(&models.RunResultAnalysis{RunResultID: rr.ID, JobID: jobID, Engine: models.AnalysisEngineTypeSafe,
		Verdict: models.VerdictFlakyTest, Confidence: models.ConfidenceHigh, NarrativeStatus: models.NarrativeStatusPending})
	require.NoError(t, err)
	return a
}

func TestWorker_SettlePendingPublishesSweptRows(t *testing.T) {
	s := newStore(t)
	run := &models.TestRun{Name: "r"}
	require.NoError(t, s.CreateTestRun(run))
	jobID, other := "job-1", "job-2"
	mine := pendingRow(t, s, run.ID, &jobID)
	theirs := pendingRow(t, s, run.ID, &other)
	bc := &recordingBC{}
	w := NewWorker(s, staticResolver(failureanalysis.JobDeps{}), bc, time.Hour)

	w.settlePending(jobID, run.ID)

	got, err := s.GetAnalysisByID(mine.ID)
	require.NoError(t, err)
	require.Equal(t, models.NarrativeStatusUnavailable, got.NarrativeStatus)
	require.Equal(t, store.SweptNarrativeSummary, got.Summary)
	still, err := s.GetAnalysisByID(theirs.ID)
	require.NoError(t, err)
	require.Equal(t, models.NarrativeStatusPending, still.NarrativeStatus)
	require.Equal(t, 1, bc.count("updated"))
}

func TestWorker_StartupSweepsEveryPendingExplanation(t *testing.T) {
	s := newStore(t)
	run := &models.TestRun{Name: "r"}
	require.NoError(t, s.CreateTestRun(run))
	jobID := "old-job"
	a := pendingRow(t, s, run.ID, &jobID)
	b := pendingRow(t, s, run.ID, nil) // an Explain claim interrupted by the restart
	w := NewWorker(s, staticResolver(failureanalysis.JobDeps{}), nil, time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	w.Run(ctx) // sweeps, then returns at once

	for _, id := range []string{a.ID, b.ID} {
		got, err := s.GetAnalysisByID(id)
		require.NoError(t, err)
		require.Equal(t, models.NarrativeStatusUnavailable, got.NarrativeStatus)
	}
}

// A job that ends with a group still waiting for its ack (its deadline passed first) leaves
// nothing pending.
func TestWorker_JobEndSweepSettlesLeftovers(t *testing.T) {
	s := newStore(t)
	run := seedRunWithFailures(t, s, [][2]string{{"timeout", "Timeout waiting for #checkout button after 5000ms"}})
	job, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	// Simulate a row left pending by a group that never narrated.
	left := pendingRow(t, s, run.ID, &job.ID)
	ts := &fakeTS{fn: tsVerdict("flaky_test", "automation_bug", 0.95)}
	deps := failureanalysis.JobDeps{Narrative: &verdictProvider{verdict: "product_bug"}, NarrativeModel: "mock",
		Decider: failureanalysis.NewTypeSafeDecider(ts, "jev-1.13.0"), DeciderModel: "jev-1.13.0"}
	w := NewWorker(s, staticResolver(deps), nil, 10*time.Millisecond)
	require.NoError(t, w.processOnce(context.Background()))

	got, err := s.GetAnalysisByID(left.ID)
	require.NoError(t, err)
	require.Equal(t, models.NarrativeStatusUnavailable, got.NarrativeStatus)
	o, err := s.AnalysisJobOutcomes(job.ID)
	require.NoError(t, err)
	require.Zero(t, o.ExplanationPending, "a completed job carries no explanations in progress")
}
