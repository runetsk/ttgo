package typesafe

import (
	"fmt"
	"time"
)

// Category is the normalized failure taxonomy for TypeSafe calls.
type Category string

const (
	CategoryAuth       Category = "auth"       // 401
	CategoryValidation Category = "validation" // 422
	CategoryRateLimit  Category = "rate_limit" // 429
	CategoryOverloaded Category = "overloaded" // 529
	CategoryTimeout    Category = "timeout"    // per-attempt deadline exceeded
	CategoryNetwork    Category = "network"    // dial/TLS/reset
	CategoryParse      Category = "parse"      // body not decodable or answers invalid
	CategoryInternal   Category = "internal"   // any other status (incl. 5xx)
	// CategoryConfiguration: TypeSafe is selected but not usable as configured. TTGO raises it
	// for a missing or undecryptable key; HTTP 400 and 404 (unknown model id, wrong base URL)
	// map to it too. Never retried: only a settings change helps.
	CategoryConfiguration Category = "configuration"
)

// Error is a classified TypeSafe failure. Message never contains the request body or key.
// Oversized marks a 422 whose body names the context/token limit: callers may shrink and retry.
type Error struct {
	Status     int
	Category   Category
	Message    string
	RetryAfter time.Duration
	Oversized  bool
}

func (e *Error) Error() string {
	if e.Status > 0 {
		return fmt.Sprintf("typesafe: %s (HTTP %d): %s", e.Category, e.Status, e.Message)
	}
	return fmt.Sprintf("typesafe: %s: %s", e.Category, e.Message)
}

// Retryable: rate limit, overloaded, 5xx and network failures. Never auth, validation,
// configuration, parse, timeout (the timeout is respected, not doubled) or other 4xx.
func (e *Error) Retryable() bool {
	switch e.Category {
	case CategoryRateLimit, CategoryOverloaded, CategoryNetwork:
		return true
	case CategoryInternal:
		return e.Status >= 500
	}
	return false
}
