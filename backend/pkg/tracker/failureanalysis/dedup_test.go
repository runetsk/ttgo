package failureanalysis

import (
	"testing"
	"time"
	"ttgo/pkg/tracker/models"
)

func TestSignatureStable(t *testing.T) {
	a := Signature("assertion", "Expected 200, got 500 at /api/users/12345")
	b := Signature("assertion", "Expected 200, got 500 at /api/users/99999")
	if a != b {
		t.Errorf("signatures should be equal after numeric-id normalization: %q vs %q", a, b)
	}
}

func TestSignatureDiffersOnDifferentErrors(t *testing.T) {
	a := Signature("assertion", "Expected 200, got 500")
	b := Signature("assertion", "NullPointerException at AuthService")
	if a == b {
		t.Errorf("distinct errors should produce distinct signatures")
	}
}

func TestSignatureStripsTimestampsAndHex(t *testing.T) {
	a := Signature("error", "crash 2026-04-22T10:00:00Z at 0xdeadbeef in /foo/bar/auth.go:42")
	b := Signature("error", "crash 2020-01-01T00:00:00Z at 0x00000000 in /x/y/auth.go:42")
	if a != b {
		t.Errorf("should be equal after normalization: %q vs %q", a, b)
	}
}

func TestGroupFailuresPicksOldestRepresentative(t *testing.T) {
	t0 := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Minute)
	t2 := t0.Add(2 * time.Minute)
	results := []*models.RunResult{
		{ID: "mid", FailureType: "assertion", ErrorMessage: "Expected 200 got 500", StartTime: t1},
		{ID: "first", FailureType: "assertion", ErrorMessage: "Expected 200 got 500", StartTime: t0},
		{ID: "late", FailureType: "assertion", ErrorMessage: "Expected 200 got 500", StartTime: t2},
		{ID: "other", FailureType: "crash", ErrorMessage: "NPE", StartTime: t0},
	}
	groups := GroupFailures(results)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	// Group with 3 members: representative must be "first" (oldest StartTime).
	var big *FailureGroup
	for _, g := range groups {
		if len(g.Members) == 3 {
			big = g
			break
		}
	}
	if big == nil {
		t.Fatal("expected a group of 3 members")
	}
	if big.Representative.ID != "first" {
		t.Errorf("representative should be oldest (first), got %s", big.Representative.ID)
	}
}
