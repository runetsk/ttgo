package callstats

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCounter_CountsOnItsContextAndChildren(t *testing.T) {
	ctx, c := WithCounter(context.Background())
	RecordRateLimit(ctx)
	RecordRateLimit(ctx)
	require.Equal(t, 2, c.RateLimitHits())

	child, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	RecordRateLimit(child)
	require.Equal(t, 3, c.RateLimitHits(), "a derived context reports into the same counter")

	RecordRateLimit(context.Background()) // no counter: a no-op, never a panic
	require.Equal(t, 3, c.RateLimitHits())

	var none *Counter
	require.Equal(t, 0, none.RateLimitHits())
}

func TestCounter_IsSafeForParallelGroups(t *testing.T) {
	ctx, c := WithCounter(context.Background())
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			RecordRateLimit(ctx)
		}()
	}
	wg.Wait()
	require.Equal(t, 50, c.RateLimitHits())
}

func TestCounter_LatencyCounters(t *testing.T) {
	ctx, c := WithCounter(context.Background())
	RecordCallTimeout(ctx)
	RecordHedgeFired(ctx)
	RecordHedgeFired(ctx)
	RecordHedgeWon(ctx)
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	RecordHedgePrompt(child, 1200)
	RecordHedgePrompt(child, 0)
	require.Equal(t, 1, c.CallTimeouts())
	require.Equal(t, 2, c.HedgesFired())
	require.Equal(t, 1, c.HedgesWon())
	require.Equal(t, []int{1200, 0}, c.HedgePromptTokens())
	require.Equal(t, 0, c.RateLimitHits(), "the counters are independent")

	got := c.HedgePromptTokens()
	got[0] = 7
	require.Equal(t, 1200, c.HedgePromptTokens()[0], "callers get a copy")

	// No counter on the context, or a nil counter: no-ops, never panics.
	RecordCallTimeout(context.Background())
	RecordHedgeFired(context.Background())
	RecordHedgeWon(context.Background())
	RecordHedgePrompt(context.Background(), 5)
	var none *Counter
	require.Equal(t, 0, none.CallTimeouts())
	require.Equal(t, 0, none.HedgesFired())
	require.Equal(t, 0, none.HedgesWon())
	require.Nil(t, none.HedgePromptTokens())
}

func TestCounter_HedgePromptsAreSafeForParallelGroups(t *testing.T) {
	ctx, c := WithCounter(context.Background())
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			RecordHedgeFired(ctx)
			RecordHedgePrompt(ctx, 10)
		}()
	}
	wg.Wait()
	require.Equal(t, 50, c.HedgesFired())
	require.Len(t, c.HedgePromptTokens(), 50)
}
