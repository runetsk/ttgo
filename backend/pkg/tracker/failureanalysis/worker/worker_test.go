package worker

import (
	"context"
	"encoding/json"
	"testing"
	"time"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"

	"github.com/stretchr/testify/require"
)

func newStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(":memory:")
	require.NoError(t, err)
	return s
}

type verdictProvider struct{ verdict string }

func (p *verdictProvider) Chat(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	body, _ := json.Marshal(map[string]string{
		"verdict":     p.verdict,
		"confidence":  "medium",
		"summary":     "s",
		"next_action": "n",
		"rationale":   "r",
	})
	return &llm.ChatResponse{
		Content: string(body),
		Model:   "mock",
		Usage:   &llm.ChatUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	}, nil
}

func TestWorkerHappyPathWithCap(t *testing.T) {
	s := newStore(t)
	run := &models.TestRun{Name: "r"}
	require.NoError(t, s.CreateTestRun(run))

	mk := func(err string) {
		rr := &models.RunResult{
			TestRunID: run.ID, TestNameSnapshot: "t",
			AttemptNumber: 1, Status: models.StatusFail,
			FailureType: "assertion", ErrorMessage: err,
		}
		require.NoError(t, s.AddRunResult(rr))
	}
	mk("Expected 401 got 500")
	mk("Expected 401 got 500")
	mk("NullPointerException at AuthService")

	_, err := s.UpdateFailureAnalysisSettings(&models.AIFailureAnalysisSettings{
		MaxAnalysesPerRun: 1, DedupEnabled: true, RedactionEnabled: true,
	})
	require.NoError(t, err)

	job, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)

	w := NewWorker(s, &verdictProvider{verdict: "product_bug"}, nil, 10*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, w.processOnce(ctx))

	got, err := s.GetAnalysisJob(job.ID)
	require.NoError(t, err)
	require.Equal(t, models.RunAnalysisJobStatusCompleted, got.Status)
	require.Equal(t, 1, got.CappedAt)
	require.Equal(t, 2, got.UniqueGroups)
	require.Equal(t, 3, got.TotalFailures)

	analyses, err := s.ListAnalysesForResult(firstFailingResultID(t, s, run.ID, 0))
	require.NoError(t, err)
	require.Len(t, analyses, 1)
}

func firstFailingResultID(t *testing.T, s *store.Store, runID string, idx int) string {
	t.Helper()
	rows, err := s.ListLatestFailingResults(runID)
	require.NoError(t, err)
	require.Greater(t, len(rows), idx)
	return rows[idx].ID
}

func TestWorkerCancellationStopsAfterCurrentGroup(t *testing.T) {
	s := newStore(t)
	run := &models.TestRun{Name: "r"}
	require.NoError(t, s.CreateTestRun(run))

	for i := 0; i < 3; i++ {
		rr := &models.RunResult{
			TestRunID: run.ID, TestNameSnapshot: "t",
			AttemptNumber: 1, Status: models.StatusFail,
			FailureType:  "assertion",
			ErrorMessage: []string{"a", "b", "c"}[i],
		}
		require.NoError(t, s.AddRunResult(rr))
	}
	_, err := s.UpdateFailureAnalysisSettings(&models.AIFailureAnalysisSettings{
		MaxAnalysesPerRun: 10, DedupEnabled: true, RedactionEnabled: true,
	})
	require.NoError(t, err)

	job, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)

	require.NoError(t, s.UpdateAnalysisJobStatus(job.ID, models.RunAnalysisJobStatusCancelled, ""))

	w := NewWorker(s, &verdictProvider{verdict: "flaky_test"}, nil, 10*time.Millisecond)
	require.NoError(t, w.processOnce(context.Background()))

	got, err := s.GetAnalysisJob(job.ID)
	require.NoError(t, err)
	require.Equal(t, models.RunAnalysisJobStatusCancelled, got.Status)
}
