package runs

import (
	apiws "ttgo/internal/api/websocket"
	"ttgo/pkg/tracker/models"
)

// resultDelta is the wire payload for result-level events (result_updated,
// result_bulk_updated, result_retried, result_deleted). It carries the
// affected rows and the parent run's summary (status + latest-attempt
// counters, no run_results) so fan-out bytes stay O(changed), not O(run) —
// full-run payloads overflowed per-client egress buffers at 1000 subscribers
// and the per-post GetTestRun re-fetch throttled ingestion (S4 campaign).
type resultDelta struct {
	RunID            string              `json:"run_id"`
	Run              *models.TestRun     `json:"run"`
	Results          []*models.RunResult `json:"results,omitempty"`
	ResultIDs        []string            `json:"result_ids,omitempty"`
	Patch            map[string]any      `json:"patch,omitempty"`
	DeletedResultIDs []string            `json:"deleted_result_ids,omitempty"`
}

// broadcastResultDelta emits a result-level event with a delta payload.
// Exactly one of the three forms is sent, chosen by the arguments:
//   - resultIDs with nil patch → full rows for those IDs ("results")
//   - resultIDs with a patch   → ids + the field patch ("result_ids"/"patch")
//   - deletedIDs               → removed ids ("deleted_result_ids")
//
// Best-effort like the other broadcasts: fetch errors skip the event.
func (h *Handler) broadcastResultDelta(eventType, runID string, resultIDs []string, patch map[string]any, deletedIDs []string) {
	if h.hub == nil {
		return
	}
	summary, err := h.store.GetTestRunSummary(runID)
	if err != nil || summary == nil {
		return
	}
	payload := &resultDelta{RunID: runID, Run: summary, DeletedResultIDs: deletedIDs}
	switch {
	case patch != nil:
		payload.ResultIDs = resultIDs
		payload.Patch = patch
	case len(resultIDs) > 0:
		rows, err := h.store.GetRunResultsByIDs(runID, resultIDs)
		if err != nil || len(rows) == 0 {
			return
		}
		payload.Results = rows
	}
	h.hub.Broadcast(apiws.NewEvent(eventType, "run:"+runID, payload))
}
