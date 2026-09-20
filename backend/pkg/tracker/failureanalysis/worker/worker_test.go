package worker

import (
	"context"
	"encoding/json"
	"testing"
	"time"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"
	"ttgo/pkg/tracker/typesafe"

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
	calls int
}

func (f *fakeTS) Evaluate(_ context.Context, req typesafe.Request) (*typesafe.Response, error) {
	f.calls++
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
	require.Equal(t, failureanalysis.PolicyVersion, rep.PolicyVersion)
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
