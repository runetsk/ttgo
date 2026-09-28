package failureanalysis

import (
	"time"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"
)

// MinGroupDeadline is the floor of a group's deadline: the fixed bound used before the LLM
// call timeout existed.
const MinGroupDeadline = 5 * time.Minute

// LLMMaxRetryAfter caps how long a failure-analysis LLM retry waits on a provider's
// Retry-After, so each backoff stays inside GroupDeadlineFor's budget.
const LLMMaxRetryAfter = 30 * time.Second

// GroupDeadlineFor bounds one group's whole analysis from the job's timeouts (spec §B): the
// TypeSafe decision (3 attempts + 2 backoffs of at most 30 s), one LLM stage (the first call and
// the JSON repair, each with its transient retry: 4 attempts + 2 backoffs of at most
// LLMMaxRetryAfter), and 30 s for everything else — never below MinGroupDeadline.
// Defaults (TypeSafe 30 s, LLM 45 s): 90 + 60 + 180 + 60 + 30 = 420 s.
func GroupDeadlineFor(tsTimeout, llmTimeout time.Duration) time.Duration {
	const (
		typeSafeAttempts = 3
		llmAttempts      = 2 * TransportAttempts
		backoff          = 30 * time.Second
		slack            = 30 * time.Second
	)
	d := typeSafeAttempts*tsTimeout + 2*backoff + llmAttempts*llmTimeout + 2*LLMMaxRetryAfter + slack
	if d < MinGroupDeadline {
		return MinGroupDeadline
	}
	return d
}

// LLMCallTimeoutFor is the stored llm_call_timeout_seconds as a duration; a value outside
// 10–120 s (a row from before the setting reads 0) falls back to the 45 s default.
func LLMCallTimeoutFor(seconds int) time.Duration {
	if seconds < models.MinLLMCallTimeoutSeconds || seconds > models.MaxLLMCallTimeoutSeconds {
		seconds = models.DefaultLLMCallTimeoutSeconds
	}
	return time.Duration(seconds) * time.Second
}

// HedgeAfterFor is the stored hedge_after_seconds as a duration, 0 (off) unless it is at least
// 3 s and shorter than the call timeout.
func HedgeAfterFor(seconds int, callTimeout time.Duration) time.Duration {
	d := time.Duration(seconds) * time.Second
	if seconds < models.MinHedgeAfterSeconds || d >= callTimeout {
		return 0
	}
	return d
}

// llmRetryOptions bounds each failure-analysis LLM call: TransportAttempts, and a Retry-After
// wait capped at LLMMaxRetryAfter.
func llmRetryOptions() llm.RetryOptions {
	return llm.RetryOptions{MaxAttempts: TransportAttempts, MaxRetryAfter: LLMMaxRetryAfter}
}
