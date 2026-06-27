// Package ratelimit is a small dependency-free token-bucket limiter with an
// HTTP middleware. Buckets are keyed by an arbitrary string (IP, token, user)
// so callers can rate-limit per-client. A janitor evicts idle buckets.
package ratelimit

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"ttgo/internal/api/httpx"
)

type bucket struct {
	tokens float64
	last   time.Time
}

// Limiter is a thread-safe per-key token-bucket rate limiter.
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64 // tokens added per second
	burst   float64 // maximum tokens
	now     func() time.Time
}

// New creates a limiter allowing ratePerSec sustained requests with the given burst.
func New(ratePerSec float64, burst int) *Limiter {
	if burst < 1 {
		burst = 1
	}
	return &Limiter{
		buckets: make(map[string]*bucket),
		rate:    ratePerSec,
		burst:   float64(burst),
		now:     time.Now,
	}
}

// Allow consumes one token for key and reports whether the request may proceed.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	b, ok := l.buckets[key]
	if !ok {
		l.buckets[key] = &bucket{tokens: l.burst - 1, last: now}
		return true
	}
	b.tokens = min(l.burst, b.tokens+now.Sub(b.last).Seconds()*l.rate)
	b.last = now
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// StartJanitor periodically removes buckets idle for longer than maxIdle.
// It runs until stop is closed.
func (l *Limiter) StartJanitor(interval, maxIdle time.Duration, stop <-chan struct{}) {
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				cutoff := l.now().Add(-maxIdle)
				l.mu.Lock()
				for k, b := range l.buckets {
					if b.last.Before(cutoff) {
						delete(l.buckets, k)
					}
				}
				l.mu.Unlock()
			}
		}
	}()
}

// Middleware wraps a handler, rejecting requests with HTTP 429 when the bucket
// for keyFn(r) is empty. An empty key bypasses limiting.
func (l *Limiter) Middleware(keyFn func(*http.Request) string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if key := keyFn(r); key != "" && !l.Allow(key) {
				w.Header().Set("Retry-After", "1")
				httpx.JSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded, slow down"})
				return
			}
			next(w, r)
		}
	}
}

// ClientIP extracts a best-effort client IP for keying (honours X-Forwarded-For
// first hop, falls back to RemoteAddr).
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// BearerOrIP keys by the Authorization bearer token when present, else by IP.
func BearerOrIP(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return "tok:" + strings.TrimPrefix(h, "Bearer ")
	}
	if c, err := r.Cookie("session_token"); err == nil && c.Value != "" {
		return "sess:" + c.Value
	}
	return "ip:" + ClientIP(r)
}
