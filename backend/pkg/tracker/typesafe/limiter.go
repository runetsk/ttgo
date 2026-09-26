package typesafe

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// EnvRPM names the operator-set environment variable that caps outbound TypeSafe
	// requests per minute across the whole process; "0" disables the cap.
	EnvRPM = "TYPESAFE_RPM"
	// DefaultRPM is the cap when EnvRPM is unset or unreadable.
	DefaultRPM = 1000
)

// Limiter paces outbound TypeSafe requests. Wait blocks until one request may be sent, or
// returns the context's error when ctx ends first.
type Limiter interface {
	Wait(ctx context.Context) error
}

// NewRPMLimiter returns a token bucket allowing rpm requests per minute with a burst of one
// second's worth (at least 1). rpm <= 0 returns nil: no limit.
func NewRPMLimiter(rpm int) Limiter {
	if rpm <= 0 {
		return nil
	}
	return newRPMLimiter(rpm, time.Now, sleepCtx)
}

// LimiterFromEnv builds the process-wide limiter from TYPESAFE_RPM (default 1000/min, 0 = off).
func LimiterFromEnv() Limiter {
	v := strings.TrimSpace(os.Getenv(EnvRPM))
	if v == "" {
		return NewRPMLimiter(DefaultRPM)
	}
	rpm, err := strconv.Atoi(v)
	if err != nil || rpm < 0 {
		slog.Warn("typesafe: ignoring an invalid "+EnvRPM+", using the default", "value", v, "default", DefaultRPM)
		return NewRPMLimiter(DefaultRPM)
	}
	return NewRPMLimiter(rpm)
}

type rpmLimiter struct {
	mu     sync.Mutex
	rate   float64 // tokens added per second
	burst  float64
	tokens float64
	last   time.Time
	now    func() time.Time
	sleep  func(ctx context.Context, d time.Duration) error
}

func newRPMLimiter(rpm int, now func() time.Time, sleep func(context.Context, time.Duration) error) *rpmLimiter {
	rate := float64(rpm) / 60
	burst := rate
	if burst < 1 {
		burst = 1
	}
	return &rpmLimiter{rate: rate, burst: burst, tokens: burst, last: now(), now: now, sleep: sleep}
}

// Wait reserves a token and sleeps until it is due. The balance may go negative, which queues
// later callers behind this one in arrival order. A cancelled wait returns its token.
func (l *rpmLimiter) Wait(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	l.mu.Lock()
	now := l.now()
	l.tokens = min(l.burst, l.tokens+now.Sub(l.last).Seconds()*l.rate)
	l.last = now
	l.tokens--
	var wait time.Duration
	if l.tokens < 0 {
		wait = time.Duration(-l.tokens / l.rate * float64(time.Second))
	}
	l.mu.Unlock()
	if wait <= 0 {
		return nil
	}
	if err := l.sleep(ctx, wait); err != nil {
		l.mu.Lock()
		l.tokens++
		l.mu.Unlock()
		return err
	}
	return nil
}

// sleepCtx waits for d or until ctx ends, whichever comes first.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
