// Package callstats counts outbound-call events for one unit of work, such as a
// failure-analysis job, through its context. Clients report into whatever counter the
// context carries; a context without one makes reporting a no-op.
package callstats

import (
	"context"
	"sync/atomic"
)

type ctxKey struct{}

// Counter is safe for concurrent use: a job analyzes its groups in parallel.
type Counter struct {
	rateLimits atomic.Int64
}

// WithCounter returns a context that carries a fresh counter, and the counter.
func WithCounter(ctx context.Context) (context.Context, *Counter) {
	c := &Counter{}
	return context.WithValue(ctx, ctxKey{}, c), c
}

// RecordRateLimit counts one rate-limit response (HTTP 429) against the context's counter.
func RecordRateLimit(ctx context.Context) {
	if ctx == nil {
		return
	}
	if c, ok := ctx.Value(ctxKey{}).(*Counter); ok {
		c.rateLimits.Add(1)
	}
}

// RateLimitHits is the number of rate-limit responses recorded so far.
func (c *Counter) RateLimitHits() int {
	if c == nil {
		return 0
	}
	return int(c.rateLimits.Load())
}
