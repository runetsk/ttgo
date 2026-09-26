package typesafe

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// countingLimiter records every Wait and answers with err (nil = go ahead).
type countingLimiter struct {
	calls atomic.Int32
	err   error
}

func (l *countingLimiter) Wait(context.Context) error {
	l.calls.Add(1)
	return l.err
}

func TestRPMLimiter_BurstThenPaces(t *testing.T) {
	clock := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	var slept []time.Duration
	l := newRPMLimiter(120, func() time.Time { return clock }, func(_ context.Context, d time.Duration) error {
		slept = append(slept, d)
		clock = clock.Add(d)
		return nil
	})
	// 120/min = 2 per second with a burst of 2: two go at once, the third waits half a second.
	for i := 0; i < 3; i++ {
		require.NoError(t, l.Wait(context.Background()))
	}
	require.Len(t, slept, 1)
	require.InDelta(t, float64(500*time.Millisecond), float64(slept[0]), float64(time.Millisecond))
}

func TestRPMLimiter_CancelledWaitReturnsPromptlyAndRefunds(t *testing.T) {
	l := NewRPMLimiter(1).(*rpmLimiter) // one per minute, burst 1
	require.NoError(t, l.Wait(context.Background()))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := l.Wait(ctx)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, time.Since(start), 2*time.Second)
	l.mu.Lock()
	defer l.mu.Unlock()
	require.InDelta(t, 0, l.tokens, 0.01, "the cancelled wait gives its token back")
}

func TestNewRPMLimiter_ZeroOrNegativeDisables(t *testing.T) {
	require.Nil(t, NewRPMLimiter(0))
	require.Nil(t, NewRPMLimiter(-5))
}

func TestLimiterFromEnv(t *testing.T) {
	rate := func(l Limiter) float64 { return l.(*rpmLimiter).rate }
	t.Setenv(EnvRPM, "")
	require.InDelta(t, float64(DefaultRPM)/60, rate(LimiterFromEnv()), 1e-9)
	t.Setenv(EnvRPM, "300")
	require.InDelta(t, 5.0, rate(LimiterFromEnv()), 1e-9)
	t.Setenv(EnvRPM, "0")
	require.Nil(t, LimiterFromEnv())
	t.Setenv(EnvRPM, "lots")
	require.InDelta(t, float64(DefaultRPM)/60, rate(LimiterFromEnv()), 1e-9, "an unreadable value falls back to the default")
}
