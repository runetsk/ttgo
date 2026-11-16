package backups

import (
	"errors"
	"sync"
	"time"
)

// ErrDrainTimeout is returned by BeginMaintenance when active leases did not drain
// within the allotted window. Admission is reopened and the live database is left
// untouched, so the caller returns 409 and the operator can retry.
var ErrDrainTimeout = errors.New("timed out draining active database operations")

// ErrMaintenanceInProgress is returned when BeginMaintenance is called while the
// gate is already closed (a concurrent restore).
var ErrMaintenanceInProgress = errors.New("a maintenance operation is already in progress")

// Gate is a process-wide admission gate for database work. Ordinary request paths
// take a short-lived lease through their database work via Admit/release; a
// restore closes admission and drains the outstanding leases (BeginMaintenance)
// before it may close and replace the SQLite file, guaranteeing no handler is
// mid-transaction when the connection is swapped. It coordinates one server
// process only — a second server or a CLI run directly against the same file
// during a restore is unsupported.
type Gate struct {
	mu      sync.Mutex
	active  int           // outstanding leases
	closed  bool          // admission closed (maintenance)
	drained chan struct{} // non-nil while a BeginMaintenance waits; closed when active hits 0
}

func NewGate() *Gate {
	return &Gate{}
}

// Admit grants a lease when admission is open. The returned release function is
// idempotent and must be called when the request's database work is done. When
// admission is closed it returns (nil, false) and the caller answers 503.
func (g *Gate) Admit() (func(), bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return nil, false
	}
	g.active++
	var once sync.Once
	return func() { once.Do(g.release) }, true
}

func (g *Gate) release() {
	g.mu.Lock()
	g.active--
	if g.active == 0 && g.drained != nil {
		close(g.drained)
		g.drained = nil
	}
	g.mu.Unlock()
}

// Closed reports whether admission is currently closed (maintenance in progress).
func (g *Gate) Closed() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.closed
}

// BeginMaintenance closes admission and waits up to drain for the outstanding
// leases to finish. On success the gate is closed and the caller may replace the
// database; the caller must pair it with EndMaintenance. If the leases do not
// drain in time it returns ErrDrainTimeout and reopens admission (nothing was
// touched). A second concurrent call returns ErrMaintenanceInProgress.
func (g *Gate) BeginMaintenance(drain time.Duration) error {
	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		return ErrMaintenanceInProgress
	}
	g.closed = true
	if g.active == 0 {
		g.mu.Unlock()
		return nil
	}
	ch := make(chan struct{})
	g.drained = ch
	g.mu.Unlock()

	select {
	case <-ch:
		return nil
	case <-time.After(drain):
		g.mu.Lock()
		defer g.mu.Unlock()
		g.drained = nil
		if g.active == 0 {
			// Drained in the moment the timer fired; proceed.
			return nil
		}
		g.closed = false // reopen admission; the restore is aborted
		return ErrDrainTimeout
	}
}

// EndMaintenance reopens admission after a restore completes (or fails and is
// rolled back).
func (g *Gate) EndMaintenance() {
	g.mu.Lock()
	g.closed = false
	g.mu.Unlock()
}
