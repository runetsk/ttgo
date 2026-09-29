package worker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/typesafe"

	"github.com/stretchr/testify/require"
)

// blockingTS answers nothing until its context ends.
type blockingTS struct{ started chan struct{} }

func (b blockingTS) Evaluate(ctx context.Context, _ typesafe.Request) (*typesafe.Response, error) {
	select {
	case b.started <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return nil, ctx.Err()
}
func (blockingTS) ListModels(context.Context) ([]typesafe.Model, error) { return nil, nil }

func semanticGroup() []*failureanalysis.FailureGroup {
	rep := &models.RunResult{ID: "rr-1", FailureType: "timeout", ErrorMessage: "boom after 5000ms"}
	sem := &models.RunResult{ID: "rr-2", FailureType: "timeout", ErrorMessage: "boom after 7000ms"}
	return []*failureanalysis.FailureGroup{{Key: "k", Representative: rep, Members: []*models.RunResult{rep, sem},
		SemanticMembers: map[string]float64{"rr-2": 0.93}}}
}

// realTransfer is the worker's check against client, as processOnce wires it.
func realTransfer(client typesafe.Client) func(context.Context, *failureanalysis.FailureGroup, failureanalysis.NarrationDelta) failureanalysis.NarrationDelta {
	return func(ctx context.Context, g *failureanalysis.FailureGroup, d failureanalysis.NarrationDelta) failureanalysis.NarrationDelta {
		in := failureanalysis.TransferInputFor(d, g.Representative, failureanalysis.SemanticClones(g), false)
		return failureanalysis.RunTransferCheck(ctx, failureanalysis.TransferDeps{Client: client}, in, d)
	}
}

func TestAnalyzeGroups_TransferCheckRidesOnAnOkNarration(t *testing.T) {
	w := &Worker{store: newStore(t)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls, checks atomic.Int32
	var checkedKey atomic.Value
	fns := phaseFns(models.NarrativeStatusPending, &calls, okNarration)
	fns.transfer = func(_ context.Context, g *failureanalysis.FailureGroup, d failureanalysis.NarrationDelta) failureanalysis.NarrationDelta {
		checks.Add(1)
		checkedKey.Store(g.Key) // asserted on the test goroutine below
		d.Fits = map[string]float64{"rr-2": 0.2}
		return d
	}
	out := w.analyzeGroups(ctx, cancel, "job", semanticGroup(), 1, time.Minute, fns)
	decided, ok := recvOutcome(t, out)
	require.True(t, ok)
	require.Zero(t, checks.Load(), "nothing is checked before the decision is stored")
	decided.ack <- decidedAck{repID: "rep-1"}
	narrated, ok := recvOutcome(t, out)
	require.True(t, ok)
	require.Equal(t, phaseNarrated, narrated.phase)
	require.Equal(t, map[string]float64{"rr-2": 0.2}, narrated.delta.Fits, "the fits travel with the explanation, in one apply")
	require.Equal(t, int32(1), checks.Load())
	require.Equal(t, "k", checkedKey.Load(), "the check sees its own group")
	_, ok = recvOutcome(t, out)
	require.False(t, ok)
}

func TestAnalyzeGroups_NoCheckWithoutAWrittenExplanation(t *testing.T) {
	w := &Worker{store: newStore(t)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls, checks atomic.Int32
	fns := phaseFns(models.NarrativeStatusPending, &calls, func(context.Context) failureanalysis.NarrationDelta {
		return failureanalysis.NarrationDelta{NarrativeStatus: models.NarrativeStatusUnavailable, Reason: "timeout", Summary: "AI narrative unavailable: timeout"}
	})
	fns.transfer = func(_ context.Context, _ *failureanalysis.FailureGroup, d failureanalysis.NarrationDelta) failureanalysis.NarrationDelta {
		checks.Add(1)
		return d
	}
	out := w.analyzeGroups(ctx, cancel, "job", semanticGroup(), 1, time.Minute, fns)
	decided, _ := recvOutcome(t, out)
	decided.ack <- decidedAck{repID: "rep-1"}
	narrated, ok := recvOutcome(t, out)
	require.True(t, ok)
	require.Equal(t, models.NarrativeStatusUnavailable, narrated.delta.NarrativeStatus)
	require.Zero(t, checks.Load())
}

func TestAnalyzeGroups_DeadlineDuringTheCheckStillDeliversTheExplanation(t *testing.T) {
	w := &Worker{store: newStore(t)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	fns := phaseFns(models.NarrativeStatusPending, &calls, okNarration)
	fns.transfer = realTransfer(blockingTS{started: make(chan struct{}, 1)})
	out := w.analyzeGroups(ctx, cancel, "job", semanticGroup(), 1, 300*time.Millisecond, fns)
	decided, _ := recvOutcome(t, out)
	decided.ack <- decidedAck{repID: "rep-1"}

	narrated, ok := recvOutcome(t, out)
	require.True(t, ok)
	require.Equal(t, models.NarrativeStatusOK, narrated.delta.NarrativeStatus, "the explanation is applied")
	require.Equal(t, "S", narrated.delta.Summary)
	require.True(t, narrated.delta.TransferFailed, "the group counts in transfer_check_failed")
	require.Nil(t, narrated.delta.Fits, "fits stay NULL")
	_, ok = recvOutcome(t, out)
	require.False(t, ok)
}

func TestAnalyzeGroups_CancelDuringTheCheckStillDeliversTheExplanation(t *testing.T) {
	w := &Worker{store: newStore(t)}
	ctx, cancel := context.WithCancel(context.Background())
	var calls atomic.Int32
	ts := blockingTS{started: make(chan struct{}, 1)}
	fns := phaseFns(models.NarrativeStatusPending, &calls, okNarration)
	fns.transfer = realTransfer(ts)
	out := w.analyzeGroups(ctx, cancel, "job", semanticGroup(), 1, time.Minute, fns)
	decided, _ := recvOutcome(t, out)
	decided.ack <- decidedAck{repID: "rep-1"}
	<-ts.started
	cancel() // the job is cancelled while TypeSafe checks the explanation

	narrated, ok := recvOutcome(t, out)
	require.True(t, ok, "a finished explanation is kept after a cancel")
	require.Equal(t, models.NarrativeStatusOK, narrated.delta.NarrativeStatus)
	require.True(t, narrated.delta.TransferFailed)
	_, ok = recvOutcome(t, out)
	require.False(t, ok)
}

// transferEnv seeds a representative, a signature clone and a semantic clone, and answers the
// verdict and the pair question like tsVerdict; fits requests go to fits.
func transferEnv(t *testing.T, fits func(req typesafe.Request) (*typesafe.Response, error)) (*Worker, *models.TestRun, *models.RunAnalysisJob) {
	t.Helper()
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
	base := tsVerdict("flaky_test", "automation_bug", 0.95)
	ts := &fakeTS{fn: func(req typesafe.Request) (*typesafe.Response, error) {
		if _, ok := req.Questions["fits_1"]; ok {
			return fits(req)
		}
		return base(req)
	}}
	deps := failureanalysis.JobDeps{
		Narrative: &verdictProvider{verdict: "product_bug"}, NarrativeModel: "mock",
		Decider: failureanalysis.NewTypeSafeDecider(ts, "jev-1.13.0"), DeciderModel: "jev-1.13.0",
		Semantic: &failureanalysis.SemanticDeps{Client: ts, Model: "jev-1.13.0"},
		Transfer: &failureanalysis.TransferDeps{Client: ts, Model: "jev-1.13.0"},
		Pricing:  failureanalysis.Pricing{TypeSafePerMTok: 0.042},
	}
	return NewWorker(s, staticResolver(deps), nil, 10*time.Millisecond), run, job
}

func rowsByMethod(t *testing.T, w *Worker, runID string) map[string]*models.RunResultAnalysis {
	t.Helper()
	rows, err := w.store.GetCurrentAnalysesByRun(runID)
	require.NoError(t, err)
	out := map[string]*models.RunResultAnalysis{}
	for _, a := range rows {
		key := a.DedupMethod
		if key == "" {
			key = "rep"
		}
		out[key] = a
	}
	require.Len(t, out, 3, "a representative, a signature clone and a semantic clone")
	return out
}

func TestWorker_LowFitFlagsTheSemanticClone(t *testing.T) {
	var mu sync.Mutex
	var fitsReq typesafe.Request
	w, run, job := transferEnv(t, func(req typesafe.Request) (*typesafe.Response, error) {
		mu.Lock()
		fitsReq = req
		mu.Unlock()
		return &typesafe.Response{Model: "jev-1.13.0", Usage: typesafe.Usage{InputTokens: 70},
			Answers: map[string]typesafe.Answer{"fits_1": {Type: "noul", Noul: 0.2}}}, nil
	})
	require.NoError(t, w.processOnce(context.Background()))

	rows := rowsByMethod(t, w, run.ID)
	for _, a := range rows {
		require.Equal(t, models.NarrativeStatusOK, a.NarrativeStatus)
		require.Equal(t, "s", a.Summary)
	}
	require.Nil(t, rows["rep"].NarrativeFit)
	require.Nil(t, rows[models.DedupMethodSignature].NarrativeFit, "signature clones share the error by construction")
	require.NotNil(t, rows[models.DedupMethodSemantic].NarrativeFit)
	require.InDelta(t, 0.2, *rows[models.DedupMethodSemantic].NarrativeFit, 1e-12)

	mu.Lock()
	failures := fitsReq.State.(map[string]any)["failures"].([]map[string]any)
	mu.Unlock()
	require.Len(t, failures, 2, "the representative and the one semantic clone")
	require.Contains(t, failures[0]["error"], "after 5000ms")
	require.Contains(t, failures[1]["error"], "after 7000ms")

	o, err := w.store.AnalysisJobOutcomes(job.ID)
	require.NoError(t, err)
	require.Equal(t, 1, o.TransferMismatch)
	require.Zero(t, o.TransferCheckFailed)
	require.Zero(t, o.TransferUnchecked)

	events, err := w.store.ListAnalysisCostEventsForRun(run.ID)
	require.NoError(t, err)
	var transfer []*models.AIAnalysisCostEvent
	for _, ev := range events {
		if ev.Kind == models.AnalysisCostKindTransfer {
			transfer = append(transfer, ev)
		}
	}
	require.Len(t, transfer, 1)
	require.Equal(t, models.AnalysisCostEngineTypeSafe, transfer[0].Engine)
	require.Equal(t, 70, transfer[0].TypeSafeInputTokens)
	require.Equal(t, "jev-1.13.0", transfer[0].Model)
	require.Equal(t, rows["rep"].ID, *transfer[0].AnalysisID)
	require.Equal(t, job.ID, *transfer[0].JobID)
	require.InDelta(t, 70*0.042/1e6, *transfer[0].EstimatedCost, 1e-15)
}

func TestWorker_AFailedCheckKeepsTheExplanationAndCountsTheGroup(t *testing.T) {
	w, run, job := transferEnv(t, func(typesafe.Request) (*typesafe.Response, error) {
		return nil, errors.New("typesafe down")
	})
	require.NoError(t, w.processOnce(context.Background()))

	rows := rowsByMethod(t, w, run.ID)
	for _, a := range rows {
		require.Equal(t, models.NarrativeStatusOK, a.NarrativeStatus, "a failed check never withholds the explanation")
		require.Nil(t, a.NarrativeFit)
	}
	o, err := w.store.AnalysisJobOutcomes(job.ID)
	require.NoError(t, err)
	require.Equal(t, 1, o.TransferCheckFailed)
	require.Zero(t, o.TransferMismatch)
	events, err := w.store.ListAnalysisCostEventsForRun(run.ID)
	require.NoError(t, err)
	for _, ev := range events {
		require.NotEqual(t, models.AnalysisCostKindTransfer, ev.Kind, "nothing was billed")
	}
}
