package typesafe

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"

	"ttgo/pkg/tracker/callstats"

	"github.com/stretchr/testify/require"
)

// Every 429 counts, including the ones a retry then gets past.
func TestEvaluate_CountsEveryRateLimitOnTheContext(t *testing.T) {
	var calls int32
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(okBody()))
	})
	ctx, counter := callstats.WithCounter(context.Background())
	_, err := c.Evaluate(ctx, Request{State: "s", Model: "m", Questions: choiceQ()})
	require.NoError(t, err)
	require.Equal(t, 2, counter.RateLimitHits(), "both 429s count although the call succeeded")
}

func TestEvaluate_OtherErrorsAreNotRateLimits(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	ctx, counter := callstats.WithCounter(context.Background())
	_, err := c.Evaluate(ctx, Request{State: "s", Model: "m", Questions: choiceQ()})
	require.Error(t, err)
	require.Equal(t, 0, counter.RateLimitHits())
}
