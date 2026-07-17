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

// capturingProvider records the rendered prompt from the first chat message so
// tests can assert what enrichment actually reached the model.
type capturingProvider struct{ lastPrompt string }

func (c *capturingProvider) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if len(req.Messages) > 0 {
		c.lastPrompt = req.Messages[0].Content
	}
	body, _ := json.Marshal(map[string]string{
		"verdict": "product_bug", "confidence": "medium",
		"summary": "s", "next_action": "n", "rationale": "r",
	})
	return &llm.ChatResponse{
		Content: string(body),
		Model:   "mock",
		Usage:   &llm.ChatUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	}, nil
}

// TestWorkerSendsEnrichmentToProvider is the anti-regression guard for the
// worker call site: it seeds a test case with a linked defect plus a prior
// failure in another run, runs the job, and asserts BOTH pieces of enrichment
// (the linked defect and the historical-failure block) reach the model. If the
// worker ever reverts to a bare AnalyzeContext{Result: ...}, this fails.
func TestWorkerSendsEnrichmentToProvider(t *testing.T) {
	s := newStore(t)

	// Test case with a linked defect.
	folder, err := s.CreateFolder("Login Suite", nil)
	require.NoError(t, err)
	tc := &models.TestCase{FolderID: folder.ID, Name: "Login flow"}
	require.NoError(t, s.CreateTestCase(tc))
	tcID := tc.ID

	defect := &models.Defect{Title: "Login returns 500", Status: "open", ExternalKey: "JIRA-777"}
	require.NoError(t, s.CreateDefect(defect))
	_, err = s.LinkDefectToTestCase(defect.ID, tcID)
	require.NoError(t, err)

	// A prior failure for the same test case in a DIFFERENT run, inside the
	// 30-day window (StartTime must be set explicitly — AddRunResult does not).
	priorRun := &models.TestRun{Name: "nightly"}
	require.NoError(t, s.CreateTestRun(priorRun))
	require.NoError(t, s.AddRunResult(&models.RunResult{
		TestRunID: priorRun.ID, TestCaseID: &tcID, TestNameSnapshot: "Login flow",
		AttemptNumber: 1, Status: models.StatusFail, FailureType: "assertion",
		ErrorMessage: "PRIOR_FAILURE_MARKER connection refused",
		DefectType:   "product_bug",
		StartTime:    time.Now().Add(-24 * time.Hour),
	}))

	// The current run whose failure will be analyzed.
	run := &models.TestRun{Name: "current"}
	require.NoError(t, s.CreateTestRun(run))
	require.NoError(t, s.AddRunResult(&models.RunResult{
		TestRunID: run.ID, TestCaseID: &tcID, TestNameSnapshot: "Login flow",
		AttemptNumber: 1, Status: models.StatusFail, FailureType: "assertion",
		ErrorMessage: "current failure", StartTime: time.Now(),
	}))

	// Redaction off so the historical marker passes through verbatim (redaction
	// is covered separately in the analyzer tests).
	_, err = s.UpdateFailureAnalysisSettings(&models.AIFailureAnalysisSettings{
		MaxAnalysesPerRun: 5, DedupEnabled: true, RedactionEnabled: false,
	})
	require.NoError(t, err)

	job, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)

	prov := &capturingProvider{}
	w := NewWorker(s, prov, nil, 10*time.Millisecond)
	require.NoError(t, w.processOnce(context.Background()))

	got, err := s.GetAnalysisJob(job.ID)
	require.NoError(t, err)
	require.Equal(t, models.RunAnalysisJobStatusCompleted, got.Status)

	// Enrichment must reach the model: linked defect (key + title) and the
	// historical-failure block (its message + human triage label / rollup).
	require.NotEmpty(t, prov.lastPrompt, "provider captured no prompt")
	require.Contains(t, prov.lastPrompt, "JIRA-777", "linked defect key missing from prompt")
	require.Contains(t, prov.lastPrompt, "Login returns 500", "linked defect title missing from prompt")
	require.Contains(t, prov.lastPrompt, "PRIOR_FAILURE_MARKER", "historical failure message missing from prompt")
	require.Contains(t, prov.lastPrompt, "product_bug", "human triage label / rollup missing from prompt")
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
