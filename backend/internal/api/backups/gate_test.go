package backups

import (
	"errors"
	"testing"
	"time"
)

func TestGate_AdmitsWhenOpen(t *testing.T) {
	g := NewGate()
	rel, ok := g.Admit()
	if !ok {
		t.Fatal("an open gate must admit")
	}
	if g.Closed() {
		t.Fatal("gate should not report closed")
	}
	rel()
}

func TestGate_RejectsWhenClosed(t *testing.T) {
	g := NewGate()
	if err := g.BeginMaintenance(time.Second); err != nil {
		t.Fatalf("BeginMaintenance: %v", err)
	}
	if !g.Closed() {
		t.Fatal("gate should report closed during maintenance")
	}
	if _, ok := g.Admit(); ok {
		t.Fatal("a closed gate must reject admission")
	}
	g.EndMaintenance()
	if g.Closed() {
		t.Fatal("gate should reopen after EndMaintenance")
	}
	if _, ok := g.Admit(); !ok {
		t.Fatal("a reopened gate must admit")
	}
}

func TestGate_DrainsActiveLeases(t *testing.T) {
	g := NewGate()
	rel, _ := g.Admit()
	done := make(chan error, 1)
	go func() { done <- g.BeginMaintenance(2 * time.Second) }()

	// Must not complete while the lease is still held.
	select {
	case <-done:
		t.Fatal("BeginMaintenance returned before the active lease was released")
	case <-time.After(100 * time.Millisecond):
	}

	rel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("drain should succeed once the lease is released: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("BeginMaintenance did not return after the lease was released")
	}
}

func TestGate_DrainTimeoutReopensAdmission(t *testing.T) {
	g := NewGate()
	rel, _ := g.Admit() // deliberately held past the drain window
	defer rel()

	err := g.BeginMaintenance(50 * time.Millisecond)
	if !errors.Is(err, ErrDrainTimeout) {
		t.Fatalf("expected ErrDrainTimeout, got %v", err)
	}
	if g.Closed() {
		t.Fatal("a drain timeout must reopen admission (live DB untouched)")
	}
	if _, ok := g.Admit(); !ok {
		t.Fatal("admission must be open again after a drain timeout")
	}
}

func TestGate_RejectsConcurrentMaintenance(t *testing.T) {
	g := NewGate()
	if err := g.BeginMaintenance(time.Second); err != nil {
		t.Fatal(err)
	}
	defer g.EndMaintenance()
	if err := g.BeginMaintenance(time.Second); !errors.Is(err, ErrMaintenanceInProgress) {
		t.Fatalf("expected ErrMaintenanceInProgress, got %v", err)
	}
}

func TestGate_ReleaseIsIdempotent(t *testing.T) {
	g := NewGate()
	rel, _ := g.Admit()
	rel()
	rel() // a second release must not double-decrement
	if err := g.BeginMaintenance(100 * time.Millisecond); err != nil {
		t.Fatalf("with no active leases maintenance should begin at once: %v", err)
	}
}
