package ai

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"ttgo/pkg/tracker/llm"
)

// TestHTTPStatusForCategory locks the category→status mapping used for both
// first-time failures (writeGenerationFailure) and idempotency replays
// (writeExistingRun). Schema failures mean the provider rejected our
// structured-output request — a gateway/capability fault (502), not client
// input (422).
func TestHTTPStatusForCategory(t *testing.T) {
	cases := map[llm.ErrorCategory]int{
		llm.ErrCatParse:          http.StatusUnprocessableEntity, // 422
		llm.ErrCatValidation:     http.StatusUnprocessableEntity, // 422
		llm.ErrCatTimeout:        http.StatusGatewayTimeout,      // 504
		llm.ErrCatRateLimit:      http.StatusTooManyRequests,     // 429
		llm.ErrCatInternal:       http.StatusInternalServerError, // 500
		llm.ErrCatSchema:         http.StatusBadGateway,          // 502
		llm.ErrCatProvider:       http.StatusBadGateway,          // 502
		llm.ErrCatAuthentication: http.StatusBadGateway,          // 502
		llm.ErrCatAuthorization:  http.StatusBadGateway,          // 502
	}
	for cat, want := range cases {
		assert.Equal(t, want, httpStatusForCategory(cat), string(cat))
	}
}
