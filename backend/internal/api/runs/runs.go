package runs

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"ttgo/internal/api/authctx"
	"ttgo/internal/api/httpx"
	apiws "ttgo/internal/api/websocket"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"

	"gorm.io/gorm"
)

// CreateTestRun godoc
//
// @Summary      Create a test run
// @Description  Creates a new test run, either empty, seeded from a category snapshot, or seeded from an explicit list of test case IDs (category_id and test_case_ids are mutually exclusive).
// @Tags         runs
// @Accept       json
// @Produce      json
// @Param        body  body      object{category_id=string,name=string,run_folder_id=string,test_case_ids=[]string}  true  "Run to create"
// @Security     BearerAuth
// @Success      201  {object}  models.TestRun
// @Failure      400  {object}  object{error=string}
// @Failure      500  {object}  object{error=string}
// @Router       /runs [post]
func (h *Handler) CreateTestRun(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CategoryID  *string  `json:"category_id"`
		Name        string   `json:"name"`
		RunFolderID *string  `json:"run_folder_id"`
		TestCaseIDs []string `json:"test_case_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	if req.CategoryID != nil && *req.CategoryID == "" {
		req.CategoryID = nil
	}

	if len(req.TestCaseIDs) > 0 && req.CategoryID != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "provide either category_id or test_case_ids, not both"})
		return
	}
	if len(req.TestCaseIDs) > httpx.MaxBulkIDs {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "too many test_case_ids (max 500 per request)"})
		return
	}

	if req.RunFolderID != nil && *req.RunFolderID != "" {
		folder, err := h.store.GetRunFolder(*req.RunFolderID)
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, err)
			return
		}
		if folder == nil {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "run_folder_id references a non-existent folder"})
			return
		}
	}

	run := &models.TestRun{
		CategoryID:  req.CategoryID,
		Name:        req.Name,
		RunFolderID: req.RunFolderID,
	}
	if err := h.store.CreateTestRunWithCases(run, req.TestCaseIDs); err != nil {
		if errors.Is(err, store.ErrUnknownTestCases) {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "one or more test_case_ids do not exist"})
			return
		}
		slog.ErrorContext(r.Context(), "failed to create test run", "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	if h.hub != nil {
		h.hub.Broadcast(apiws.NewEvent(apiws.EventRunCreated, "runs:*", run))
	}

	httpx.JSON(w, http.StatusCreated, run)
}

// GetTestRuns returns a paginated, filtered list of test runs.
//
// @Summary      List test runs
// @Description  Returns test runs with optional filtering by category list, status, date ranges, and folder.
// @Tags         runs
// @Accept       json
// @Produce      json
// @Param        category_ids  query     string  false  "Comma-separated list of category IDs to filter by (OR logic)"
// @Param        category_id   query     string  false  "Single category ID (backward-compatible alias; overridden by category_ids)"
// @Param        status        query     string  false  "Filter by run status (e.g. PENDING, PASS, FAIL)"
// @Param        created_from  query     string  false  "Include runs created on or after this date (YYYY-MM-DD, UTC)"
// @Param        created_to    query     string  false  "Include runs created on or before this date inclusive (YYYY-MM-DD, UTC)"
// @Param        updated_from  query     string  false  "Include runs updated on or after this date (YYYY-MM-DD, UTC)"
// @Param        updated_to    query     string  false  "Include runs updated on or before this date inclusive (YYYY-MM-DD, UTC)"
// @Param        sort_by       query     string  false  "Sort column: name, status, created_at, updated_at"
// @Param        order         query     string  false  "Sort direction: ASC or DESC (default DESC)"
// @Param        limit         query     int     false  "Page size (default 50)"
// @Param        offset        query     int     false  "Page offset"
// @Param        run_folder_id query     string  false  "Filter by folder ID; use 'uncategorised' for runs with no folder"
// @Success      200  {object}  object{runs=[]models.TestRun,total=int}
// @Failure      500  {object}  object{error=string}
// @Router       /runs [get]
func (h *Handler) GetTestRuns(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	status := q.Get("status")
	sortBy := q.Get("sort_by")
	sortDir := q.Get("order")
	folderID := q.Get("run_folder_id")

	assigneeID := q.Get("assignee_id")
	if assigneeID == "me" {
		if u := authctx.UserFromRequest(r); u != nil {
			assigneeID = u.ID
		}
		// Token-authed requests have no session user: "me" stays literal and matches no run.
	}

	var categoryIDs []string
	if v := q.Get("category_ids"); v != "" {
		categoryIDs = strings.Split(v, ",")
	} else if v := q.Get("category_id"); v != "" { // backward-compatible single value
		categoryIDs = []string{v}
	}

	limit := 50
	if l, err := strconv.Atoi(q.Get("limit")); err == nil {
		limit = l
	}
	offset := 0
	if o, err := strconv.Atoi(q.Get("offset")); err == nil {
		offset = o
	}

	runs, total, err := h.store.GetTestRuns(store.RunFilter{
		CategoryIDs: categoryIDs,
		Status:      status,
		CreatedFrom: q.Get("created_from"),
		CreatedTo:   q.Get("created_to"),
		UpdatedFrom: q.Get("updated_from"),
		UpdatedTo:   q.Get("updated_to"),
		SortBy:      sortBy,
		SortDir:     sortDir,
		Limit:       limit,
		Offset:      offset,
		FolderID:    folderID,
		AssigneeID:  assigneeID,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get test runs", "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	if len(runs) > 0 {
		runIDs := make([]string, len(runs))
		for i, run := range runs {
			runIDs[i] = run.ID
		}
		if counts, err := h.store.CountCommentsByTargets("run", runIDs); err == nil {
			for i := range runs {
				runs[i].CommentCount = counts[runs[i].ID]
			}
		}
		if openCounts, closedCounts, err := h.store.CountDefectLinksByRuns(runIDs); err == nil {
			for i := range runs {
				runs[i].OpenDefectLinkCount = openCounts[runs[i].ID]
				runs[i].ClosedDefectLinkCount = closedCounts[runs[i].ID]
			}
		}
	}

	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"runs":  runs,
		"total": total,
	})
}

// GetTestRun godoc
//
// @Summary      Get a test run
// @Description  Returns a single test run by ID.
// @Tags         runs
// @Produce      json
// @Param        id  path      string  true  "Test run ID"
// @Security     BearerAuth
// @Success      200  {object}  models.TestRun
// @Failure      404  {object}  object{error=string}
// @Failure      500  {object}  object{error=string}
// @Router       /runs/{id} [get]
func (h *Handler) GetTestRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	run, err := h.store.GetTestRun(id)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get test run", "run_id", id, "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if run == nil {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "test run not found"})
		return
	}

	httpx.JSON(w, http.StatusOK, run)
}

// UpdateRunResult godoc
//
// @Summary      Update a run result
// @Description  Applies a partial update to a single result within a test run (status, defect classification, failure details, artifacts, environment, or steps).
// @Tags         runs
// @Accept       json
// @Produce      json
// @Param        id         path  string                         true  "Test run ID"
// @Param        result_id  path  string                         true  "Run result ID"
// @Param        body       body  models.UpdateRunResultRequest  true  "Fields to update"
// @Security     BearerAuth
// @Success      200  {object}  object{status=string}
// @Failure      400  {object}  object{error=string}
// @Failure      500  {object}  object{error=string}
// @Router       /runs/{id}/results/{result_id} [put]
func (h *Handler) UpdateRunResult(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")
	resultID := r.PathValue("result_id")

	var req models.UpdateRunResultRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	updateMap := map[string]interface{}{}

	if req.Status != nil {
		if !models.IsValidExecutionStatus(*req.Status) {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid status"})
			return
		}
		updateMap["status"] = *req.Status
	}
	if req.DefectType != nil {
		// Validated against the same canonical set the bulk endpoint uses. Without this the two
		// endpoints would disagree about what is acceptable: bulk would 400 on garbage while this
		// path silently persisted it into a column every calibration query then groups by.
		if !models.IsValidDefectType(*req.DefectType) {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid defect_type"})
			return
		}
	}

	// defect_type is decided from the status the row will HOLD, never from what the caller asked
	// for on its own. Resolving it here rather than inside the `req.Status != nil` block above is
	// what stops {status:"PASS", defect_type:"product_bug"} from persisting a defect type on a
	// passing row: the old shape wrote "" for the non-failure and then let the caller's value
	// overwrite it two lines later, so the single-result path contradicted both the bulk endpoint
	// and the invariant documented on models.RunResult.DefectType.
	//
	// The lookup costs nothing extra: it is skipped entirely when the caller supplied a status, and
	// otherwise IS the read snapshotAISuggestion would have made — the resolved status is handed
	// down to it rather than re-derived, so this path performs exactly one.
	//
	// guardedTriage marks the status-omitted triage write, which must carry the failure-status test
	// INSIDE its UPDATE. See the store call at the bottom of this function.
	guardedTriage := false
	if req.Status != nil || req.DefectType != nil {
		effStatus, known, err := h.effectiveResultStatus(resultID, req.Status)
		if err != nil {
			// FAIL CLOSED. An errored read establishes NOTHING about this row: it may exist, and it
			// may be a PASS. Falling through would hand the caller's defect_type to the write below,
			// whose WHERE matches on id alone and so lands it on whatever the row actually is —
			// persisting, out of a transient SQLITE_BUSY or IO blip, exactly the non-failure-with-a-
			// defect-type state models.RunResult.DefectType declares impossible. Refusing costs the
			// caller a retry; guessing costs an invisible row nobody can correct through the UI.
			slog.ErrorContext(r.Context(), "failed to resolve result status for triage",
				"result_id", resultID, "run_id", runID, "error", err)
			httpx.Error(w, http.StatusInternalServerError, err)
			return
		}
		switch {
		case known && !models.IsFailureStatus(effStatus):
			// Nothing to triage on a non-failure, whatever the caller sent. The snapshot columns go
			// with it: leaving a previous decision's suggestion behind on a row whose defect_type
			// was just blanked is the stale-pairing this codebase clears everywhere else.
			updateMap["defect_type"] = ""
			clearAISuggestion(updateMap)
		case req.DefectType != nil:
			// An explicitly supplied defect_type is a human triage decision — snapshot what the AI
			// suggested at this exact moment. `known` is false only when the row is genuinely
			// absent (the errored read returned above), so the write below matches nothing anyway.
			updateMap["defect_type"] = *req.DefectType
			h.snapshotAISuggestion(r.Context(), resultID, updateMap, effStatus, known)
			guardedTriage = req.Status == nil
		case req.Status != nil:
			// A failure with no explicit decision starts at the "nobody has looked at this yet"
			// default. Deliberately NOT snapshotted: it is not a decision.
			updateMap["defect_type"] = "to_investigate"
		}
	}
	if req.ErrorMessage != nil {
		updateMap["error_message"] = *req.ErrorMessage
	}
	if req.StackTrace != nil {
		updateMap["stack_trace"] = *req.StackTrace
	}
	if req.FailureType != nil {
		updateMap["failure_type"] = *req.FailureType
	}
	if req.DurationMs != nil {
		updateMap["duration_ms"] = *req.DurationMs
	}
	if req.Screenshots != nil {
		if !screenshotsURLsSafe(*req.Screenshots) {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "screenshots must be a JSON array of http(s)/relative URLs"})
			return
		}
		updateMap["screenshots"] = *req.Screenshots
	}
	if req.Video != nil {
		if !isSafeArtifactURL(*req.Video) {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "video must be an http(s) or relative URL"})
			return
		}
		updateMap["video"] = *req.Video
	}
	if req.TraceURL != nil {
		if !isSafeArtifactURL(*req.TraceURL) {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "trace_url must be an http(s) or relative URL"})
			return
		}
		updateMap["trace_url"] = *req.TraceURL
	}
	if req.LogText != nil {
		updateMap["log_text"] = *req.LogText
	}
	if req.Browser != nil {
		updateMap["browser"] = *req.Browser
	}
	if req.OS != nil {
		updateMap["os"] = *req.OS
	}
	if req.Environment != nil {
		updateMap["environment"] = *req.Environment
	}
	if req.AppVersion != nil {
		updateMap["app_version"] = *req.AppVersion
	}
	if req.Steps != nil {
		raw := *req.Steps
		if len(raw) > 32*1024 {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "steps too large (max 32 KB)"})
			return
		}
		s := strings.TrimSpace(string(raw))
		if !json.Valid(raw) || s == "" || s[0] != '[' {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "steps must be a JSON array"})
			return
		}
		updateMap["steps"] = json.RawMessage(raw)
	}
	updateMap["updated_at"] = time.Now()

	// The status-omitted triage write goes through the status-guarded statement so the check and the
	// act are ONE operation, exactly as bulk Mode 2 does. Resolving the status above and updating
	// here is otherwise a check-then-act: between the two, CI can re-report the result, a bulk
	// update can land, or the execute page can mark it PASS — and this UPDATE, keyed on the id
	// alone, would stamp the decision onto a row that stopped being a failure. This is the
	// highest-traffic triage writer in the app (the per-row select and the AI "Accept" button), so
	// it is the last place that window belongs. A caller that DID supply a status needs no guard:
	// it is stating what the row should become, so the prior status is irrelevant by definition.
	var err error
	if guardedTriage {
		// Count deliberately discarded: this endpoint has always answered 200 to a write that
		// matched nothing (an unknown result id), and a row concurrently moved out of the failure
		// set is the same non-event.
		_, err = h.store.TriageRunResult(runID, resultID, updateMap)
	} else {
		err = h.store.UpdateRunResult(runID, resultID, updateMap)
	}
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to update result", "result_id", resultID, "run_id", runID, "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	if req.Status != nil {
		if _, err := h.store.MarkRunRunningIfPending(runID); err != nil {
			slog.WarnContext(r.Context(), "failed to auto-start run", "run_id", runID, "error", err)
		}
	}

	h.store.TouchTestRun(runID)

	h.broadcastResultDelta(apiws.EventResultUpdated, runID, []string{resultID}, nil, nil)

	httpx.JSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// clearAISuggestion blanks the snapshot columns in updateMap. Every path that writes a triage
// decision WITHOUT a matching snapshot must call this: the columns are written in the same
// statement as defect_type, so leaving them untouched would pair a brand-new human decision
// with the AI suggestion recorded for some EARLIER one and fabricate an agreement.
func clearAISuggestion(updateMap map[string]interface{}) {
	updateMap["suggested_verdict"] = ""
	updateMap["suggested_defect_type"] = ""
	updateMap["suggested_confidence"] = ""
	updateMap["decided_at"] = nil
}

// snapshotAISuggestion adds the AI failure-analysis suggestion columns to updateMap so the
// calibration record captures what the AI proposed at the instant a human triaged the result.
// Snapshotting (rather than joining later) keeps the record immune to a subsequent re-analysis
// changing the verdict.
//
// Callers must invoke this ONLY when the caller explicitly supplied a defect_type, and must pass
// the EFFECTIVE status already resolved by effectiveResultStatus (with its `known` flag) rather
// than let this function re-read it — one read per request, and the decision this snapshot is
// filed against is provably the same one the caller branched on.
// It further requires the result to be a FAILURE (FAIL or ERROR, per models.IsFailureStatus)
// *after this update lands* and to already have an analysis.
// Gating on the EFFECTIVE status is what makes this path agree with the bulk one: both decide
// from the status the row will have, so identical input produces an identical record whichever
// endpoint wrote it. Gating on the stored status instead would miss the single-call
// {status:"FAIL", defect_type:"X"} shape the CLI and the execute page actually send, and would
// wrongly snapshot {status:"PASS", defect_type:"X"} on a row that happened to be FAIL before.
//
// Best-effort by design: an unknown status or a missing analysis logs and CLEARS the snapshot
// instead of failing. A calibration nicety must never fail a human's triage write. (A read that
// ERRORED never reaches here — the caller rejects the request outright.)
func (h *Handler) snapshotAISuggestion(ctx context.Context, resultID string, updateMap map[string]interface{}, status models.ExecutionStatus, known bool) {
	// Cleared first, filled in only on the fully-verified path below, so every early return
	// leaves the columns blank rather than stale.
	clearAISuggestion(updateMap)

	if !known || !models.IsFailureStatus(status) {
		return
	}

	a, err := h.store.GetCurrentAnalysisForResult(resultID)
	if err != nil {
		slog.WarnContext(ctx, "ai-suggestion snapshot: analysis lookup failed", "result_id", resultID, "error", err)
		return
	}
	if a == nil {
		return
	}

	updateMap["suggested_verdict"] = a.Verdict
	updateMap["suggested_defect_type"] = models.SuggestedDefectType(a.Verdict)
	updateMap["suggested_confidence"] = a.Confidence
	updateMap["decided_at"] = time.Now().UTC()
}

// effectiveResultStatus resolves the status the result will hold once the update is applied:
// the requested status when the caller supplied one, otherwise the stored status.
//
// The two ways it can fail to establish a status are returned SEPARATELY, and callers must keep
// them apart. A missing row is (false, nil): a legitimate no-op, since a statement keyed on that
// id matches nothing. A read that ERRORED is (false, non-nil): the row may well exist and hold any
// status at all, so nothing may be decided from it. Collapsing the two into a single "unknown"
// boolean is what let a transient read failure be treated as "row absent" and fall through to
// writing a triage decision onto a row whose status was never established.
func (h *Handler) effectiveResultStatus(resultID string, reqStatus *string) (models.ExecutionStatus, bool, error) {
	if reqStatus != nil {
		return models.ExecutionStatus(*reqStatus), true, nil
	}
	rr, err := h.store.GetRunResultByID(resultID)
	if err != nil {
		return "", false, err
	}
	if rr == nil {
		return "", false, nil
	}
	return rr.Status, true, nil
}

// snapshotAISuggestionsBulk records, for each bulk-triaged result, what the AI suggested at the
// moment of the human decision — the bulk counterpart of snapshotAISuggestion.
//
// It must run as a SEPARATE pass after the main bulk UPDATE: store.BulkUpdateRunResults applies
// ONE shared map to every ID, so the shared status/defect_type can go in a single statement but
// the snapshot values, which differ per row, cannot. Writing them there would smear one row's
// verdict across the whole selection and silently corrupt the calibration record.
//
// Rather than one UPDATE per row (up to 500), results are bucketed by their snapshot values and
// each bucket is written with a single grouped "id IN (...)" statement. suggested_defect_type is
// a pure function of the verdict, so the bucket key needs only (verdict, confidence) — in
// practice at most 6 verdicts x 3 confidences = 18 statements, regardless of selection size.
//
// Results with no analysis are not skipped but CLEARED. The caller's main UPDATE has already
// blanked these columns atomically with defect_type, so this is a redundant safety net rather
// than the only guard — it keeps the function correct on its own terms (as snapshotAISuggestion
// is, clearing before every early return) should a future caller not pre-blank.
//
// Best-effort by design: a failed lookup or grouped write logs and moves on. The human's bulk
// triage has already been committed and must stand. That is only safe BECAUSE the caller blanked
// in the main statement: every write here either fills in a correct snapshot or leaves the
// columns blank, and blank is excluded from the calibration set. This pass can never resurrect a
// stale suggestion by failing.
func (h *Handler) snapshotAISuggestionsBulk(ctx context.Context, runID string, resultIDs []string, now time.Time) {
	clearRows := func(ids []string, reason string) {
		if len(ids) == 0 {
			return
		}
		updates := map[string]interface{}{"updated_at": now}
		clearAISuggestion(updates)
		if _, err := h.store.BulkUpdateRunResults(runID, ids, updates); err != nil {
			slog.WarnContext(ctx, "ai-suggestion bulk snapshot: clearing update failed",
				"run_id", runID, "reason", reason, "count", len(ids), "error", err)
		}
	}

	analyses, err := h.store.GetCurrentAnalysesByRun(runID)
	if err != nil {
		slog.WarnContext(ctx, "ai-suggestion bulk snapshot: analysis lookup failed", "run_id", runID, "error", err)
		clearRows(resultIDs, "analysis lookup failed")
		return
	}

	type snapshotKey struct{ verdict, confidence string }
	buckets := make(map[snapshotKey][]string)
	var noAnalysis []string
	for _, id := range resultIDs {
		a := analyses[id]
		if a == nil {
			noAnalysis = append(noAnalysis, id)
			continue
		}
		k := snapshotKey{verdict: a.Verdict, confidence: a.Confidence}
		buckets[k] = append(buckets[k], id)
	}
	clearRows(noAnalysis, "no analysis")

	decidedAt := now.UTC() // UTC: the accuracy window compares decided_at as TEXT, see models.RunResult
	for k, ids := range buckets {
		updates := map[string]interface{}{
			"suggested_verdict":     k.verdict,
			"suggested_defect_type": models.SuggestedDefectType(k.verdict),
			"suggested_confidence":  k.confidence,
			"decided_at":            decidedAt,
			"updated_at":            now,
		}
		if _, err := h.store.BulkUpdateRunResults(runID, ids, updates); err != nil {
			slog.WarnContext(ctx, "ai-suggestion bulk snapshot: grouped update failed",
				"run_id", runID, "verdict", k.verdict, "count", len(ids), "error", err)
		}
	}
}

// DeleteTestRun godoc
//
// @Summary      Delete a test run
// @Description  Permanently deletes a test run and its results.
// @Tags         runs
// @Param        id  path  string  true  "Test run ID"
// @Security     BearerAuth
// @Success      204  "No Content"
// @Failure      500  {object}  object{error=string}
// @Router       /runs/{id} [delete]
func (h *Handler) DeleteTestRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.DeleteTestRun(id); err != nil {
		slog.ErrorContext(r.Context(), "failed to delete run", "run_id", id, "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	if h.hub != nil {
		h.hub.Broadcast(apiws.NewEvent(apiws.EventRunDeleted, "runs:*", map[string]string{"id": id}))
	}

	w.WriteHeader(http.StatusNoContent)
}

// BulkDeleteTestRuns godoc
//
// @Summary      Bulk delete test runs
// @Description  Permanently deletes multiple test runs (and their results) in one request.
// @Tags         runs
// @Accept       json
// @Param        body  body  object{ids=[]string}  true  "IDs of the test runs to delete (max 500)"
// @Security     BearerAuth
// @Success      204  "No Content"
// @Failure      400  {object}  object{error=string}
// @Failure      500  {object}  object{error=string}
// @Router       /runs/bulk-delete [post]
func (h *Handler) BulkDeleteTestRuns(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if len(req.IDs) == 0 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "ids are required"})
		return
	}
	if len(req.IDs) > httpx.MaxBulkIDs {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "too many ids (max 500 per request)"})
		return
	}
	if err := h.store.DeleteTestRuns(req.IDs); err != nil {
		slog.ErrorContext(r.Context(), "failed to bulk delete runs", "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// UpdateTestRun godoc
//
// @Summary      Update a test run
// @Description  Updates a test run's name, category, and/or status.
// @Tags         runs
// @Accept       json
// @Produce      json
// @Param        id    path  string                                                true  "Test run ID"
// @Param        body  body  object{name=string,category_id=string,status=string}  true  "Fields to update"
// @Security     BearerAuth
// @Success      200  {object}  object{status=string}
// @Failure      400  {object}  object{error=string}
// @Failure      500  {object}  object{error=string}
// @Router       /runs/{id} [put]
func (h *Handler) UpdateTestRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Name       *string `json:"name"`
		CategoryID *string `json:"category_id"`
		Status     *string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if req.Status != nil && !models.IsValidRunStatus(*req.Status) {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid status"})
		return
	}
	if err := h.store.UpdateTestRun(id, req.Name, req.CategoryID, req.Status); err != nil {
		slog.ErrorContext(r.Context(), "failed to update run", "run_id", id, "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	if fullRun, err := h.store.GetTestRun(id); err == nil && fullRun != nil && h.hub != nil {
		h.hub.Broadcast(apiws.NewEvent(apiws.EventRunUpdated, "run:"+id, fullRun))
		h.hub.Broadcast(apiws.NewEvent(apiws.EventRunUpdated, "runs:*", fullRun))
	}

	httpx.JSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// AddRunResult godoc
//
// @Summary      Add a result to a run
// @Description  Adds a single test-case result snapshot to an existing test run, validating artifact URLs (video, trace_url, screenshots) and defect_type. defect_type is kept only for a failure status (FAIL or ERROR) and cleared for any other.
// @Tags         runs
// @Accept       json
// @Produce      json
// @Param        id    path  string                         true  "Test run ID"
// @Param        body  body  models.CreateRunResultRequest  true  "Result to add"
// @Security     BearerAuth
// @Success      201  {object}  models.RunResult
// @Failure      400  {object}  object{error=string}
// @Failure      409  {object}  object{error=string}
// @Failure      500  {object}  object{error=string}
// @Router       /runs/{id}/results [post]
func (h *Handler) AddRunResult(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")
	var req models.CreateRunResultRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	if req.TestCaseID == nil || *req.TestCaseID == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "test_case_id is required"})
		return
	}
	if req.AttemptNumber < 0 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "attempt_number must be positive"})
		return
	}

	// Validate artifact URLs on the create path too, not just on update (F-012).
	if !isSafeArtifactURL(req.Video) {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "video must be an http(s) or relative URL"})
		return
	}
	if !isSafeArtifactURL(req.TraceURL) {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "trace_url must be an http(s) or relative URL"})
		return
	}
	if !screenshotsURLsSafe(req.Screenshots) {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "screenshots must be a JSON array of http(s)/relative URLs"})
		return
	}
	// Validate defect_type on the create path too, for the same reason the update and bulk paths do
	// it — and with more urgency, because this is the endpoint CI ingest and `ttgo runs results add
	// --defect-type` write through, i.e. the highest-volume and least-supervised writer of the
	// column. Skipping it here let an unvalidated string reach every calibration and counter query
	// that groups by defect_type, and reach the failure-analysis prompt, where enrich.go interpolates
	// the stored value OUTSIDE the <<<DATA ... DATA>>> fences that wrap untrusted error text.
	if !models.IsValidDefectType(req.DefectType) {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid defect_type"})
		return
	}

	result := req.ToRunResult(runID)
	// Same status gate as the update and bulk paths: defect_type is meaningful only on a failure, so
	// a create that reports PASS carries nothing to triage no matter what it sent.
	if !models.IsFailureStatus(result.Status) {
		result.DefectType = ""
	}
	if err := h.store.AddRunResult(result); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			httpx.JSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		slog.ErrorContext(r.Context(), "failed to add result to run", "run_id", runID, "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	h.broadcastResultDelta(apiws.EventResultUpdated, runID, []string{result.ID}, nil, nil)

	httpx.JSON(w, http.StatusCreated, result)
}

// AddRunResultsBulk godoc
//
// @Summary      Bulk add results to a run
// @Description  Adds several test cases to an existing run in one shot, snapshotting each as a new attempt-1 PENDING result and skipping any already present. Mirrors the create path's test_case_ids handling.
// @Tags         runs
// @Accept       json
// @Produce      json
// @Param        id    path  string                          true  "Test run ID"
// @Param        body  body  object{test_case_ids=[]string}  true  "Test case IDs to add"
// @Security     BearerAuth
// @Success      201  {array}   models.RunResult
// @Failure      400  {object}  object{error=string}
// @Failure      404  {object}  object{error=string}
// @Failure      500  {object}  object{error=string}
// @Router       /runs/{id}/results/bulk [post]
func (h *Handler) AddRunResultsBulk(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")
	var req struct {
		TestCaseIDs []string `json:"test_case_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	nonEmpty := 0
	for _, id := range req.TestCaseIDs {
		if id != "" {
			nonEmpty++
		}
	}
	if nonEmpty == 0 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "test_case_ids is required"})
		return
	}
	if len(req.TestCaseIDs) > httpx.MaxBulkIDs {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "too many test_case_ids (max 500 per request)"})
		return
	}

	created, err := h.store.AddTestCasesToRun(runID, req.TestCaseIDs)
	if err != nil {
		if errors.Is(err, store.ErrUnknownTestCases) {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "one or more test_case_ids do not exist"})
			return
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "test run not found"})
			return
		}
		slog.ErrorContext(r.Context(), "failed to bulk-add results to run", "run_id", runID, "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	if created == nil {
		created = []models.RunResult{}
	}
	if len(created) > 0 {
		ids := make([]string, len(created))
		for i := range created {
			ids[i] = created[i].ID
		}
		h.broadcastResultDelta(apiws.EventResultUpdated, runID, ids, nil, nil)
	}

	httpx.JSON(w, http.StatusCreated, created)
}

// DeleteRunResult godoc
//
// @Summary      Delete a run result
// @Description  Removes a single result from a test run.
// @Tags         runs
// @Param        id         path  string  true  "Test run ID"
// @Param        result_id  path  string  true  "Run result ID"
// @Security     BearerAuth
// @Success      204  "No Content"
// @Failure      500  {object}  object{error=string}
// @Router       /runs/{id}/results/{result_id} [delete]
func (h *Handler) DeleteRunResult(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")
	resultID := r.PathValue("result_id")

	if err := h.store.DeleteRunResult(runID, resultID); err != nil {
		slog.ErrorContext(r.Context(), "failed to delete result from run", "result_id", resultID, "run_id", runID, "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	h.broadcastResultDelta(apiws.EventResultDeleted, runID, nil, nil, []string{resultID})

	w.WriteHeader(http.StatusNoContent)
}

// AssignRunToFolder godoc
//
// @Summary      Assign a run to a folder
// @Description  Moves a test run into the given run folder, or clears its folder when run_folder_id is null.
// @Tags         runs
// @Accept       json
// @Produce      json
// @Param        id    path  string                        true  "Test run ID"
// @Param        body  body  object{run_folder_id=string}  true  "Target folder ID, or null to unassign"
// @Security     BearerAuth
// @Success      200  {object}  object{status=string}
// @Failure      400  {object}  object{error=string}
// @Failure      404  {object}  object{error=string}
// @Failure      500  {object}  object{error=string}
// @Router       /runs/{id}/folder [patch]
func (h *Handler) AssignRunToFolder(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	var folderID *string
	if v, ok := body["run_folder_id"]; ok && v != nil {
		value, ok := v.(string)
		if !ok {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "run_folder_id must be a string or null"})
			return
		}
		folderID = &value
	}

	if folderID != nil {
		folder, err := h.store.GetRunFolder(*folderID)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to look up run folder", "folder_id", *folderID, "error", err)
			httpx.Error(w, http.StatusInternalServerError, err)
			return
		}
		if folder == nil {
			httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "target run folder not found"})
			return
		}
	}

	if err := h.store.AssignRunToFolder(id, folderID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "test run not found"})
			return
		}
		slog.ErrorContext(r.Context(), "failed to assign run to folder", "run_id", id, "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	slog.InfoContext(r.Context(), "run assigned to folder", "run_id", id, "folder_id", folderID)
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// isSafeArtifactURL allows only http(s) or root-relative URLs for artifact links
// that the UI renders as anchors, blocking javascript:/data: stored-XSS (F-012).
func isSafeArtifactURL(s string) bool {
	ts := strings.TrimSpace(s)
	if ts == "" {
		return true // empty clears the field
	}
	ls := strings.ToLower(ts)
	return strings.HasPrefix(ls, "http://") || strings.HasPrefix(ls, "https://") || strings.HasPrefix(ts, "/")
}

// screenshotsURLsSafe reports whether s (a JSON array of URL strings, or empty)
// contains only http(s)/relative URLs. The UI renders screenshots[0] as an
// anchor, so a javascript:/data: entry would be a stored-XSS sink (F-012).
func screenshotsURLsSafe(s string) bool {
	if strings.TrimSpace(s) == "" {
		return true
	}
	var urls []string
	if err := json.Unmarshal([]byte(s), &urls); err != nil {
		return false // screenshots must be a JSON array
	}
	for _, u := range urls {
		if !isSafeArtifactURL(u) {
			return false
		}
	}
	return true
}

// CopyTestRun godoc
//
// @Summary      Copy a test run
// @Description  Creates a new PENDING run from an existing one, copying the latest attempt of each result. If name is omitted, it defaults to "Copy of <source name>".
// @Tags         runs
// @Accept       json
// @Produce      json
// @Param        id    path  string                                     true   "Source test run ID"
// @Param        body  body  object{name=string,run_folder_id=string}  false  "Copy options (optional)"
// @Security     BearerAuth
// @Success      201  {object}  models.TestRun
// @Failure      500  {object}  object{error=string}
// @Router       /runs/{id}/copy [post]
func (h *Handler) CopyTestRun(w http.ResponseWriter, r *http.Request) {
	sourceID := r.PathValue("id")

	var req struct {
		Name        string  `json:"name"`
		RunFolderID *string `json:"run_folder_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	run, err := h.store.CopyTestRun(sourceID, req.Name, req.RunFolderID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to copy test run", "source_id", sourceID, "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	httpx.JSON(w, http.StatusCreated, run)
}

// CompleteRun godoc
//
// @Summary      Complete a run
// @Description  Marks a test run as finished, computing its final status from its results, broadcasting the update, and (if configured) enqueuing AI failure analysis for its failures.
// @Tags         runs
// @Produce      json
// @Param        id  path  string  true  "Test run ID"
// @Security     BearerAuth
// @Success      200  {object}  object{id=string,status=string,updated_at=string}
// @Failure      404  {object}  object{error=string}
// @Failure      500  {object}  object{error=string}
// @Router       /runs/{id}/complete [post]
func (h *Handler) CompleteRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	run, changed, err := h.store.CompleteRun(id)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to complete run", "run_id", id, "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if run == nil {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "test run not found"})
		return
	}

	if changed {
		if h.notifyRunCompleted != nil {
			h.notifyRunCompleted(r.Context(), run)
		}
		if fullRun, err := h.store.GetTestRun(id); err == nil && fullRun != nil && h.hub != nil {
			h.hub.Broadcast(apiws.NewEvent(apiws.EventRunUpdated, "run:"+id, fullRun))
			h.hub.Broadcast(apiws.NewEvent(apiws.EventRunUpdated, "runs:*", fullRun))
		}

		// ai-failure-analysis: auto-on-completion enqueue.
		// Non-fatal — run finalization succeeds regardless.
		if settings, err := h.store.GetFailureAnalysisSettings(); err == nil && settings.EnabledOnCompletion {
			provider, _ := h.store.GetDefaultProviderConfig()
			if provider != nil && provider.AllowAutoFailureAnalysis {
				failures, err := h.store.ListLatestFailingResults(run.ID)
				if err == nil && len(failures) > 0 {
					if _, _, err := h.store.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerAutoOnDone, ""); err != nil {
						slog.WarnContext(r.Context(), "ai-failure-analysis: auto enqueue failed", "run_id", run.ID, "err", err)
					}
				}
			} else {
				slog.WarnContext(r.Context(), "ai-failure-analysis: auto skipped — no approved default provider", "run_id", run.ID)
			}
		}
	}

	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"id":         run.ID,
		"status":     run.Status,
		"updated_at": run.UpdatedAt,
	})
}

// ReopenRun godoc
//
// @Summary      Reopen a run
// @Description  Reverts a completed test run back to an in-progress state.
// @Tags         runs
// @Produce      json
// @Param        id  path  string  true  "Test run ID"
// @Security     BearerAuth
// @Success      200  {object}  object{id=string,status=string,updated_at=string}
// @Failure      404  {object}  object{error=string}
// @Failure      500  {object}  object{error=string}
// @Router       /runs/{id}/reopen [post]
func (h *Handler) ReopenRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	run, err := h.store.ReopenRun(id)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to reopen run", "run_id", id, "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if run == nil {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "test run not found"})
		return
	}

	if fullRun, err := h.store.GetTestRun(id); err == nil && fullRun != nil && h.hub != nil {
		h.hub.Broadcast(apiws.NewEvent(apiws.EventRunUpdated, "run:"+id, fullRun))
		h.hub.Broadcast(apiws.NewEvent(apiws.EventRunUpdated, "runs:*", fullRun))
	}

	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"id":         run.ID,
		"status":     run.Status,
		"updated_at": run.UpdatedAt,
	})
}

// AssignRun godoc
//
// @Summary      Assign a run
// @Description  Sets or clears the assignee for a test run; assignee_id must reference an active user.
// @Tags         runs
// @Accept       json
// @Produce      json
// @Param        id    path  string                      true  "Test run ID"
// @Param        body  body  object{assignee_id=string}  true  "Active user ID to assign, or null to unassign"
// @Security     BearerAuth
// @Success      200  {object}  object{status=string}
// @Failure      400  {object}  object{error=string}
// @Failure      404  {object}  object{error=string}
// @Failure      500  {object}  object{error=string}
// @Router       /runs/{id}/assignee [put]
func (h *Handler) AssignRun(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")
	var req struct {
		AssigneeID *string `json:"assignee_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if req.AssigneeID != nil && *req.AssigneeID == "" {
		req.AssigneeID = nil // empty string clears
	}
	if req.AssigneeID != nil {
		u, err := h.store.GetUser(*req.AssigneeID)
		if err != nil || u == nil || !u.Active || u.Deleted {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "assignee_id must reference an active user"})
			return
		}
	}
	changed, err := h.store.AssignRun(runID, req.AssigneeID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to assign run", "run_id", runID, "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if !changed {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "test run not found"})
		return
	}
	if fullRun, err := h.store.GetTestRun(runID); err == nil && fullRun != nil && h.hub != nil {
		h.hub.Broadcast(apiws.NewEvent(apiws.EventRunUpdated, "run:"+runID, fullRun))
		h.hub.Broadcast(apiws.NewEvent(apiws.EventRunUpdated, "runs:*", fullRun))
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "assigned"})
}

// dedupeStrings returns the input with repeats removed, preserving first-seen order.
func dedupeStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// BulkUpdateRunResults godoc
//
// @Summary      Bulk update run results
// @Description  Applies the same status and/or defect_type to multiple results within a run. At least one of the two is required. When status is given it is applied to every selected result; for a failure status (FAIL or ERROR) defect_type defaults to "to_investigate" when not given, and every other status clears it. When status is omitted the request is a triage-only decision: defect_type is applied without touching status, and only to results whose stored status is FAIL or ERROR — the rest are left untouched and reported in "skipped". "updated" counts the rows actually written, so repeated ids and ids outside this run land in "skipped" rather than being reported as writes.
// @Tags         runs
// @Accept       json
// @Produce      json
// @Param        id    path  string                                                       true  "Test run ID"
// @Param        body  body  object{result_ids=[]string,status=string,defect_type=string}  true  "Result IDs and the status/defect_type to apply"
// @Security     BearerAuth
// @Success      200  {object}  object{status=string,updated=int,skipped=int}
// @Failure      400  {object}  object{error=string}
// @Failure      500  {object}  object{error=string}
// @Router       /runs/{id}/results/bulk-update [post]
func (h *Handler) BulkUpdateRunResults(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")

	var req struct {
		ResultIDs  []string `json:"result_ids"`
		Status     string   `json:"status"`
		DefectType string   `json:"defect_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if len(req.ResultIDs) == 0 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "result_ids are required"})
		return
	}
	if len(req.ResultIDs) > httpx.MaxBulkIDs {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "too many result_ids (max 500 per request)"})
		return
	}
	// Either field alone is a complete request: status-only is a plain status change, defect_type-only
	// is a triage decision that leaves status untouched. Requiring at least one stops an empty body
	// from reporting rows "updated" after a no-op UPDATE.
	if req.Status == "" && req.DefectType == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "at least one of status or defect_type is required"})
		return
	}
	if req.Status != "" && !models.IsValidExecutionStatus(req.Status) {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid status"})
		return
	}
	// IsValidDefectType accepts "", so this also covers the status-only shape.
	if !models.IsValidDefectType(req.DefectType) {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid defect_type"})
		return
	}

	now := time.Now()

	// Which rows the request actually applies to.
	//
	// Mode 1 (status supplied): every selected row, as always — the caller is stating what the
	// rows should become, so their stored status is irrelevant.
	//
	// Mode 2 (status absent, defect_type supplied): a pure triage decision. defect_type is only
	// meaningful on a failure, and there is no status in the request to make a non-failure row
	// into one, so the STORED status decides. Rows failing that test are reported as skipped and
	// left completely alone — writing the shared map over them would set defect_type on a PASS,
	// which every other path in this file treats as impossible.
	//
	// The selection is deduplicated first, because every count below is a count of ROWS: an UPDATE
	// matches a row once however many times the caller listed its id, so reporting over the raw
	// list would claim more rows were written than the run even contains.
	requestedIDs := dedupeStrings(req.ResultIDs)
	targetIDs := requestedIDs
	if req.Status == "" {
		statuses, err := h.store.ListRunResultStatuses(runID, requestedIDs)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to read result statuses for bulk triage", "run_id", runID, "error", err)
			httpx.Error(w, http.StatusInternalServerError, err)
			return
		}
		applicable := make([]string, 0, len(requestedIDs))
		for _, id := range requestedIDs {
			// An id absent from the map (not in this run) resolves to the zero status, which is
			// not a failure — so foreign ids are skipped rather than silently counted as updated.
			if models.IsFailureStatus(statuses[id]) {
				applicable = append(applicable, id)
			}
		}
		targetIDs = applicable
	}
	if len(targetIDs) == 0 {
		// Only reachable in Mode 2 (Mode 1 keeps the whole non-empty selection). Nothing was
		// written, so nothing is broadcast either. This is a 200, not an error: selecting a mixed
		// set and triaging it is the normal way to use the picker, and "none of these were
		// failures" is a reportable outcome rather than a bad request.
		httpx.JSON(w, http.StatusOK, map[string]interface{}{
			"status": "updated", "updated": 0, "skipped": len(requestedIDs),
		})
		return
	}

	// An explicitly supplied defect_type on a failure is a human triage decision; the
	// "to_investigate" auto-default (empty req.DefectType) means "not triaged yet" and is
	// deliberately not one. It matches the single-result path, which touches these columns only
	// when the caller explicitly supplied a defect_type.
	//
	// One gate drives BOTH the clearing folded into the main statement below and the snapshot pass
	// after it, because both now run over targetIDs — the rows that are failures either way they
	// got there. That pairing is what keeps bulk fail-closed: the clear lands atomically with
	// defect_type, the snapshot is a separate best-effort pass, so a failure of the second leaves
	// the columns blank and blank is excluded from the calibration set. The reverse pairing —
	// snapshotting rows the main statement did not clear — would be the bug.
	isTriage := req.DefectType != "" &&
		(req.Status == "" || models.IsFailureStatus(models.ExecutionStatus(req.Status)))

	updateMap := map[string]interface{}{
		"updated_at": now,
	}
	switch {
	case req.Status == "":
		// Mode 2: apply the defect_type and leave status alone. "status" is deliberately absent
		// from this map — targetIDs was already narrowed to the stored failures above, so there is
		// nothing to correct here, and a status key would silently rewrite rows the caller never
		// asked to change.
		updateMap["defect_type"] = req.DefectType
	case models.IsFailureStatus(models.ExecutionStatus(req.Status)):
		// FAIL and ERROR are both failures and both triageable, exactly as the single-result path
		// treats them — gating on FAIL alone here would force defect_type="" on an ERROR that the
		// single-result endpoint triages, so the same request would mean two different things
		// depending on which endpoint received it.
		updateMap["status"] = req.Status
		if req.DefectType != "" {
			updateMap["defect_type"] = req.DefectType
		} else {
			updateMap["defect_type"] = "to_investigate"
		}
	default:
		// PASS/SKIP/PENDING/RUNNING have nothing to triage. Blanking defect_type without also
		// blanking the snapshot columns would strand a previous decision's suggestion on the row,
		// so the clear is folded in here for the same reason isTriage folds it in below — this
		// branch is simply the other way a row's defect_type gets rewritten.
		updateMap["status"] = req.Status
		updateMap["defect_type"] = ""
		clearAISuggestion(updateMap)
	}
	if isTriage {
		// Blanked in the SAME statement that writes defect_type, exactly as the single-result
		// path does — this is what makes bulk fail CLOSED. The per-row snapshot below is a
		// separate, best-effort pass; if it never lands (SQLite busy, disk error) these columns
		// stay BLANK, which accuracyCalibrationFilter excludes. Leaving them untouched here
		// instead would pair the brand-new defect_type with the suggestion snapshotted for some
		// EARLIER decision and fabricate a calibration record out of two unrelated events.
		clearAISuggestion(updateMap)
	}

	// Mode 2 goes through the status-guarded statement so the partition above is ATOMIC. Reading the
	// statuses and updating afterwards is a check-then-act: between the two, CI can re-report a
	// result, another bulk update can land, or the execute page can mark it PASS — and the UPDATE
	// would then write a triage decision onto a row that stopped being a failure. Repeating the test
	// inside the statement closes that window. Mode 1 needs no such guard: the caller is stating what
	// the rows should become, so their prior status is irrelevant by definition.
	update := h.store.BulkUpdateRunResults
	if req.Status == "" {
		update = h.store.BulkTriageRunResults
	}
	updated, err := update(runID, targetIDs, updateMap)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to bulk update results in run", "run_id", runID, "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	// Snapshot what the AI suggested for each result. snapshotAISuggestionsBulk has no status gate
	// of its own — it snapshots every id handed to it — which is exactly why it is handed targetIDs
	// and not req.ResultIDs: in Mode 1 a supplied failure status makes every selected row a
	// failure, and in Mode 2 the partition above already dropped everything that was not one.
	if isTriage {
		h.snapshotAISuggestionsBulk(r.Context(), runID, targetIDs, now)
	}

	// Mode 2 changes no result status, so it must not start a PENDING run either — matching the
	// single-result path, which auto-starts only when the caller supplied a status.
	if req.Status != "" {
		if _, err := h.store.MarkRunRunningIfPending(runID); err != nil {
			slog.WarnContext(r.Context(), "failed to auto-start run", "run_id", runID, "error", err)
		}
	}

	h.store.TouchTestRun(runID)

	// The patch is applied client-side to every id in the event (frontend utils/runResults.js,
	// applyResultDelta), so it must carry ONLY values that hold for every listed row and target ONLY
	// the rows it was written to. A "status" key here in Mode 2 would carry the empty req.Status and
	// blank the status of every listed row in the live grid; listing the skipped ids would apply
	// someone else's defect_type to them. Mode 1 is unaffected: targetIDs is the full selection
	// and req.Status is non-empty by definition.
	//
	// The snapshot columns are deliberately NOT in here even though the statement above blanked
	// them, for two different reasons depending on the branch. Where isTriage held,
	// snapshotAISuggestionsBulk has already refilled each row with its OWN verdict, so they are
	// per-row by the time the client would see them and a shared patch could only carry the
	// pre-snapshot blanks. Where it did NOT — the non-failure `default:` branch, which blanks them
	// with nothing running behind it to refill — the client simply never learns they were cleared.
	// Neither costs anything: no frontend view reads result.suggested_*/decided_at at all (the
	// suggestion chip reads analysis.suggested_defect_type), so a stale local copy is invisible and
	// corrects itself on the next fetch regardless.
	patch := map[string]any{
		"defect_type": updateMap["defect_type"],
		"updated_at":  now,
	}
	if req.Status != "" {
		patch["status"] = req.Status
	}
	h.broadcastResultDelta(apiws.EventResultBulkUpdated, runID, targetIDs, patch, nil)

	// Counted from what the statement matched, not from what the caller listed: ids belonging to
	// another run, ids that no longer exist, and rows a concurrent write moved out of the failure
	// set in Mode 2 all have to land in "skipped", or the response would claim writes that never
	// happened.
	resp := map[string]interface{}{
		"status":  "updated",
		"updated": updated,
		"skipped": len(requestedIDs) - int(updated),
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// RetryRunResult godoc
//
// @Summary      Retry a run result
// @Description  Creates a new PENDING attempt for a result, incrementing its attempt number; fails if the result is orphaned (its test case was deleted).
// @Tags         runs
// @Produce      json
// @Param        id         path  string  true  "Test run ID"
// @Param        result_id  path  string  true  "Run result ID"
// @Security     BearerAuth
// @Success      201  {object}  object{id=string,test_case_id=string,attempt_number=int,status=string}
// @Failure      400  {object}  object{error=string}
// @Failure      404  {object}  object{error=string}
// @Failure      500  {object}  object{error=string}
// @Router       /runs/{id}/results/{result_id}/retry [post]
func (h *Handler) RetryRunResult(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")
	resultID := r.PathValue("result_id")

	newResult, err := h.store.RetryRunResult(runID, resultID)
	if err != nil {
		if err.Error() == "record not found" {
			httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "result not found"})
			return
		}
		if strings.Contains(err.Error(), "orphaned") {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "cannot retry orphaned result"})
			return
		}
		slog.ErrorContext(r.Context(), "failed to retry result", "result_id", resultID, "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	h.broadcastResultDelta(apiws.EventResultRetried, runID, []string{newResult.ID}, nil, nil)

	httpx.JSON(w, http.StatusCreated, map[string]interface{}{
		"id":             newResult.ID,
		"test_case_id":   newResult.TestCaseID,
		"attempt_number": newResult.AttemptNumber,
		"status":         newResult.Status,
	})
}
