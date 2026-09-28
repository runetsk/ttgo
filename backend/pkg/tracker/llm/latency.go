package llm

import (
	"context"
	"errors"
	"fmt"
	"time"

	"ttgo/pkg/tracker/callstats"
)

// WithCallTimeout bounds every Chat call of p to d, as a child deadline of the caller's
// context. A call cut by that deadline while the caller's context is still live fails with a
// retryable timeout (ProviderError.CallTimeout), so ChatWithRetry sends it once more; the
// caller's own deadline and a provider HTTP timeout keep CallTimeout false and are not retried.
// Each call timeout is counted on the context's callstats counter here, where it is raised.
// d <= 0 (or a nil p) returns p unchanged.
func WithCallTimeout(p Provider, d time.Duration) Provider {
	if p == nil || d <= 0 {
		return p
	}
	return &callTimeoutProvider{p: p, d: d}
}

type callTimeoutProvider struct {
	p Provider
	d time.Duration
}

func (c *callTimeoutProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	cctx, cancel := context.WithTimeout(ctx, c.d)
	defer cancel()
	resp, err := c.p.Chat(cctx, req)
	if err == nil {
		return resp, nil
	}
	if ctx.Err() == nil && errors.Is(cctx.Err(), context.DeadlineExceeded) {
		callstats.RecordCallTimeout(ctx)
		return nil, &ProviderError{Category: ErrCatTimeout, CallTimeout: true,
			Message: fmt.Sprintf("LLM call timed out after %s", c.d), Err: err}
	}
	return nil, err
}

// WithHedge sends an identical second request when p has not answered within after. The first
// success wins and the other request is cancelled through its own context; the call fails only
// when both fail, with the error of the request that failed last. A request that fails before
// the hedge is sent is returned at once (ChatWithRetry decides about retrying). Each request
// runs in its own goroutine with its own cancellable context and a buffered result slot, so
// both always finish once Chat returns (providers honour their context).
//
// Counting (callstats): a sent hedge is RecordHedgeFired; a win by the hedge RecordHedgeWon; a
// success after a hedge was sent records the winner's prompt tokens (RecordHedgePrompt) so the
// hedge can be priced. The error Chat returns is counted by its caller (ChatWithRetry counts
// rate limits); the error of a request discarded here is classified here, so every 429 counts
// exactly once. Call timeouts are counted where they are raised (WithCallTimeout), never here.
// after <= 0 (or a nil p) returns p unchanged.
func WithHedge(p Provider, after time.Duration) Provider {
	if p == nil || after <= 0 {
		return p
	}
	return &hedgeProvider{p: p, after: after}
}

type hedgeProvider struct {
	p     Provider
	after time.Duration
}

type hedgeResult struct {
	resp  *ChatResponse
	err   error
	hedge bool
}

func (h *hedgeProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	results := make(chan hedgeResult, 2) // one slot per request: no sender ever blocks
	var cancels []context.CancelFunc
	defer func() {
		for _, cancel := range cancels {
			cancel() // the loser, if still running, stops now
		}
	}()
	launch := func(hedge bool) {
		bctx, cancel := context.WithCancel(ctx)
		cancels = append(cancels, cancel)
		go func() {
			resp, err := h.p.Chat(bctx, req)
			results <- hedgeResult{resp: resp, err: err, hedge: hedge}
		}()
	}

	launch(false)
	timer := time.NewTimer(h.after)
	defer timer.Stop()
	running, fired := 1, false
	for {
		select {
		case <-timer.C:
			if !fired && running == 1 {
				fired = true
				running++
				callstats.RecordHedgeFired(ctx)
				launch(true)
			}
		case r := <-results:
			running--
			if r.err == nil {
				if fired {
					if r.hedge {
						callstats.RecordHedgeWon(ctx)
					}
					prompt := 0
					if r.resp != nil && r.resp.Usage != nil {
						prompt = r.resp.Usage.PromptTokens
					}
					callstats.RecordHedgePrompt(ctx, prompt)
				}
				return r.resp, nil
			}
			if running == 0 {
				return nil, r.err // the caller classifies the error it receives
			}
			// The other request is still running: this error is discarded, so count it here.
			if Classify(r.err) == ErrCatRateLimit {
				callstats.RecordRateLimit(ctx)
			}
		}
	}
}
