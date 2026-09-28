package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"
	"ttgo/pkg/tracker/typesafe"

	"github.com/stretchr/testify/require"
)

// newStore opens a temp-file database: jobs analyze groups concurrently, and with :memory:
// every pooled connection would get its own empty database.
func newStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir) // the store creates backups/ in the working directory
	s, err := store.New(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() }) // Windows cannot remove an open SQLite file
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
type capturingProvider struct {
	mu         sync.Mutex
	lastPrompt string
}

func (c *capturingProvider) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(req.Messages) > 0 {
		for _, m := range req.Messages { // the evidence travels in the user message; SYSTEM goes separately
			if m.Role == "user" {
				c.lastPrompt = m.Content
				break
			}
		}
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
	w := NewWorker(s, staticResolver(failureanalysis.JobDeps{Narrative: prov, NarrativeModel: "mock"}), nil, 10*time.Millisecond)
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

	w := NewWorker(s, staticResolver(failureanalysis.JobDeps{Narrative: &verdictProvider{verdict: "product_bug"}, NarrativeModel: "mock"}), nil, 10*time.Millisecond)
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

	_, err = s.UpdateAnalysisJobStatus(job.ID, models.RunAnalysisJobStatusCancelled, "")
	require.NoError(t, err)

	w := NewWorker(s, staticResolver(failureanalysis.JobDeps{Narrative: &verdictProvider{verdict: "flaky_test"}, NarrativeModel: "mock"}), nil, 10*time.Millisecond)
	require.NoError(t, w.processOnce(context.Background()))

	got, err := s.GetAnalysisJob(job.ID)
	require.NoError(t, err)
	require.Equal(t, models.RunAnalysisJobStatusCancelled, got.Status)
}

// staticResolver returns the same deps for every trigger.
func staticResolver(deps failureanalysis.JobDeps) failureanalysis.DepsResolver {
	return func(string) (failureanalysis.JobDeps, error) { return deps, nil }
}

type fakeTS struct {
	fn    func(req typesafe.Request) (*typesafe.Response, error)
	calls atomic.Int32
}

func (f *fakeTS) Evaluate(_ context.Context, req typesafe.Request) (*typesafe.Response, error) {
	f.calls.Add(1)
	return f.fn(req)
}
func (f *fakeTS) ListModels(context.Context) ([]typesafe.Model, error) { return nil, nil }

// tsVerdict answers the verdict/defect_type request; pair questions get p(same).
func tsVerdict(verdict, defect string, pSame float64) func(req typesafe.Request) (*typesafe.Response, error) {
	return func(req typesafe.Request) (*typesafe.Response, error) {
		resp := &typesafe.Response{Model: "jev-1.13.0", Answers: map[string]typesafe.Answer{}, Usage: typesafe.Usage{InputTokens: 500}}
		if _, ok := req.Questions["verdict"]; ok {
			vp := map[string]float64{"product_bug": 0.02, "flaky_test": 0.02, "environment": 0.02, "test_data": 0.02, "infrastructure": 0.02, "unknown": 0.02}
			vp[verdict] = 0.9
			dp := map[string]float64{"product_bug": 0.03, "automation_bug": 0.03, "system_issue": 0.03, "insufficient_evidence": 0.03}
			dp[defect] = 0.91
			resp.Answers["verdict"] = typesafe.Answer{Type: "choice", Choice: verdict, Confidence: 0.92, Probabilities: vp}
			resp.Answers["defect_type"] = typesafe.Answer{Type: "choice", Choice: defect, Confidence: 0.87, Probabilities: dp}
			return resp, nil
		}
		for id := range req.Questions {
			resp.Answers[id] = typesafe.Answer{Type: "noul", Noul: pSame}
		}
		return resp, nil
	}
}

// seedRunWithFailures creates a run with the given (failure_type, message) rows, one test case each.
func seedRunWithFailures(t *testing.T, s *store.Store, rows [][2]string) *models.TestRun {
	t.Helper()
	folder, err := s.CreateFolder("Suite", nil)
	require.NoError(t, err)
	run := &models.TestRun{Name: "nightly"}
	require.NoError(t, s.CreateTestRun(run))
	for i, r := range rows {
		tc := &models.TestCase{FolderID: folder.ID, Name: "case " + string(rune('A'+i))}
		require.NoError(t, s.CreateTestCase(tc))
		id := tc.ID
		require.NoError(t, s.AddRunResult(&models.RunResult{TestRunID: run.ID, TestCaseID: &id, TestNameSnapshot: tc.Name,
			AttemptNumber: 1, Status: models.StatusFail, FailureType: r[0], ErrorMessage: r[1], StartTime: time.Now().Add(time.Duration(i) * time.Minute)}))
	}
	return run
}

func TestWorker_TypeSafeDecidesAndClonesCarryProvenance(t *testing.T) {
	s := newStore(t)
	run := seedRunWithFailures(t, s, [][2]string{
		{"timeout", "Timeout waiting for #checkout button after 5000ms"},
		{"timeout", "Timeout waiting for #checkout button after 5000ms"}, // same signature → signature clone
		{"timeout", "Timeout waiting for #checkout button after 7000ms"}, // different signature → semantic candidate
	})
	job, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	ts := &fakeTS{fn: tsVerdict("flaky_test", "automation_bug", 0.95)}
	deps := failureanalysis.JobDeps{
		Narrative: &verdictProvider{verdict: "product_bug"}, NarrativeModel: "mock",
		Decider:  failureanalysis.NewTypeSafeDecider(ts, "jev-1.13.0"),
		Semantic: &failureanalysis.SemanticDeps{Client: ts, Model: "jev-1.13.0", Redact: true},
	}
	w := NewWorker(s, staticResolver(deps), nil, 10*time.Millisecond)
	require.NoError(t, w.processOnce(context.Background()))

	got, err := s.GetAnalysisJob(job.ID)
	require.NoError(t, err)
	require.Equal(t, models.RunAnalysisJobStatusCompleted, got.Status)
	require.Equal(t, 1, got.UniqueGroups, "semantic merge collapsed both groups")
	require.Equal(t, 500, got.SemanticInputTokens)

	analyses, err := s.GetCurrentAnalysesByRun(run.ID)
	require.NoError(t, err)
	require.Len(t, analyses, 3)
	var rep, sigClone, semClone *models.RunResultAnalysis
	for _, a := range analyses {
		switch {
		case a.DedupGroupKey == nil:
			rep = a
		case a.DedupMethod == models.DedupMethodSignature:
			sigClone = a
		case a.DedupMethod == models.DedupMethodSemantic:
			semClone = a
		}
	}
	require.NotNil(t, rep)
	require.Equal(t, models.AnalysisEngineTypeSafe, rep.Engine)
	require.Equal(t, models.VerdictFlakyTest, rep.Verdict, "TypeSafe's verdict wins over the LLM's product_bug")
	require.Equal(t, "automation_bug", rep.SuggestedDefectType)
	require.InDelta(t, 0.92, *rep.ConfidenceScore, 1e-9)
	require.Equal(t, failureanalysis.PolicyVersionNoExamples, rep.PolicyVersion)
	require.Equal(t, 500, rep.TypeSafeInputTokens)
	require.Equal(t, models.NarrativeStatusOK, rep.NarrativeStatus)

	require.NotNil(t, sigClone)
	require.Equal(t, "automation_bug", sigClone.SuggestedDefectType)
	require.Equal(t, 0, sigClone.TypeSafeInputTokens)
	require.Nil(t, sigClone.DedupPSame)
	require.Contains(t, sigClone.Rationale, "[Grouped from representative analysis]")

	require.NotNil(t, semClone)
	require.InDelta(t, 0.95, *semClone.DedupPSame, 1e-9)
	require.Equal(t, "jev-1.13.0", semClone.DedupModel)
	require.Equal(t, failureanalysis.SemanticPolicyVersion, semClone.DedupPolicyVersion)
	require.Contains(t, semClone.Rationale, "[Grouped semantically with representative analysis]")
	require.Equal(t, models.VerdictFlakyTest, semClone.Verdict)
}

func TestWorker_SemanticOnlyModeStampsProvenance(t *testing.T) {
	s := newStore(t)
	run := seedRunWithFailures(t, s, [][2]string{
		{"timeout", "Timeout waiting for #checkout button after 5000ms"},
		{"timeout", "Timeout waiting for #checkout button after 7000ms"},
	})
	_, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	ts := &fakeTS{fn: tsVerdict("flaky_test", "automation_bug", 0.95)}
	deps := failureanalysis.JobDeps{Narrative: &verdictProvider{verdict: "environment"}, NarrativeModel: "mock",
		Semantic: &failureanalysis.SemanticDeps{Client: ts, Model: "jev-1.13.0"}} // no Decider
	w := NewWorker(s, staticResolver(deps), nil, 10*time.Millisecond)
	require.NoError(t, w.processOnce(context.Background()))
	analyses, _ := s.GetCurrentAnalysesByRun(run.ID)
	for _, a := range analyses {
		require.Equal(t, models.AnalysisEngineGenerative, a.Engine)
		require.Equal(t, "system_issue", a.SuggestedDefectType, "generative rows persist the legacy mapping")
		if a.DedupGroupKey != nil {
			require.Equal(t, models.DedupMethodSemantic, a.DedupMethod)
			require.Equal(t, "jev-1.13.0", a.DedupModel)
		}
	}
}

func TestWorker_TypeSafeOutageFallsBackToGenerative(t *testing.T) {
	s := newStore(t)
	run := seedRunWithFailures(t, s, [][2]string{
		{"timeout", "Timeout waiting for #checkout button after 5000ms"},
		{"timeout", "Timeout waiting for #checkout button after 7000ms"},
	})
	job, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	ts := &fakeTS{fn: func(typesafe.Request) (*typesafe.Response, error) {
		return nil, &typesafe.Error{Status: 529, Category: typesafe.CategoryOverloaded}
	}}
	deps := failureanalysis.JobDeps{Narrative: &verdictProvider{verdict: "product_bug"}, NarrativeModel: "mock",
		Decider: failureanalysis.NewTypeSafeDecider(ts, "m"), Semantic: &failureanalysis.SemanticDeps{Client: ts, Model: "m"}}
	w := NewWorker(s, staticResolver(deps), nil, 10*time.Millisecond)
	require.NoError(t, w.processOnce(context.Background()))
	got, _ := s.GetAnalysisJob(job.ID)
	require.Equal(t, models.RunAnalysisJobStatusCompleted, got.Status)
	require.Equal(t, 2, got.UniqueGroups, "semantic pass failed → signature groups kept")
	analyses, _ := s.GetCurrentAnalysesByRun(run.ID)
	for _, a := range analyses {
		require.Equal(t, models.AnalysisEngineGenerative, a.Engine)
		require.Contains(t, a.Rationale, "[verdict engine: TypeSafe unavailable (overloaded)")
	}
}

func TestWorker_NoNarrativeProviderFailsJob(t *testing.T) {
	s := newStore(t)
	run := seedRunWithFailures(t, s, [][2]string{{"timeout", "x"}})
	job, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	w := NewWorker(s, staticResolver(failureanalysis.JobDeps{}), nil, 10*time.Millisecond)
	require.NoError(t, w.processOnce(context.Background()))
	got, _ := s.GetAnalysisJob(job.ID)
	require.Equal(t, models.RunAnalysisJobStatusFailed, got.Status)
	require.Contains(t, got.ErrorMessage, "no LLM provider configured")
}

// countingProvider wraps verdictProvider and counts narrative calls.
type countingProvider struct {
	verdictProvider
	calls atomic.Int32
}

func (c *countingProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	c.calls.Add(1)
	return c.verdictProvider.Chat(ctx, req)
}

// truncatingProvider always stops at the token limit with incomplete JSON.
type truncatingProvider struct{}

func (truncatingProvider) Chat(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{Content: `{"verdict":"environ`, FinishReason: "length", Model: "verbose",
		Usage: &llm.ChatUsage{PromptTokens: 50, CompletionTokens: failureanalysis.ReplyTokenCap}}, nil
}

func TestWorker_CutOffReplyIsStoredAsFailedCall(t *testing.T) {
	s := newStore(t)
	run := seedRunWithFailures(t, s, [][2]string{{"timeout", "Timeout waiting for #checkout button after 5000ms"}})
	_, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	w := NewWorker(s, staticResolver(failureanalysis.JobDeps{Narrative: truncatingProvider{}, NarrativeModel: "verbose"}), nil, 10*time.Millisecond)
	require.NoError(t, w.processOnce(context.Background()))
	analyses, err := s.GetCurrentAnalysesByRun(run.ID)
	require.NoError(t, err)
	require.Len(t, analyses, 1)
	for _, a := range analyses {
		require.Equal(t, models.DecisionStatusFailed, a.DecisionStatus, "a cut-off reply is a failed attempt, not a decision")
		require.Equal(t, "truncated", a.ErrorCategory)
		require.Equal(t, "verbose", a.ModelName, "the failed attempt keeps the model it tried")
		require.NotNil(t, a.JobID, "the row carries the job that produced it")
		require.True(t, strings.HasPrefix(a.Summary, "analysis failed: "), a.Summary)
		require.Contains(t, a.Summary, "cut off at the length limit")
		require.Equal(t, 100, a.TokenUsagePrompt, "the failed row keeps what both calls cost")
		require.Equal(t, 2*failureanalysis.ReplyTokenCap, a.TokenUsageCompletion)
	}
}

func TestWorker_DeciderWithoutNarratorStillRuns(t *testing.T) {
	s := newStore(t)
	run := seedRunWithFailures(t, s, [][2]string{{"timeout", "Timeout waiting for #checkout button after 5000ms"}})
	job, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	ts := &fakeTS{fn: tsVerdict("flaky_test", "automation_bug", 0.95)}
	deps := failureanalysis.JobDeps{Decider: failureanalysis.NewTypeSafeDecider(ts, "jev-1.13.0")}
	w := NewWorker(s, staticResolver(deps), nil, 10*time.Millisecond)
	require.NoError(t, w.processOnce(context.Background()))

	got, _ := s.GetAnalysisJob(job.ID)
	require.Equal(t, models.RunAnalysisJobStatusCompleted, got.Status, "a TypeSafe decider can produce analyses without any LLM provider")
	analyses, err := s.GetCurrentAnalysesByRun(run.ID)
	require.NoError(t, err)
	require.Len(t, analyses, 1)
	for _, a := range analyses {
		require.Equal(t, models.AnalysisEngineTypeSafe, a.Engine)
		require.Equal(t, models.VerdictFlakyTest, a.Verdict)
		require.Equal(t, models.NarrativeStatusUnavailable, a.NarrativeStatus)
	}
}

func TestWorker_ExplanationsOffStoreDecisionsWithoutCallingLLM(t *testing.T) {
	s := newStore(t)
	run := seedRunWithFailures(t, s, [][2]string{
		{"timeout", "Timeout waiting for #checkout button after 5000ms"},
		{"timeout", "Timeout waiting for #checkout button after 5000ms"}, // signature clone
	})
	job, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	ts := &fakeTS{fn: tsVerdict("flaky_test", "automation_bug", 0.95)}
	prov := &countingProvider{verdictProvider: verdictProvider{verdict: "product_bug"}}
	deps := failureanalysis.JobDeps{
		Narrative: prov, NarrativeModel: "mock", NarrativeSkipped: true,
		Decider: failureanalysis.NewTypeSafeDecider(ts, "jev-1.13.0"),
	}
	w := NewWorker(s, staticResolver(deps), nil, 10*time.Millisecond)
	require.NoError(t, w.processOnce(context.Background()))

	got, _ := s.GetAnalysisJob(job.ID)
	require.Equal(t, models.RunAnalysisJobStatusCompleted, got.Status)
	require.Equal(t, int32(0), prov.calls.Load(), "explanations off: the narrator is never called")
	analyses, err := s.GetCurrentAnalysesByRun(run.ID)
	require.NoError(t, err)
	require.Len(t, analyses, 2)
	for _, a := range analyses {
		require.Equal(t, models.VerdictFlakyTest, a.Verdict)
		require.Equal(t, "automation_bug", a.SuggestedDefectType)
		require.Equal(t, models.NarrativeStatusSkipped, a.NarrativeStatus)
		require.Equal(t, 0, a.TokenUsagePrompt+a.TokenUsageCompletion)
	}
}

func TestWorker_CancelDuringLastGroupStaysCancelled(t *testing.T) {
	s := newStore(t)
	run := seedRunWithFailures(t, s, [][2]string{{"timeout", "only one"}})
	job, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	// The provider cancels the job while the (last) group is in flight.
	cancelling := &cancellingProvider{s: s, jobID: job.ID}
	w := NewWorker(s, staticResolver(failureanalysis.JobDeps{Narrative: cancelling, NarrativeModel: "mock"}), nil, 10*time.Millisecond)
	require.NoError(t, w.processOnce(context.Background()))
	got, _ := s.GetAnalysisJob(job.ID)
	require.Equal(t, models.RunAnalysisJobStatusCancelled, got.Status, "completed must not overwrite cancelled")
	analyses, _ := s.GetCurrentAnalysesByRun(run.ID)
	require.Len(t, analyses, 1, "the in-flight group still persists")
}

type cancellingProvider struct {
	s     *store.Store
	jobID string
}

func (c *cancellingProvider) Chat(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	_, _ = c.s.UpdateAnalysisJobStatus(c.jobID, models.RunAnalysisJobStatusCancelled, "")
	body, _ := json.Marshal(map[string]string{"verdict": "product_bug", "confidence": "medium", "summary": "s", "next_action": "n", "rationale": "r"})
	return &llm.ChatResponse{Content: string(body), Model: "mock"}, nil
}

func TestWorker_StampsJobIDAndPipeline(t *testing.T) {
	s := newStore(t)
	run := seedRunWithFailures(t, s, [][2]string{
		{"timeout", "Timeout waiting for #checkout button after 5000ms"},
		{"timeout", "Timeout waiting for #checkout button after 5000ms"}, // signature clone
	})
	job, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	ts := &fakeTS{fn: tsVerdict("flaky_test", "automation_bug", 0.95)}
	deps := failureanalysis.JobDeps{
		Narrative: &verdictProvider{verdict: "product_bug"}, NarrativeModel: "mock",
		Decider: failureanalysis.NewTypeSafeDecider(ts, "jev-1.13.0"), DeciderModel: "jev-1.13.0",
	}
	w := NewWorker(s, staticResolver(deps), nil, 10*time.Millisecond)
	require.NoError(t, w.processOnce(context.Background()))

	got, err := s.GetAnalysisJob(job.ID)
	require.NoError(t, err)
	require.Equal(t, "TypeSafe jev-1.13.0, explained by mock", got.PipelineLabel)
	require.Contains(t, got.Pipeline, `"decider":"jev-1.13.0"`)
	require.Contains(t, got.Pipeline, `"reply_token_cap":2048`)

	analyses, err := s.GetCurrentAnalysesByRun(run.ID)
	require.NoError(t, err)
	require.Len(t, analyses, 2)
	for _, a := range analyses {
		require.NotNil(t, a.JobID)
		require.Equal(t, job.ID, *a.JobID, "representative and clone both carry the job")
	}
	o, err := s.AnalysisJobOutcomes(job.ID)
	require.NoError(t, err)
	require.Equal(t, 1, o.Groups)
	require.Equal(t, 1, o.Decided)
	require.Equal(t, 0, o.Failed)
}

func TestWorker_RetryFailedOnlyReanalyzesFailedGroups(t *testing.T) {
	s := newStore(t)
	run := seedRunWithFailures(t, s, [][2]string{{"assertion", "first failure"}, {"assertion", "second failure"}})
	failing, err := s.ListLatestFailingResults(run.ID)
	require.NoError(t, err)
	require.Len(t, failing, 2)
	failedID, okID := failing[0].ID, failing[1].ID
	_, err = s.CreateAnalysis(&models.RunResultAnalysis{RunResultID: failedID, Verdict: models.VerdictUnknown,
		Confidence: models.ConfidenceLow, DecisionStatus: models.DecisionStatusFailed, ErrorCategory: "timeout"})
	require.NoError(t, err)
	_, err = s.CreateAnalysis(&models.RunResultAnalysis{RunResultID: okID, Verdict: models.VerdictProductBug, Confidence: models.ConfidenceHigh})
	require.NoError(t, err)

	job, created, err := s.EnqueueRetryFailedForRun(run.ID, "")
	require.NoError(t, err)
	require.True(t, created)
	require.True(t, job.RetryFailedOnly)
	w := NewWorker(s, staticResolver(failureanalysis.JobDeps{Narrative: &verdictProvider{verdict: "environment"}, NarrativeModel: "mock"}), nil, 10*time.Millisecond)
	require.NoError(t, w.processOnce(context.Background()))

	retried, err := s.ListAnalysesForResult(failedID)
	require.NoError(t, err)
	require.Len(t, retried, 2, "the failed group was analyzed again")
	require.Equal(t, models.VerdictEnvironment, retried[0].Verdict)
	untouched, err := s.ListAnalysesForResult(okID)
	require.NoError(t, err)
	require.Len(t, untouched, 1, "a group with a decision is left alone")
}

// blockingProvider holds every call until its context ends.
type blockingProvider struct{ started chan struct{} }

func (b *blockingProvider) Chat(ctx context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	select {
	case b.started <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestWorker_CancelAbandonsTheRequestInFlight(t *testing.T) {
	old := cancelPoll
	cancelPoll = 20 * time.Millisecond
	defer func() { cancelPoll = old }()
	s := newStore(t)
	run := seedRunWithFailures(t, s, [][2]string{{"assertion", "slow one"}})
	job, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	prov := &blockingProvider{started: make(chan struct{}, 1)}
	w := NewWorker(s, staticResolver(failureanalysis.JobDeps{Narrative: prov, NarrativeModel: "mock"}), nil, 10*time.Millisecond)

	done := make(chan error, 1)
	go func() { done <- w.processOnce(context.Background()) }()
	<-prov.started
	_, err = s.UpdateAnalysisJobStatus(job.ID, models.RunAnalysisJobStatusCancelled, "")
	require.NoError(t, err)
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("the job kept waiting on a provider request after it was cancelled")
	}
	got, _ := s.GetAnalysisJob(job.ID)
	require.Equal(t, models.RunAnalysisJobStatusCancelled, got.Status)
	analyses, err := s.GetCurrentAnalysesByRun(run.ID)
	require.NoError(t, err)
	require.Empty(t, analyses, "an abandoned request stores nothing")
}

func TestWorker_GroupDeadlineRecordsATimeout(t *testing.T) {
	old := GroupDeadline
	GroupDeadline = 50 * time.Millisecond
	defer func() { GroupDeadline = old }()
	s := newStore(t)
	run := seedRunWithFailures(t, s, [][2]string{{"assertion", "never answers"}})
	job, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	w := NewWorker(s, staticResolver(failureanalysis.JobDeps{Narrative: &blockingProvider{started: make(chan struct{}, 1)}, NarrativeModel: "mock"}), nil, 10*time.Millisecond)
	require.NoError(t, w.processOnce(context.Background()))

	got, _ := s.GetAnalysisJob(job.ID)
	require.Equal(t, models.RunAnalysisJobStatusCompleted, got.Status, "one slow group does not sink the job")
	analyses, err := s.GetCurrentAnalysesByRun(run.ID)
	require.NoError(t, err)
	require.Len(t, analyses, 1)
	for _, a := range analyses {
		require.Equal(t, models.DecisionStatusFailed, a.DecisionStatus)
		require.Equal(t, "timeout", a.ErrorCategory)
		require.Equal(t, "mock", a.ModelName)
	}
	o, err := s.AnalysisJobOutcomes(job.ID)
	require.NoError(t, err)
	require.Equal(t, 1, o.Failed)
	require.Equal(t, 1, o.FailedRows)
}

// overlapProvider holds each call briefly and records how many were in flight at once.
type overlapProvider struct {
	mu            sync.Mutex
	inFlight, max int
	calls         int
}

func (p *overlapProvider) Chat(ctx context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	p.mu.Lock()
	p.inFlight++
	p.calls++
	if p.inFlight > p.max {
		p.max = p.inFlight
	}
	p.mu.Unlock()
	select { // long enough for the other slots to fill
	case <-time.After(150 * time.Millisecond):
	case <-ctx.Done():
	}
	p.mu.Lock()
	p.inFlight--
	p.mu.Unlock()
	return (&verdictProvider{verdict: "product_bug"}).Chat(ctx, llm.ChatRequest{})
}

func TestWorker_AnalyzesGroupsInParallel(t *testing.T) {
	for _, parallel := range []int{1, 3} {
		t.Run(fmt.Sprintf("parallel_groups=%d", parallel), func(t *testing.T) {
			s := newStore(t)
			run := seedRunWithFailures(t, s, [][2]string{
				{"assertion", "one"}, {"assertion", "two"}, {"assertion", "three"}, {"assertion", "four"}, {"assertion", "five"},
			})
			_, err := s.UpdateFailureAnalysisSettings(&models.AIFailureAnalysisSettings{
				MaxAnalysesPerRun: 10, DedupEnabled: true, RedactionEnabled: true, PromptTemplate: failureanalysis.DefaultPromptTemplate,
				ParallelGroups: parallel,
			})
			require.NoError(t, err)
			job, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
			require.NoError(t, err)
			prov := &overlapProvider{}
			w := NewWorker(s, staticResolver(failureanalysis.JobDeps{Narrative: prov, NarrativeModel: "mock"}), nil, 10*time.Millisecond)
			require.NoError(t, w.processOnce(context.Background()))

			require.Equal(t, parallel, prov.max, "at most parallel_groups calls in flight, and that many when there is work")
			require.Equal(t, 5, prov.calls)
			got, err := s.GetAnalysisJob(job.ID)
			require.NoError(t, err)
			require.Equal(t, models.RunAnalysisJobStatusCompleted, got.Status)
			require.Equal(t, 5, got.AnalyzedCount, "progress counts finished groups")
			analyses, err := s.GetCurrentAnalysesByRun(run.ID)
			require.NoError(t, err)
			require.Len(t, analyses, 5, "every group is stored by the single writer")
		})
	}
}

func TestWorker_CancelStopsStartingNewGroups(t *testing.T) {
	old := cancelPoll
	cancelPoll = 20 * time.Millisecond
	defer func() { cancelPoll = old }()
	s := newStore(t)
	run := seedRunWithFailures(t, s, [][2]string{
		{"assertion", "one"}, {"assertion", "two"}, {"assertion", "three"}, {"assertion", "four"}, {"assertion", "five"},
	})
	_, err := s.UpdateFailureAnalysisSettings(&models.AIFailureAnalysisSettings{
		MaxAnalysesPerRun: 10, DedupEnabled: true, RedactionEnabled: true, PromptTemplate: failureanalysis.DefaultPromptTemplate,
		ParallelGroups: 2,
	})
	require.NoError(t, err)
	job, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	prov := &blockingProvider{started: make(chan struct{}, 5)}
	w := NewWorker(s, staticResolver(failureanalysis.JobDeps{Narrative: prov, NarrativeModel: "mock"}), nil, 10*time.Millisecond)

	done := make(chan error, 1)
	go func() { done <- w.processOnce(context.Background()) }()
	<-prov.started
	<-prov.started // both slots busy
	_, err = s.UpdateAnalysisJobStatus(job.ID, models.RunAnalysisJobStatusCancelled, "")
	require.NoError(t, err)
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("a cancelled job kept waiting on its in-flight groups")
	}
	require.Len(t, prov.started, 0, "no group started after the cancel")
	got, _ := s.GetAnalysisJob(job.ID)
	require.Equal(t, models.RunAnalysisJobStatusCancelled, got.Status)
	analyses, err := s.GetCurrentAnalysesByRun(run.ID)
	require.NoError(t, err)
	require.Empty(t, analyses, "abandoned groups store nothing")
}

func TestParallelGroupsBounds(t *testing.T) {
	require.Equal(t, 1, parallelGroups(0), "a row from before the setting runs one at a time")
	require.Equal(t, 1, parallelGroups(-3))
	require.Equal(t, 4, parallelGroups(4))
	require.Equal(t, models.MaxParallelGroups, parallelGroups(99))
}

func TestWorker_ShutdownBeforeAnyGroupIsNotCompletion(t *testing.T) {
	s := newStore(t)
	run := seedRunWithFailures(t, s, [][2]string{{"assertion", "one"}, {"assertion", "two"}})
	job, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	prov := &countingProvider{verdictProvider: verdictProvider{verdict: "product_bug"}}
	w := NewWorker(s, staticResolver(failureanalysis.JobDeps{Narrative: prov, NarrativeModel: "mock"}), nil, 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // the server is stopping
	require.ErrorIs(t, w.processOnce(ctx), context.Canceled)
	got, _ := s.GetAnalysisJob(job.ID)
	require.Equal(t, models.RunAnalysisJobStatusRunning, got.Status, "left for the restart sweep, never marked completed")
	require.Equal(t, int32(0), prov.calls.Load())
}

// stoppingProvider stops the server during the first call and still answers it.
type stoppingProvider struct {
	verdictProvider
	stop  context.CancelFunc
	calls atomic.Int32
}

func (p *stoppingProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if p.calls.Add(1) == 1 {
		p.stop()
	}
	return p.verdictProvider.Chat(ctx, req)
}

func TestWorker_ShutdownBetweenGroupsIsNotCompletion(t *testing.T) {
	s := newStore(t)
	run := seedRunWithFailures(t, s, [][2]string{{"assertion", "one"}, {"assertion", "two"}, {"assertion", "three"}})
	_, err := s.UpdateFailureAnalysisSettings(&models.AIFailureAnalysisSettings{
		MaxAnalysesPerRun: 10, DedupEnabled: true, RedactionEnabled: true, PromptTemplate: failureanalysis.DefaultPromptTemplate,
		ParallelGroups: 1,
	})
	require.NoError(t, err)
	job, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	prov := &stoppingProvider{verdictProvider: verdictProvider{verdict: "product_bug"}, stop: stop}
	w := NewWorker(s, staticResolver(failureanalysis.JobDeps{Narrative: prov, NarrativeModel: "mock"}), nil, 10*time.Millisecond)

	require.ErrorIs(t, w.processOnce(ctx), context.Canceled)
	require.Equal(t, int32(1), prov.calls.Load(), "no group starts after the server stops")
	got, _ := s.GetAnalysisJob(job.ID)
	require.Equal(t, models.RunAnalysisJobStatusRunning, got.Status, "two groups never ran, so the job is not complete")
}

func TestWorkerSendsDefectKeyAndCategories(t *testing.T) {
	s := newStore(t)
	folder, err := s.CreateFolder("Checkout Suite", nil)
	require.NoError(t, err)
	tc := &models.TestCase{FolderID: folder.ID, Name: "Pay by card"}
	require.NoError(t, s.CreateTestCase(tc))
	tcID := tc.ID
	cat, err := s.CreateCategory("Payments", "")
	require.NoError(t, err)
	require.NoError(t, s.AssignCategoryToTest(cat.ID, tcID))

	// An earlier failure of this test, labeled and linked to its own defect.
	priorRun := &models.TestRun{Name: "nightly"}
	require.NoError(t, s.CreateTestRun(priorRun))
	prior := &models.RunResult{TestRunID: priorRun.ID, TestCaseID: &tcID, TestNameSnapshot: tc.Name,
		AttemptNumber: 1, Status: models.StatusFail, FailureType: "assertion",
		ErrorMessage: "card declined", DefectType: "product_bug", StartTime: time.Now().Add(-24 * time.Hour)}
	require.NoError(t, s.AddRunResult(prior))
	defect := &models.Defect{Title: "Card payments declined", Status: "open", ExternalKey: "PAY-42"}
	require.NoError(t, s.CreateDefect(defect))
	_, err = s.LinkDefectToResult(defect.ID, prior.ID, tcID)
	require.NoError(t, err)

	run := &models.TestRun{Name: "current"}
	require.NoError(t, s.CreateTestRun(run))
	require.NoError(t, s.AddRunResult(&models.RunResult{TestRunID: run.ID, TestCaseID: &tcID, TestNameSnapshot: tc.Name,
		AttemptNumber: 1, Status: models.StatusFail, FailureType: "assertion", ErrorMessage: "card declined again", StartTime: time.Now()}))
	_, err = s.UpdateFailureAnalysisSettings(&models.AIFailureAnalysisSettings{MaxAnalysesPerRun: 5, DedupEnabled: true, RedactionEnabled: false})
	require.NoError(t, err)
	_, _, err = s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)

	prov := &capturingProvider{}
	w := NewWorker(s, staticResolver(failureanalysis.JobDeps{Narrative: prov, NarrativeModel: "mock"}), nil, 10*time.Millisecond)
	require.NoError(t, w.processOnce(context.Background()))

	require.Contains(t, prov.lastPrompt, "Categories: Payments")
	require.Contains(t, prov.lastPrompt, "(human: product_bug → PAY-42)", "the history row names the defect linked to that result")
}

// A triaged decision on an earlier run reaches TypeSafe as an example, the decision is stamped
// v6 and the job says few-shot; with examples off the state and the stamp are v5's.
func TestWorker_FewShotExamplesReachTypeSafeAndStampThePolicy(t *testing.T) {
	for _, tc := range []struct {
		name     string
		fewShot  int
		policy   string
		label    string
		examples bool
	}{
		{"on", 4, failureanalysis.PolicyVersionWithExamples, "TypeSafe jev-1.13.0, no LLM, few-shot (4)", true},
		{"off", 0, failureanalysis.PolicyVersionNoExamples, "TypeSafe jev-1.13.0, no LLM", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newStore(t)
			run := seedRunWithFailures(t, s, [][2]string{{"timeout", "Timeout waiting for #checkout button after 5000ms"}})
			prior := &models.TestRun{Name: "earlier"}
			require.NoError(t, s.CreateTestRun(prior))
			decided := time.Now().UTC().Add(-2 * time.Hour)
			direct := false
			require.NoError(t, s.AddRunResult(&models.RunResult{TestRunID: prior.ID, TestNameSnapshot: "other", AttemptNumber: 1,
				Status: models.StatusFail, FailureType: "timeout", ErrorMessage: "EXAMPLE_MARKER spinner never went away",
				DefectType: "automation_bug", SuggestedVerdict: models.VerdictProductBug, SuggestedDefectType: "product_bug",
				SuggestedEngine: models.AnalysisEngineTypeSafe, SuggestedPolicyVersion: failureanalysis.PolicyVersionNoExamples,
				SuggestedIsClone: &direct, DecidedAt: &decided, StartTime: decided.Add(-time.Hour)}))
			job, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
			require.NoError(t, err)

			var mu sync.Mutex
			var states []string
			answer := tsVerdict("flaky_test", "automation_bug", 0.95)
			ts := &fakeTS{fn: func(req typesafe.Request) (*typesafe.Response, error) {
				if _, ok := req.Questions["verdict"]; ok {
					b, _ := json.Marshal(req.State)
					mu.Lock()
					states = append(states, string(b))
					mu.Unlock()
				}
				return answer(req)
			}}
			deps := failureanalysis.JobDeps{Decider: failureanalysis.NewTypeSafeDecider(ts, "jev-1.13.0"),
				DeciderModel: "jev-1.13.0", FewShotExamples: tc.fewShot}
			w := NewWorker(s, staticResolver(deps), nil, 10*time.Millisecond)
			require.NoError(t, w.processOnce(context.Background()))

			require.Len(t, states, 1)
			require.Equal(t, tc.examples, strings.Contains(states[0], "EXAMPLE_MARKER"), states[0])
			require.Equal(t, tc.examples, strings.Contains(states[0], `"examples"`))
			if tc.examples {
				require.Contains(t, states[0], `"corrected":true`)
			}

			got, err := s.GetAnalysisJob(job.ID)
			require.NoError(t, err)
			require.Equal(t, tc.label, got.PipelineLabel)
			require.Contains(t, got.Pipeline, `"policy":"`+tc.policy+`"`)

			analyses, err := s.GetCurrentAnalysesByRun(run.ID)
			require.NoError(t, err)
			require.Len(t, analyses, 1)
			for _, a := range analyses {
				require.Equal(t, tc.policy, a.PolicyVersion)
			}
		})
	}
}
