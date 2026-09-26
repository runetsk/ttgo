package llm

import (
	"context"
	"testing"
	"time"

	"ttgo/pkg/tracker/callstats"

	"github.com/stretchr/testify/require"
)

func TestChatWithRetry_CountsRateLimitsIncludingRetriedOnes(t *testing.T) {
	var slept []time.Duration
	p := &scriptedProvider{results: []func() (*ChatResponse, error){
		fail(&ProviderError{Category: ErrCatRateLimit, StatusCode: 429, Message: "slow down"}),
		fail(&ProviderError{Category: ErrCatProvider, StatusCode: 503, Message: "down"}),
		ok(),
	}}
	ctx, counter := callstats.WithCounter(context.Background())
	_, retries, err := ChatWithRetry(ctx, p, ChatRequest{}, testOpts(&slept))
	require.NoError(t, err)
	require.Equal(t, 2, retries)
	require.Equal(t, 1, counter.RateLimitHits(), "only the 429 counts; the 503 is not a rate limit")
}

func TestChatWithRetry_CountsAFinalRateLimit(t *testing.T) {
	var slept []time.Duration
	p := &scriptedProvider{results: []func() (*ChatResponse, error){
		fail(&ProviderError{Category: ErrCatRateLimit, StatusCode: 429, Message: "slow down"}),
	}}
	ctx, counter := callstats.WithCounter(context.Background())
	_, _, err := ChatWithRetry(ctx, p, ChatRequest{}, testOpts(&slept))
	require.Error(t, err)
	require.Equal(t, 3, counter.RateLimitHits(), "all three attempts were rate limited")
}
