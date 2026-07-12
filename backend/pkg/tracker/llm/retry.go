package llm

import (
	"context"
	"errors"
	"math/rand"
	"time"
)

// RetryOptions bounds ChatWithRetry. The zero value uses the defaults below.
// sleep and jitter are test seams; production uses time.Timer and math/rand.
type RetryOptions struct {
	MaxAttempts int           // total attempts including the first (default 3)
	BaseDelay   time.Duration // first backoff step (default 500ms)
	MaxDelay    time.Duration // backoff cap (default 8s)

	sleep  func(ctx context.Context, d time.Duration) error
	jitter func() float64 // uniform [0,1)
}

func (o *RetryOptions) withDefaults() RetryOptions {
	out := *o
	if out.MaxAttempts <= 0 {
		out.MaxAttempts = 3
	}
	if out.BaseDelay <= 0 {
		out.BaseDelay = 500 * time.Millisecond
	}
	if out.MaxDelay <= 0 {
		out.MaxDelay = 8 * time.Second
	}
	if out.sleep == nil {
		out.sleep = func(ctx context.Context, d time.Duration) error {
			t := time.NewTimer(d)
			defer t.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-t.C:
				return nil
			}
		}
	}
	if out.jitter == nil {
		out.jitter = rand.Float64
	}
	return out
}

// ChatWithRetry calls p.Chat, retrying transient failures (rate limits,
// 5xx/network) with bounded exponential backoff plus jitter. It honors a
// provider Retry-After value when larger than the computed backoff and stops
// immediately on context cancellation. Returns the response, the number of
// retries performed, and the last error.
func ChatWithRetry(ctx context.Context, p Provider, req ChatRequest, opts RetryOptions) (*ChatResponse, int, error) {
	o := opts.withDefaults()
	var lastErr error
	for attempt := 0; attempt < o.MaxAttempts; attempt++ {
		resp, err := p.Chat(ctx, req)
		if err == nil {
			return resp, attempt, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return nil, attempt, err
		}
		var pe *ProviderError
		if !errors.As(err, &pe) || !pe.Retryable() || attempt == o.MaxAttempts-1 {
			return nil, attempt, err
		}
		delay := o.BaseDelay << attempt
		if delay > o.MaxDelay {
			delay = o.MaxDelay
		}
		// Half fixed + half jittered avoids thundering herds while keeping a floor.
		delay = delay/2 + time.Duration(float64(delay/2)*o.jitter())
		if pe.RetryAfter > delay {
			delay = pe.RetryAfter
		}
		if err := o.sleep(ctx, delay); err != nil {
			return nil, attempt, lastErr
		}
	}
	return nil, o.MaxAttempts - 1, lastErr
}
