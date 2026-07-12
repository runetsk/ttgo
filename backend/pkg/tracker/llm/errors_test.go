package llm

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestClassifyStatus(t *testing.T) {
	cases := []struct {
		status int
		body   string
		want   ErrorCategory
	}{
		{401, "bad key", ErrCatAuthentication},
		{403, "forbidden", ErrCatAuthorization},
		{408, "timeout", ErrCatTimeout},
		{429, "slow down", ErrCatRateLimit},
		{500, "boom", ErrCatProvider},
		{503, "unavailable", ErrCatProvider},
		{400, "invalid response_format json_schema", ErrCatSchema},
		{400, "some other bad request", ErrCatProvider},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, classifyStatus(c.status, c.body), "status %d body %q", c.status, c.body)
	}
}

func TestProviderErrorRetryable(t *testing.T) {
	assert.True(t, (&ProviderError{Category: ErrCatRateLimit, StatusCode: 429}).Retryable())
	assert.True(t, (&ProviderError{Category: ErrCatProvider, StatusCode: 503}).Retryable())
	assert.True(t, (&ProviderError{Category: ErrCatProvider, StatusCode: 0}).Retryable(), "network-level failures are retryable")
	assert.False(t, (&ProviderError{Category: ErrCatAuthentication, StatusCode: 401}).Retryable())
	assert.False(t, (&ProviderError{Category: ErrCatSchema, StatusCode: 400}).Retryable())
	assert.False(t, (&ProviderError{Category: ErrCatTimeout}).Retryable(), "respect the provider timeout — do not double it")
}

func TestClassify(t *testing.T) {
	assert.Equal(t, ErrCatTimeout, Classify(context.DeadlineExceeded))
	assert.Equal(t, ErrCatTimeout, Classify(fmt.Errorf("wrapped: %w", context.DeadlineExceeded)))
	assert.Equal(t, ErrCatCancelled, Classify(context.Canceled))
	assert.Equal(t, ErrCatRateLimit, Classify(fmt.Errorf("call failed: %w", &ProviderError{Category: ErrCatRateLimit})))
	assert.Equal(t, ErrCatProvider, Classify(errors.New("plain error")))
	assert.Equal(t, ErrorCategory(""), Classify(nil))
}

func TestParseRetryAfter(t *testing.T) {
	assert.Equal(t, 5*time.Second, parseRetryAfter("5"))
	assert.Equal(t, 30*time.Second, parseRetryAfter("999"), "capped at 30s")
	assert.Equal(t, time.Duration(0), parseRetryAfter(""))
	assert.Equal(t, time.Duration(0), parseRetryAfter("Wed, 21 Oct 2026 07:28:00 GMT"), "HTTP-date form unsupported -> 0")
}
