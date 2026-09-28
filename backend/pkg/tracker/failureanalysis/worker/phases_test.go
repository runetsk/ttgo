package worker

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

// recvOutcome reads one outcome; ok=false means the channel closed. Fails the test when neither
// happens in time, i.e. a goroutine is stuck.
func recvOutcome(t *testing.T, ch <-chan groupOutcome) (groupOutcome, bool) {
	t.Helper()
	select {
	case o, ok := <-ch:
		return o, ok
	case <-time.After(5 * time.Second):
		t.Fatal("no outcome and the outcome channel did not close: a group goroutine is stranded")
		return groupOutcome{}, false
	}
}

func oneGroup() []*failureanalysis.FailureGroup {
	rr := &models.RunResult{ID: "rr-1", FailureType: "timeout", ErrorMessage: "boom"}
	return []*failureanalysis.FailureGroup{{Key: "k", Representative: rr, Members: []*models.RunResult{rr}}}
}

// phaseFns decides with the given narrative status; narrate runs body and counts its calls.
func phaseFns(status string, calls *atomic.Int32, body func(ctx context.Context) failureanalysis.NarrationDelta) groupFuncs {
	return groupFuncs{
		decide: func(context.Context, *failureanalysis.FailureGroup) (*failureanalysis.AnalyzeResult, failureanalysis.AnalyzeContext, error) {
			return &failureanalysis.AnalyzeResult{Verdict: models.VerdictFlakyTest, NarrativeStatus: status,
				DecisionStatus: models.DecisionStatusOK}, failureanalysis.AnalyzeContext{}, nil
		},
		narrate: func(ctx context.Context, _ failureanalysis.AnalyzeContext, _ *failureanalysis.AnalyzeResult) (failureanalysis.NarrationDelta, bool) {
			calls.Add(1)
			return body(ctx), true
		},
	}
}

func okNarration(context.Context) failureanalysis.NarrationDelta {
	return failureanalysis.NarrationDelta{Summary: "S", NarrativeStatus: models.NarrativeStatusOK, LLMCalls: 1}
}

func TestAnalyzeGroups_BuffersTwoOutcomesPerGroup(t *testing.T) {
	w := &Worker{store: newStore(t)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	groups := append(oneGroup(), oneGroup()...)
	out := w.analyzeGroups(ctx, cancel, "job", groups, 1, time.Minute, phaseFns(models.NarrativeStatusSkipped, &calls, okNarration))
	require.Equal(t, 4, cap(out))
	for {
		if _, ok := recvOutcome(t, out); !ok {
			break
		}
	}
}

func TestAnalyzeGroups_PendingDecisionNarratesAfterTheAck(t *testing.T) {
	w := &Worker{store: newStore(t)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	out := w.analyzeGroups(ctx, cancel, "job", oneGroup(), 1, time.Minute, phaseFns(models.NarrativeStatusPending, &calls, okNarration))

	decided, ok := recvOutcome(t, out)
	require.True(t, ok)
	require.Equal(t, phaseDecided, decided.phase)
	require.Zero(t, calls.Load(), "no narration before the decision is stored")
	decided.ack <- decidedAck{repID: "rep-1"}

	narrated, ok := recvOutcome(t, out)
	require.True(t, ok)
	require.Equal(t, phaseNarrated, narrated.phase)
	require.Equal(t, "rep-1", narrated.repAnalysisID)
	require.Equal(t, "S", narrated.delta.Summary)
	_, ok = recvOutcome(t, out)
	require.False(t, ok)
}

func TestAnalyzeGroups_FinalDecisionNeverWaitsForAnAck(t *testing.T) {
	w := &Worker{store: newStore(t)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	out := w.analyzeGroups(ctx, cancel, "job", oneGroup(), 1, time.Minute, phaseFns(models.NarrativeStatusSkipped, &calls, okNarration))
	decided, ok := recvOutcome(t, out)
	require.True(t, ok)
	require.Equal(t, phaseDecided, decided.phase)
	_, ok = recvOutcome(t, out) // never acked: the group still ends
	require.False(t, ok)
	require.Zero(t, calls.Load())
}

func TestAnalyzeGroups_AckErrorSkipsNarration(t *testing.T) {
	w := &Worker{store: newStore(t)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	out := w.analyzeGroups(ctx, cancel, "job", oneGroup(), 1, time.Minute, phaseFns(models.NarrativeStatusPending, &calls, okNarration))
	decided, ok := recvOutcome(t, out)
	require.True(t, ok)
	decided.ack <- decidedAck{err: errors.New("disk full")}
	_, ok = recvOutcome(t, out)
	require.False(t, ok, "the group ends without a narrated outcome")
	require.Zero(t, calls.Load(), "a decision that was not stored is never explained")
}

func TestAnalyzeGroups_WriterGoneBeforeAckStrandsNothing(t *testing.T) {
	w := &Worker{store: newStore(t)}
	ctx, cancel := context.WithCancel(context.Background())
	var calls atomic.Int32
	out := w.analyzeGroups(ctx, cancel, "job", oneGroup(), 1, time.Minute, phaseFns(models.NarrativeStatusPending, &calls, okNarration))
	_, ok := recvOutcome(t, out)
	require.True(t, ok)
	cancel() // the writer returned (server stopping) without answering; processOnce's defer cancels the job
	_, ok = recvOutcome(t, out)
	require.False(t, ok, "the channel closes, so the feeder and every group goroutine have returned")
	require.Zero(t, calls.Load())
}

func TestAnalyzeGroups_CancelDuringNarrationSendsAnUnavailableDelta(t *testing.T) {
	w := &Worker{store: newStore(t)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	started := make(chan struct{})
	out := w.analyzeGroups(ctx, cancel, "job", oneGroup(), 1, time.Minute, phaseFns(models.NarrativeStatusPending, &calls,
		func(ctx context.Context) failureanalysis.NarrationDelta {
			close(started)
			<-ctx.Done()
			// What P2's Narrate returns for a cut-off call: the internal "cancelled" reason.
			return failureanalysis.NarrationDelta{NarrativeStatus: models.NarrativeStatusUnavailable, Reason: "cancelled", LLMCalls: 1, LLMMs: 40}
		}))
	decided, _ := recvOutcome(t, out)
	decided.ack <- decidedAck{repID: "rep-1"}
	<-started
	cancel()

	narrated, ok := recvOutcome(t, out)
	require.True(t, ok)
	require.Equal(t, phaseNarrated, narrated.phase)
	require.Equal(t, "rep-1", narrated.repAnalysisID)
	require.Equal(t, models.NarrativeStatusUnavailable, narrated.delta.NarrativeStatus)
	require.Contains(t, narrated.delta.Summary, "before the explanation was written")
	require.NotEqual(t, "cancelled", narrated.delta.Reason, "the internal marker never reaches a row")
	require.Equal(t, 1, narrated.delta.LLMCalls, "the abandoned call's usage is kept: it was billed")
	_, ok = recvOutcome(t, out)
	require.False(t, ok)
}
