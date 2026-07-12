package llm

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scriptedProvider returns each queued result in order.
type scriptedProvider struct {
	results []func() (*ChatResponse, error)
	calls   int
}

func (s *scriptedProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	i := s.calls
	s.calls++
	if i >= len(s.results) {
		i = len(s.results) - 1
	}
	return s.results[i]()
}

func ok() func() (*ChatResponse, error) {
	return func() (*ChatResponse, error) { return &ChatResponse{Content: "ok"}, nil }
}

func fail(pe *ProviderError) func() (*ChatResponse, error) {
	return func() (*ChatResponse, error) { return nil, pe }
}

func testOpts(slept *[]time.Duration) RetryOptions {
	return RetryOptions{
		MaxAttempts: 3,
		BaseDelay:   time.Second,
		MaxDelay:    8 * time.Second,
		sleep: func(ctx context.Context, d time.Duration) error {
			*slept = append(*slept, d)
			return nil
		},
		jitter: func() float64 { return 0 }, // deterministic: delay = base<<i / 2 * (1+0)
	}
}

func TestChatWithRetry_SucceedsFirstTry(t *testing.T) {
	var slept []time.Duration
	p := &scriptedProvider{results: []func() (*ChatResponse, error){ok()}}
	resp, retries, err := ChatWithRetry(context.Background(), p, ChatRequest{}, testOpts(&slept))
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Content)
	assert.Equal(t, 0, retries)
	assert.Empty(t, slept)
}

func TestChatWithRetry_RetriesRateLimitWithBackoffAndRetryAfter(t *testing.T) {
	var slept []time.Duration
	p := &scriptedProvider{results: []func() (*ChatResponse, error){
		fail(&ProviderError{Category: ErrCatRateLimit, StatusCode: 429, RetryAfter: 3 * time.Second, Message: "429"}),
		fail(&ProviderError{Category: ErrCatProvider, StatusCode: 503, Message: "503"}),
		ok(),
	}}
	resp, retries, err := ChatWithRetry(context.Background(), p, ChatRequest{}, testOpts(&slept))
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Content)
	assert.Equal(t, 2, retries)
	require.Len(t, slept, 2)
	// First delay: max(backoff 500ms, Retry-After 3s) = 3s. Second: base<<1 /2 = 1s.
	assert.Equal(t, 3*time.Second, slept[0])
	assert.Equal(t, time.Second, slept[1])
}

func TestChatWithRetry_DoesNotRetryPermanentErrors(t *testing.T) {
	var slept []time.Duration
	p := &scriptedProvider{results: []func() (*ChatResponse, error){
		fail(&ProviderError{Category: ErrCatAuthentication, StatusCode: 401, Message: "401"}),
	}}
	_, retries, err := ChatWithRetry(context.Background(), p, ChatRequest{}, testOpts(&slept))
	require.Error(t, err)
	assert.Equal(t, 0, retries)
	assert.Equal(t, 1, p.calls)
}

func TestChatWithRetry_BoundedAttempts(t *testing.T) {
	var slept []time.Duration
	p := &scriptedProvider{results: []func() (*ChatResponse, error){
		fail(&ProviderError{Category: ErrCatProvider, StatusCode: 500, Message: "500"}),
	}}
	_, retries, err := ChatWithRetry(context.Background(), p, ChatRequest{}, testOpts(&slept))
	require.Error(t, err)
	assert.Equal(t, 2, retries)
	assert.Equal(t, 3, p.calls, "MaxAttempts bounds total calls")
}

func TestChatWithRetry_StopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	p := &scriptedProvider{results: []func() (*ChatResponse, error){
		func() (*ChatResponse, error) {
			cancel() // provider call observes cancellation mid-flight
			return nil, &ProviderError{Category: ErrCatProvider, StatusCode: 500, Message: "500"}
		},
	}}
	var slept []time.Duration
	_, _, err := ChatWithRetry(ctx, p, ChatRequest{}, testOpts(&slept))
	require.Error(t, err)
	assert.Equal(t, 1, p.calls, "no retry after the request context is cancelled")
}

func TestChatWithRetry_CapsDelayAtMaxDelay(t *testing.T) {
	var slept []time.Duration
	p := &scriptedProvider{results: []func() (*ChatResponse, error){
		fail(&ProviderError{Category: ErrCatProvider, StatusCode: 500, Message: "500"}),
		ok(),
	}}
	opts := RetryOptions{
		MaxAttempts: 2,
		BaseDelay:   2 * time.Second, // 2s << 0 = 2s, exceeds the cap below
		MaxDelay:    1500 * time.Millisecond,
		sleep: func(ctx context.Context, d time.Duration) error {
			slept = append(slept, d)
			return nil
		},
		jitter: func() float64 { return 0 },
	}
	_, retries, err := ChatWithRetry(context.Background(), p, ChatRequest{}, opts)
	require.NoError(t, err)
	assert.Equal(t, 1, retries)
	require.Len(t, slept, 1)
	// 2s is capped to MaxDelay 1500ms, then halved (jitter=0) → 750ms.
	// If the cap branch were absent, this would be 2s/2 = 1s — so 750ms proves the cap fired.
	assert.Equal(t, 750*time.Millisecond, slept[0])
}
