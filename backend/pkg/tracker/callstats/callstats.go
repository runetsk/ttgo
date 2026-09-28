// Package callstats counts outbound-call events for one unit of work, such as a
// failure-analysis job, through its context. Clients report into whatever counter the
// context carries; a context without one makes reporting a no-op.
package callstats

import (
	"context"
	"sync"
	"sync/atomic"
)

type ctxKey struct{}

// Counter is safe for concurrent use: a job analyzes its groups in parallel.
type Counter struct {
	rateLimits   atomic.Int64
	callTimeouts atomic.Int64
	hedgesFired  atomic.Int64
	hedgesWon    atomic.Int64

	mu           sync.Mutex
	hedgePrompts []int
}

// WithCounter returns a context that carries a fresh counter, and the counter.
func WithCounter(ctx context.Context) (context.Context, *Counter) {
	c := &Counter{}
	return context.WithValue(ctx, ctxKey{}, c), c
}

func from(ctx context.Context) *Counter {
	if ctx == nil {
		return nil
	}
	c, _ := ctx.Value(ctxKey{}).(*Counter)
	return c
}

// RecordRateLimit counts one rate-limit response (HTTP 429) against the context's counter.
func RecordRateLimit(ctx context.Context) {
	if c := from(ctx); c != nil {
		c.rateLimits.Add(1)
	}
}

// RecordCallTimeout counts one LLM call cut by the per-call timeout (llm.WithCallTimeout).
func RecordCallTimeout(ctx context.Context) {
	if c := from(ctx); c != nil {
		c.callTimeouts.Add(1)
	}
}

// RecordHedgeFired counts one hedged second request sent (llm.WithHedge).
func RecordHedgeFired(ctx context.Context) {
	if c := from(ctx); c != nil {
		c.hedgesFired.Add(1)
	}
}

// RecordHedgeWon counts one hedged second request that answered first.
func RecordHedgeWon(ctx context.Context) {
	if c := from(ctx); c != nil {
		c.hedgesWon.Add(1)
	}
}

// RecordHedgePrompt records the winning request's prompt tokens for one fired hedge, so the
// hedge can be priced: the other request sent the same prompt.
func RecordHedgePrompt(ctx context.Context, tokens int) {
	if c := from(ctx); c != nil {
		c.mu.Lock()
		c.hedgePrompts = append(c.hedgePrompts, tokens)
		c.mu.Unlock()
	}
}

// RateLimitHits is the number of rate-limit responses recorded so far.
func (c *Counter) RateLimitHits() int {
	if c == nil {
		return 0
	}
	return int(c.rateLimits.Load())
}

// CallTimeouts is the number of per-call timeouts recorded so far.
func (c *Counter) CallTimeouts() int {
	if c == nil {
		return 0
	}
	return int(c.callTimeouts.Load())
}

// HedgesFired is the number of hedged second requests sent so far.
func (c *Counter) HedgesFired() int {
	if c == nil {
		return 0
	}
	return int(c.hedgesFired.Load())
}

// HedgesWon is the number of hedged second requests that answered first.
func (c *Counter) HedgesWon() int {
	if c == nil {
		return 0
	}
	return int(c.hedgesWon.Load())
}

// HedgePromptTokens returns a copy of the winner prompt tokens recorded per fired hedge.
func (c *Counter) HedgePromptTokens() []int {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.hedgePrompts) == 0 {
		return nil
	}
	return append([]int(nil), c.hedgePrompts...)
}
