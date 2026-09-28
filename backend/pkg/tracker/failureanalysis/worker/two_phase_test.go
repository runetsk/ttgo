package worker

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/typesafe"

	"github.com/stretchr/testify/require"
)

// recordingBC keeps every broadcast in order.
type recordingBC struct {
	mu     sync.Mutex
	events []bcEvent
}

type bcEvent struct {
	kind     string
	a        models.RunResultAnalysis
	analyzed int
}

func (r *recordingBC) add(e bcEvent) { r.mu.Lock(); r.events = append(r.events, e); r.mu.Unlock() }
func (r *recordingBC) BroadcastRunAnalysisProgress(job *models.RunAnalysisJob, _ int) {
	r.add(bcEvent{kind: "progress", analyzed: job.AnalyzedCount})
}
func (r *recordingBC) BroadcastRunAnalysisCompleted(*models.RunAnalysisJob, int) {
	r.add(bcEvent{kind: "completed"})
}
func (r *recordingBC) BroadcastRunResultAnalysisCreated(a *models.RunResultAnalysis, _ string) {
	r.add(bcEvent{kind: "created", a: *a})
}
func (r *recordingBC) BroadcastRunResultAnalysisUpdated(a *models.RunResultAnalysis, _ string) {
	r.add(bcEvent{kind: "updated", a: *a})
}
func (r *recordingBC) count(kind string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, e := range r.events {
		if e.kind == kind {
			n++
		}
	}
	return n
}
func (r *recordingBC) first(kind string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, e := range r.events {
		if e.kind == kind {
			return i
		}
	}
	return -1
}

// gatedNarrator holds each call until release is closed (or its context ends) and keeps the
// last user prompt.
type gatedNarrator struct {
	started chan struct{}
	release chan struct{}
	calls   atomic.Int32
	mu      sync.Mutex
	prompt  string
}

func newGatedNarrator() *gatedNarrator {
	return &gatedNarrator{started: make(chan struct{}, 1), release: make(chan struct{})}
}

func (p *gatedNarrator) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	p.calls.Add(1)
	p.mu.Lock()
	for _, m := range req.Messages {
		if m.Role == "user" {
			p.prompt = m.Content
		}
	}
	p.mu.Unlock()
	select {
	case p.started <- struct{}{}:
	default:
	}
	select {
	case <-p.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return &llm.ChatResponse{Content: `{"summary":"Shared timing race","next_action":"Wait for the button","rationale":"R"}`,
		Model: "mock", FinishReason: "stop", Usage: &llm.ChatUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}}, nil
}

func waitDone(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("the job did not finish")
	}
}

func TestWorker_TwoPhase_DecisionPublishedBeforeNarration(t *testing.T) {
	s := newStore(t)
	run := seedRunWithFailures(t, s, [][2]string{
		{"timeout", "Timeout waiting for #checkout button after 5000ms"},
		{"timeout", "Timeout waiting for #checkout button after 5000ms"}, // signature clone
		{"timeout", "Timeout waiting for #checkout button after 7000ms"}, // semantic clone
	})
	_, err := s.UpdateFailureAnalysisSettings(&models.AIFailureAnalysisSettings{
		MaxAnalysesPerRun: 10, DedupEnabled: true, RedactionEnabled: false, PromptTemplate: failureanalysis.DefaultPromptTemplate,
		ParallelGroups: 1,
	})
	require.NoError(t, err)
	job, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	ts := &fakeTS{fn: tsVerdict("flaky_test", "automation_bug", 0.95)}
	nar := newGatedNarrator()
	deps := failureanalysis.JobDeps{
		Narrative: nar, NarrativeModel: "mock",
		Decider: failureanalysis.NewTypeSafeDecider(ts, "jev-1.13.0"), DeciderModel: "jev-1.13.0",
		Semantic: &failureanalysis.SemanticDeps{Client: ts, Model: "jev-1.13.0"},
	}
	bc := &recordingBC{}
	w := NewWorker(s, staticResolver(deps), bc, 10*time.Millisecond)
	done := make(chan error, 1)
	go func() { done <- w.processOnce(context.Background()) }()
	select {
	case <-nar.started:
	case <-time.After(10 * time.Second):
		t.Fatal("narration never started")
	}

	// The decision is stored, counted and published while its explanation is being written.
	rows, err := s.GetCurrentAnalysesByRun(run.ID)
	require.NoError(t, err)
	require.Len(t, rows, 3)
	for _, a := range rows {
		require.Equal(t, models.NarrativeStatusPending, a.NarrativeStatus)
		require.Equal(t, models.VerdictFlakyTest, a.Verdict)
	}
	got, err := s.GetAnalysisJob(job.ID)
	require.NoError(t, err)
	require.Equal(t, 1, got.AnalyzedCount, "progress counts the group on its decision")
	o, err := s.AnalysisJobOutcomes(job.ID)
	require.NoError(t, err)
	require.Equal(t, 1, o.ExplanationPending)
	require.Equal(t, 3, bc.count("created"))
	require.Zero(t, bc.count("updated"))

	close(nar.release)
	waitDone(t, done)

	rows, err = s.GetCurrentAnalysesByRun(run.ID)
	require.NoError(t, err)
	var repID string
	for _, a := range rows {
		require.Equal(t, models.NarrativeStatusOK, a.NarrativeStatus)
		require.Equal(t, "Shared timing race", a.Summary)
		require.Equal(t, "Wait for the button", a.NextAction)
		require.Equal(t, 1, a.NarrativeRevision)
		require.Equal(t, 1, a.Version, "an explanation is not a new version")
		if a.SourceAnalysisID == nil {
			repID = a.ID
			require.Equal(t, 10, a.TokenUsagePrompt)
			require.Equal(t, 5, a.TokenUsageCompletion)
			require.Equal(t, 1, a.LLMCalls)
			require.Equal(t, "stop", a.FinishReason)
			require.True(t, strings.HasSuffix(a.Rationale, "R"), a.Rationale)
		} else {
			require.Zero(t, a.TokenUsagePrompt+a.TokenUsageCompletion+a.LLMCalls+a.LLMMs, "usage stays on the representative")
			require.Empty(t, a.RawResponse)
			require.True(t, strings.HasPrefix(a.Rationale, "[Grouped"), a.Rationale)
			require.True(t, strings.HasSuffix(a.Rationale, "] R"), a.Rationale)
		}
	}
	require.NotEmpty(t, repID)
	require.Equal(t, 3, bc.count("updated"), "rep and both clones are republished")
	require.Less(t, bc.first("progress"), bc.first("updated"), "progress moves on the decision, before the explanation")
	require.Equal(t, int32(1), nar.calls.Load(), "one explanation per group")
	nar.mu.Lock()
	prompt := nar.prompt
	nar.mu.Unlock()
	require.Contains(t, prompt, "after 7000ms", "the semantic member's error reaches the group's one narration")

	o, err = s.AnalysisJobOutcomes(job.ID)
	require.NoError(t, err)
	require.Zero(t, o.ExplanationPending)
	require.Equal(t, 1, o.Decided)

	// One cost event per billed call and phase: no second TypeSafe event for the narration.
	events, err := s.ListAnalysisCostEventsForRun(run.ID)
	require.NoError(t, err)
	byKey := map[string]int{}
	for _, e := range events {
		byKey[e.Kind+"/"+e.Engine]++
		if e.Kind == models.AnalysisCostKindAnalysis {
			require.Equal(t, repID, *e.AnalysisID)
		}
	}
	require.Equal(t, map[string]int{"semantic/typesafe": 1, "analysis/typesafe": 1, "analysis/llm": 1}, byKey)
}

func TestWorker_DecideFailureStoresAFailedRowAndNeverNarrates(t *testing.T) {
	s := newStore(t)
	run := seedRunWithFailures(t, s, [][2]string{{"timeout", "Timeout waiting for #checkout button after 5000ms"}})
	job, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	ts := &fakeTS{fn: func(typesafe.Request) (*typesafe.Response, error) {
		return nil, &typesafe.Error{Status: 529, Category: typesafe.CategoryOverloaded}
	}}
	prov := &countingProvider{verdictProvider: verdictProvider{verdict: "product_bug"}}
	deps := failureanalysis.JobDeps{Narrative: prov, NarrativeModel: "mock", NoLLMFallback: true,
		Decider: failureanalysis.NewTypeSafeDecider(ts, "jev-1.13.0"), DeciderModel: "jev-1.13.0"}
	bc := &recordingBC{}
	w := NewWorker(s, staticResolver(deps), bc, 10*time.Millisecond)
	require.NoError(t, w.processOnce(context.Background()))

	require.Zero(t, prov.calls.Load())
	rows, err := s.GetCurrentAnalysesByRun(run.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	for _, a := range rows {
		require.Equal(t, models.DecisionStatusFailed, a.DecisionStatus)
		require.Equal(t, models.NarrativeStatusUnavailable, a.NarrativeStatus)
		require.Equal(t, models.AnalysisEngineTypeSafe, a.Engine)
		require.NotEmpty(t, a.ErrorCategory)
		require.Zero(t, a.NarrativeRevision)
	}
	o, err := s.AnalysisJobOutcomes(job.ID)
	require.NoError(t, err)
	require.Equal(t, 1, o.Failed)
	require.Zero(t, o.ExplanationPending)
	require.Equal(t, 1, bc.count("created"))
	require.Zero(t, bc.count("updated"))
}

func TestWorker_CancelBetweenPhasesLeavesNoPendingRows(t *testing.T) {
	old := cancelPoll
	cancelPoll = 20 * time.Millisecond
	defer func() { cancelPoll = old }()
	s := newStore(t)
	run := seedRunWithFailures(t, s, [][2]string{
		{"timeout", "Timeout waiting for #checkout button after 5000ms"},
		{"timeout", "Timeout waiting for #checkout button after 5000ms"},
	})
	job, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	ts := &fakeTS{fn: tsVerdict("flaky_test", "automation_bug", 0.95)}
	nar := newGatedNarrator() // never released
	deps := failureanalysis.JobDeps{Narrative: nar, NarrativeModel: "mock",
		Decider: failureanalysis.NewTypeSafeDecider(ts, "jev-1.13.0"), DeciderModel: "jev-1.13.0"}
	bc := &recordingBC{}
	w := NewWorker(s, staticResolver(deps), bc, 10*time.Millisecond)
	done := make(chan error, 1)
	go func() { done <- w.processOnce(context.Background()) }()
	select {
	case <-nar.started:
	case <-time.After(10 * time.Second):
		t.Fatal("narration never started")
	}
	_, err = s.UpdateAnalysisJobStatus(job.ID, models.RunAnalysisJobStatusCancelled, "")
	require.NoError(t, err)
	waitDone(t, done)

	got, err := s.GetAnalysisJob(job.ID)
	require.NoError(t, err)
	require.Equal(t, models.RunAnalysisJobStatusCancelled, got.Status)
	rows, err := s.GetCurrentAnalysesByRun(run.ID)
	require.NoError(t, err)
	require.Len(t, rows, 2, "the decision stored before the cancel is kept")
	for _, a := range rows {
		require.Equal(t, models.VerdictFlakyTest, a.Verdict)
		require.Equal(t, models.NarrativeStatusUnavailable, a.NarrativeStatus)
		require.Contains(t, a.Summary, "before the explanation was written")
		require.GreaterOrEqual(t, a.NarrativeRevision, 1)
	}
	o, err := s.AnalysisJobOutcomes(job.ID)
	require.NoError(t, err)
	require.Zero(t, o.ExplanationPending)
	require.GreaterOrEqual(t, bc.count("updated"), 2, "live views learn the explanation is not coming")
}

func TestWorker_RepresentativePersistFailureSkipsNarration(t *testing.T) {
	s := newStore(t)
	run := seedRunWithFailures(t, s, [][2]string{{"timeout", "Timeout waiting for #checkout button after 5000ms"}})
	job, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	require.NoError(t, s.DB().Exec(`CREATE TRIGGER no_analyses BEFORE INSERT ON run_result_analyses
		BEGIN SELECT RAISE(ABORT, 'disk full'); END`).Error)
	ts := &fakeTS{fn: tsVerdict("flaky_test", "automation_bug", 0.95)}
	prov := &countingProvider{verdictProvider: verdictProvider{verdict: "product_bug"}}
	deps := failureanalysis.JobDeps{Narrative: prov, NarrativeModel: "mock",
		Decider: failureanalysis.NewTypeSafeDecider(ts, "jev-1.13.0"), DeciderModel: "jev-1.13.0"}
	w := NewWorker(s, staticResolver(deps), nil, 10*time.Millisecond)
	done := make(chan error, 1)
	go func() { done <- w.processOnce(context.Background()) }()
	waitDone(t, done) // no group is left waiting for an ack

	require.Zero(t, prov.calls.Load(), "a decision that could not be stored is never explained")
	got, err := s.GetAnalysisJob(job.ID)
	require.NoError(t, err)
	require.Equal(t, models.RunAnalysisJobStatusCompleted, got.Status)
	events, err := s.ListAnalysisCostEventsForRun(run.ID)
	require.NoError(t, err)
	require.Len(t, events, 1, "the TypeSafe call was billed although its row was lost")
	require.Nil(t, events[0].AnalysisID)
}
