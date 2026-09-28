package llm

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"ttgo/pkg/tracker/callstats"

	"github.com/stretchr/testify/require"
)

// branchProvider runs the behaviour for its n-th call (0-based; the last one repeats) and
// tracks how many calls are still running, so tests can prove no request outlives Chat's caller.
type branchProvider struct {
	behaviors []func(ctx context.Context) (*ChatResponse, error)
	calls     atomic.Int32
	active    atomic.Int32
}

func (b *branchProvider) Chat(ctx context.Context, _ ChatRequest) (*ChatResponse, error) {
	b.active.Add(1)
	defer b.active.Add(-1)
	i := int(b.calls.Add(1)) - 1
	if i >= len(b.behaviors) {
		i = len(b.behaviors) - 1
	}
	return b.behaviors[i](ctx)
}

func stall(ctx context.Context) (*ChatResponse, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func answerAfter(d time.Duration, content string, prompt int) func(context.Context) (*ChatResponse, error) {
	return func(ctx context.Context) (*ChatResponse, error) {
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-t.C:
			return &ChatResponse{Content: content, Usage: &ChatUsage{PromptTokens: prompt}}, nil
		}
	}
}

func failAfter(d time.Duration, pe *ProviderError) func(context.Context) (*ChatResponse, error) {
	return func(ctx context.Context) (*ChatResponse, error) {
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-t.C:
			return nil, pe
		}
	}
}

func requireAllDone(t *testing.T, b *branchProvider) {
	t.Helper()
	require.Eventually(t, func() bool { return b.active.Load() == 0 }, 2*time.Second, 5*time.Millisecond,
		"every request ends once Chat has returned")
}

func noSleep(attempts int) RetryOptions {
	return RetryOptions{MaxAttempts: attempts, sleep: func(context.Context, time.Duration) error { return nil },
		jitter: func() float64 { return 0 }}
}

var (
	err429 = &ProviderError{Category: ErrCatRateLimit, StatusCode: 429, Message: "slow down"}
	err503 = &ProviderError{Category: ErrCatProvider, StatusCode: 503, Message: "down"}
)

// ── WithCallTimeout ─────────────────────────────────────────────────────────

func TestWithCallTimeout_CutsASlowCallAsARetryableTimeout(t *testing.T) {
	bp := &branchProvider{behaviors: []func(context.Context) (*ChatResponse, error){stall}}
	ctx, counter := callstats.WithCounter(context.Background())
	_, err := WithCallTimeout(bp, 20*time.Millisecond).Chat(ctx, ChatRequest{})
	var pe *ProviderError
	require.ErrorAs(t, err, &pe)
	require.True(t, pe.CallTimeout)
	require.Equal(t, ErrCatTimeout, pe.Category)
	require.True(t, pe.Retryable())
	require.Equal(t, 1, counter.CallTimeouts())
	requireAllDone(t, bp)
}

func TestWithCallTimeout_RetriedOnceByChatWithRetry(t *testing.T) {
	bp := &branchProvider{behaviors: []func(context.Context) (*ChatResponse, error){stall, answerAfter(0, "ok", 1)}}
	ctx, counter := callstats.WithCounter(context.Background())
	resp, retries, err := ChatWithRetry(ctx, WithCallTimeout(bp, 20*time.Millisecond), ChatRequest{}, noSleep(2))
	require.NoError(t, err)
	require.Equal(t, "ok", resp.Content)
	require.Equal(t, 1, retries)
	require.Equal(t, 1, counter.CallTimeouts())
}

func TestWithCallTimeout_ParentDeadlineIsNotRetried(t *testing.T) {
	bp := &branchProvider{behaviors: []func(context.Context) (*ChatResponse, error){stall}}
	parent, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	ctx, counter := callstats.WithCounter(parent)
	_, _, err := ChatWithRetry(ctx, WithCallTimeout(bp, time.Second), ChatRequest{}, noSleep(3))
	require.Error(t, err)
	var pe *ProviderError
	if errors.As(err, &pe) {
		require.False(t, pe.CallTimeout, "the group deadline is not our per-call timeout")
	}
	require.Equal(t, ErrCatTimeout, Classify(err))
	require.EqualValues(t, 1, bp.calls.Load(), "a parent deadline is never retried")
	require.Equal(t, 0, counter.CallTimeouts())
}

func TestWithCallTimeout_PassesOtherResultsThrough(t *testing.T) {
	httpTimeout := &ProviderError{Category: ErrCatTimeout, Message: "LLM request failed: http timeout"}
	bp := &branchProvider{behaviors: []func(context.Context) (*ChatResponse, error){failAfter(0, httpTimeout)}}
	_, err := WithCallTimeout(bp, time.Second).Chat(context.Background(), ChatRequest{})
	require.Same(t, httpTimeout, err, "a provider HTTP timeout keeps CallTimeout false")
	require.False(t, httpTimeout.Retryable())

	okp := &branchProvider{behaviors: []func(context.Context) (*ChatResponse, error){answerAfter(0, "fine", 3)}}
	resp, err := WithCallTimeout(okp, time.Second).Chat(context.Background(), ChatRequest{})
	require.NoError(t, err)
	require.Equal(t, "fine", resp.Content)

	require.Equal(t, Provider(okp), WithCallTimeout(okp, 0), "0 = no timeout: the provider itself")
	require.Nil(t, WithCallTimeout(nil, time.Second))
}

// ── WithHedge ───────────────────────────────────────────────────────────────

func TestWithHedge_NoHedgeWhenTheFirstAnswersInTime(t *testing.T) {
	bp := &branchProvider{behaviors: []func(context.Context) (*ChatResponse, error){answerAfter(0, "first", 100)}}
	ctx, counter := callstats.WithCounter(context.Background())
	resp, err := WithHedge(bp, 200*time.Millisecond).Chat(ctx, ChatRequest{})
	require.NoError(t, err)
	require.Equal(t, "first", resp.Content)
	require.EqualValues(t, 1, bp.calls.Load())
	require.Equal(t, 0, counter.HedgesFired())
	require.Nil(t, counter.HedgePromptTokens())
}

func TestWithHedge_HedgeWinsWhenTheFirstStalls(t *testing.T) {
	bp := &branchProvider{behaviors: []func(context.Context) (*ChatResponse, error){stall, answerAfter(0, "second", 120)}}
	ctx, counter := callstats.WithCounter(context.Background())
	resp, err := WithHedge(bp, 20*time.Millisecond).Chat(ctx, ChatRequest{})
	require.NoError(t, err)
	require.Equal(t, "second", resp.Content)
	require.EqualValues(t, 2, bp.calls.Load())
	require.Equal(t, 1, counter.HedgesFired())
	require.Equal(t, 1, counter.HedgesWon())
	require.Equal(t, []int{120}, counter.HedgePromptTokens())
	requireAllDone(t, bp) // the stalled first request was cancelled
}

func TestWithHedge_TheFirstCanStillWin(t *testing.T) {
	bp := &branchProvider{behaviors: []func(context.Context) (*ChatResponse, error){answerAfter(80*time.Millisecond, "first", 90), stall}}
	ctx, counter := callstats.WithCounter(context.Background())
	resp, err := WithHedge(bp, 20*time.Millisecond).Chat(ctx, ChatRequest{})
	require.NoError(t, err)
	require.Equal(t, "first", resp.Content)
	require.Equal(t, 1, counter.HedgesFired())
	require.Equal(t, 0, counter.HedgesWon())
	require.Equal(t, []int{90}, counter.HedgePromptTokens(), "the fired hedge is priced at the winner's prompt")
	requireAllDone(t, bp)
}

func TestWithHedge_OneBranchRateLimitedTheOtherSucceeds(t *testing.T) {
	bp := &branchProvider{behaviors: []func(context.Context) (*ChatResponse, error){
		failAfter(40*time.Millisecond, err429), answerAfter(80*time.Millisecond, "second", 100)}}
	ctx, counter := callstats.WithCounter(context.Background())
	resp, retries, err := ChatWithRetry(ctx, WithHedge(bp, 10*time.Millisecond), ChatRequest{}, noSleep(1))
	require.NoError(t, err)
	require.Equal(t, "second", resp.Content)
	require.Equal(t, 0, retries)
	require.Equal(t, 1, counter.RateLimitHits(), "the discarded 429 still counts")
	require.Equal(t, 1, counter.HedgesWon())
}

func TestWithHedge_BothFail(t *testing.T) {
	bp := &branchProvider{behaviors: []func(context.Context) (*ChatResponse, error){
		failAfter(30*time.Millisecond, err429), failAfter(60*time.Millisecond, err503)}}
	ctx, counter := callstats.WithCounter(context.Background())
	_, _, err := ChatWithRetry(ctx, WithHedge(bp, 10*time.Millisecond), ChatRequest{}, noSleep(1))
	require.Same(t, err503, err, "fails with the error of the request that failed last")
	require.Equal(t, 1, counter.RateLimitHits(), "the first branch's 429 is counted by the hedge")
	require.Equal(t, 1, counter.HedgesFired())
	require.Equal(t, 0, counter.HedgesWon())
	require.Nil(t, counter.HedgePromptTokens(), "no winner, nothing to price")
	requireAllDone(t, bp)

	both := &branchProvider{behaviors: []func(context.Context) (*ChatResponse, error){
		failAfter(30*time.Millisecond, err429), failAfter(60*time.Millisecond, err429)}}
	ctx2, counter2 := callstats.WithCounter(context.Background())
	_, _, err = ChatWithRetry(ctx2, WithHedge(both, 10*time.Millisecond), ChatRequest{}, noSleep(1))
	require.Error(t, err)
	require.Equal(t, 2, counter2.RateLimitHits(), "each 429 counts exactly once: one discarded, one returned")
}

func TestWithHedge_AFastFailureIsNotHedged(t *testing.T) {
	auth := &ProviderError{Category: ErrCatAuthentication, StatusCode: 401, Message: "401"}
	bp := &branchProvider{behaviors: []func(context.Context) (*ChatResponse, error){failAfter(0, auth)}}
	ctx, counter := callstats.WithCounter(context.Background())
	_, err := WithHedge(bp, 50*time.Millisecond).Chat(ctx, ChatRequest{})
	require.Same(t, auth, err)
	require.EqualValues(t, 1, bp.calls.Load())
	require.Equal(t, 0, counter.HedgesFired())
}

func TestWithHedge_ParentCancelEndsBothRequests(t *testing.T) {
	bp := &branchProvider{behaviors: []func(context.Context) (*ChatResponse, error){stall, stall}}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(40 * time.Millisecond); cancel() }()
	_, err := WithHedge(bp, 10*time.Millisecond).Chat(ctx, ChatRequest{})
	require.ErrorIs(t, err, context.Canceled)
	require.EqualValues(t, 2, bp.calls.Load())
	requireAllDone(t, bp)
}

// The resolver's order: the call timeout outside, the hedge inside. A branch cut by its own
// call timeout is counted where it is raised, once, although the hedge discards it.
func TestWithHedge_CallTimeoutsInsideAreCountedOnce(t *testing.T) {
	// First request: stalls, cut at 100 ms. Hedge: sent at 60 ms, answers 80 ms later (140 ms).
	bp := &branchProvider{behaviors: []func(context.Context) (*ChatResponse, error){stall, answerAfter(80*time.Millisecond, "second", 50)}}
	ctx, counter := callstats.WithCounter(context.Background())
	resp, err := WithHedge(WithCallTimeout(bp, 100*time.Millisecond), 60*time.Millisecond).Chat(ctx, ChatRequest{})
	require.NoError(t, err)
	require.Equal(t, "second", resp.Content)
	require.Equal(t, 1, counter.CallTimeouts())
	require.Equal(t, 0, counter.RateLimitHits())
	require.Equal(t, 1, counter.HedgesWon())
	requireAllDone(t, bp)
}

func TestWithHedge_OffReturnsTheProvider(t *testing.T) {
	bp := &branchProvider{behaviors: []func(context.Context) (*ChatResponse, error){stall}}
	require.Equal(t, Provider(bp), WithHedge(bp, 0))
	require.Nil(t, WithHedge(nil, time.Second))
}
