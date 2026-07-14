package ai

import (
	"context"
	"sync"
)

// inflightRegistry tracks the CancelFunc of every generation run currently
// executing inside a request handler, so POST /ai-generations/{id}/cancel can
// abort it from another session. In-process only — after a crash/restart,
// stale `running` rows are stamped cancelled lazily by the cancel endpoint.
type inflightRegistry struct {
	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

func newInflightRegistry() *inflightRegistry {
	return &inflightRegistry{cancels: map[string]context.CancelFunc{}}
}

func (r *inflightRegistry) register(runID string, cancel context.CancelFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cancels[runID] = cancel
}

func (r *inflightRegistry) unregister(runID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.cancels, runID)
}

// cancel fires and removes the run's CancelFunc; false when not in flight.
func (r *inflightRegistry) cancel(runID string) bool {
	r.mu.Lock()
	cancel, ok := r.cancels[runID]
	delete(r.cancels, runID)
	r.mu.Unlock()
	if ok {
		cancel()
	}
	return ok
}
