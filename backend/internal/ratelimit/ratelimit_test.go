package ratelimit

import (
	"testing"
	"time"
)

func TestAllowBurstThenDeny(t *testing.T) {
	l := New(1, 3) // 1/sec, burst 3
	base := time.Unix(0, 0)
	l.now = func() time.Time { return base }

	for i := 0; i < 3; i++ {
		if !l.Allow("k") {
			t.Fatalf("request %d should be allowed within burst", i+1)
		}
	}
	if l.Allow("k") {
		t.Error("4th request should be denied (burst exhausted)")
	}
}

func TestAllowRefillsOverTime(t *testing.T) {
	l := New(2, 1) // 2/sec, burst 1
	now := time.Unix(0, 0)
	l.now = func() time.Time { return now }

	if !l.Allow("k") {
		t.Fatal("first request should be allowed")
	}
	if l.Allow("k") {
		t.Fatal("second immediate request should be denied")
	}
	now = now.Add(600 * time.Millisecond) // 0.6s * 2/s = 1.2 tokens
	if !l.Allow("k") {
		t.Error("request after refill window should be allowed")
	}
}

func TestKeysAreIndependent(t *testing.T) {
	l := New(1, 1)
	now := time.Unix(0, 0)
	l.now = func() time.Time { return now }
	if !l.Allow("a") || !l.Allow("b") {
		t.Error("distinct keys must not share a bucket")
	}
	if l.Allow("a") {
		t.Error("key a should now be exhausted")
	}
}
