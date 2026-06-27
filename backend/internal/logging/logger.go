package logging

import (
	"context"
	"log/slog"
)

type ctxKey struct{}

// WithRequestID returns a new context containing the given request ID.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// RequestID extracts the request ID from context, or returns "" if absent.
func RequestID(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKey{}).(string); ok {
		return v
	}
	return ""
}

// FromContext returns a logger pre-populated with the request ID from context.
func FromContext(ctx context.Context) *slog.Logger {
	l := slog.Default()
	if rid := RequestID(ctx); rid != "" {
		l = l.With("request_id", rid)
	}
	return l
}
