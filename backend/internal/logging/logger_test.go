package logging_test

import (
	"context"
	"testing"
	"ttgo/internal/logging"

	"github.com/stretchr/testify/assert"
)

func TestRequestID_Empty(t *testing.T) {
	assert.Equal(t, "", logging.RequestID(context.Background()))
}

func TestWithRequestID_RoundTrip(t *testing.T) {
	ctx := logging.WithRequestID(context.Background(), "abc-123")
	assert.Equal(t, "abc-123", logging.RequestID(ctx))
}

func TestFromContext_NoID(t *testing.T) {
	// Should return the default logger without panicking.
	logger := logging.FromContext(context.Background())
	assert.NotNil(t, logger)
}

func TestFromContext_WithID(t *testing.T) {
	ctx := logging.WithRequestID(context.Background(), "req-42")
	logger := logging.FromContext(ctx)
	assert.NotNil(t, logger)
}
