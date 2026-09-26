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
