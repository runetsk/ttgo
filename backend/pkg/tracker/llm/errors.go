package llm

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
)

// ErrorCategory is the normalized failure taxonomy for LLM calls
// (spec: "Retry and error handling").
type ErrorCategory string

const (
	ErrCatAuthentication ErrorCategory = "authentication"
	ErrCatAuthorization  ErrorCategory = "authorization"
	ErrCatRateLimit      ErrorCategory = "rate_limit"
	ErrCatTimeout        ErrorCategory = "timeout"
	ErrCatProvider       ErrorCategory = "provider"
	ErrCatSchema         ErrorCategory = "schema"
	ErrCatParse          ErrorCategory = "parse"
	ErrCatValidation     ErrorCategory = "validation"
	ErrCatCancelled      ErrorCategory = "cancellation"
	ErrCatInternal       ErrorCategory = "internal"
)

// ProviderError is a classified failure from a provider HTTP call.
// Message carries the full human-readable text (same format the clients
// previously returned as plain fmt.Errorf strings).
type ProviderError struct {
	StatusCode int
	Category   ErrorCategory
	RetryAfter time.Duration
	Message    string
}

func (e *ProviderError) Error() string { return e.Message }

// Retryable reports whether the failure is transient: rate limits and
// 5xx/network-level provider failures. Timeouts are NOT retryable — the
// provider timeout is respected, not doubled.
func (e *ProviderError) Retryable() bool {
	if e.Category == ErrCatRateLimit {
		return true
	}
	return e.Category == ErrCatProvider && (e.StatusCode == 0 || e.StatusCode >= 500)
}

// classifyStatus maps a provider HTTP status (and error body) to a category.
func classifyStatus(status int, body string) ErrorCategory {
	switch {
	case status == 401:
		return ErrCatAuthentication
	case status == 403:
		return ErrCatAuthorization
	case status == 408:
		return ErrCatTimeout
	case status == 429:
		return ErrCatRateLimit
	case status >= 500:
		return ErrCatProvider
	case status == 400:
		lower := strings.ToLower(body)
		if strings.Contains(lower, "response_format") || strings.Contains(lower, "json_schema") ||
			strings.Contains(lower, "schema") {
			return ErrCatSchema
		}
		return ErrCatProvider
	default:
		return ErrCatProvider
	}
}

// Classify normalizes any error from a Chat call into an ErrorCategory.
func Classify(err error) ErrorCategory {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrCatTimeout
	}
	if errors.Is(err, context.Canceled) {
		return ErrCatCancelled
	}
	var pe *ProviderError
	if errors.As(err, &pe) {
		return pe.Category
	}
	return ErrCatProvider
}

// parseRetryAfter parses the integer-seconds form of a Retry-After header,
// capped at 30s. The HTTP-date form returns 0 (backoff applies instead).
func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	secs, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || secs <= 0 {
		return 0
	}
	if secs > 30 {
		secs = 30
	}
	return time.Duration(secs) * time.Second
}
