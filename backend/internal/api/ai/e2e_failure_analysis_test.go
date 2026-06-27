package ai_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/failureanalysis/worker"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"
)

// verdictProvider is a stub llm.Provider that always returns a fixed verdict.
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

func TestFailureAnalysisEndToEnd(t *testing.T) {
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}

	run := &models.TestRun{Name: "e2e"}
	if err := s.CreateTestRun(run); err != nil {
		t.Fatalf("CreateTestRun: %v", err)
	}

	// Seed 10 failures in 3 groups (sizes 5, 3, 2).
	seed := func(n int, msg string) {
		for i := 0; i < n; i++ {
			rr := &models.RunResult{
				TestRunID:        run.ID,
				TestNameSnapshot: "t",
				AttemptNumber:    1,
				Status:           models.StatusFail,
				FailureType:      "assertion",
				ErrorMessage:     msg,
			}
			if err := s.AddRunResult(rr); err != nil {
				t.Fatalf("AddRunResult: %v", err)
			}
		}
	}
	seed(5, "Expected 401 got 500")
	seed(3, "Expected 200 got 404")
	seed(2, "NPE at AuthService")

	// Enable auto mode with cap=2.
	if _, err := s.UpdateFailureAnalysisSettings(&models.AIFailureAnalysisSettings{
		EnabledOnCompletion: true,
		MaxAnalysesPerRun:   2,
		DedupEnabled:        true,
		RedactionEnabled:    true,
		PromptTemplate:      failureanalysis.DefaultPromptTemplate,
	}); err != nil {
		t.Fatalf("UpdateFailureAnalysisSettings: %v", err)
	}

	job, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
	if err != nil {
		t.Fatalf("MaybeEnqueueForRun: %v", err)
	}

	w := worker.NewWorker(s, &verdictProvider{verdict: "product_bug"}, nil, 10*time.Millisecond)
	if err := w.ProcessOnceForTest(context.Background()); err != nil {
		t.Fatalf("ProcessOnceForTest: %v", err)
	}

	got, err := s.GetAnalysisJob(job.ID)
	if err != nil {
		t.Fatalf("GetAnalysisJob: %v", err)
	}
	if got.Status != models.RunAnalysisJobStatusCompleted {
		t.Errorf("status: want Completed, got %q", got.Status)
	}
	if got.CappedAt != 2 {
		t.Errorf("CappedAt: want 2, got %d", got.CappedAt)
	}
	if got.UniqueGroups != 3 {
		t.Errorf("UniqueGroups: want 3, got %d", got.UniqueGroups)
	}
	if got.TotalFailures != 10 {
		t.Errorf("TotalFailures: want 10, got %d", got.TotalFailures)
	}

	// Two largest groups (5 + 3) analyzed → 2 reps + 6 clones = 8 analyses total.
	analyzedCount := 0
	results, _ := s.ListLatestFailingResults(run.ID)
	for _, r := range results {
		list, _ := s.ListAnalysesForResult(r.ID)
		if len(list) > 0 {
			analyzedCount++
		}
	}
	if analyzedCount != 8 {
		t.Errorf("analyzed results: want 8, got %d", analyzedCount)
	}
}
