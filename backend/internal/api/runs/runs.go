package runs

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"ttgo/internal/api/httpx"
	apiws "ttgo/internal/api/websocket"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"

	"gorm.io/gorm"
)

func (h *Handler) CreateTestRun(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CategoryID  *string `json:"category_id"`
		Name        string  `json:"name"`
		RunFolderID *string `json:"run_folder_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	if req.CategoryID != nil && *req.CategoryID == "" {
		req.CategoryID = nil
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
	if err := h.store.CreateTestRun(run); err != nil {
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
		switch models.ExecutionStatus(*req.Status) {
		case models.StatusFail:
			if req.DefectType == nil {
				updateMap["defect_type"] = "to_investigate"
			}
		default:
			updateMap["defect_type"] = ""
		}
	}
	if req.DefectType != nil {
		updateMap["defect_type"] = *req.DefectType
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
	updateMap["updated_at"] = time.Now()

	if err := h.store.UpdateRunResult(runID, resultID, updateMap); err != nil {
		slog.ErrorContext(r.Context(), "failed to update result", "result_id", resultID, "run_id", runID, "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	h.store.TouchTestRun(runID)

	var warnings []models.RunResultUpdateWarning
	if req.Status != nil && models.ExecutionStatus(*req.Status) == models.StatusPass {
		var rr models.RunResult
		if err := h.store.DB().Select("test_case_id").Where("id = ?", resultID).First(&rr).Error; err == nil && rr.TestCaseID != nil {
			warnings = h.triggerJiraWriteBack(r, runID, *rr.TestCaseID)
		}
	}

	if fullRun, err := h.store.GetTestRun(runID); err == nil && fullRun != nil && h.hub != nil {
		h.hub.Broadcast(apiws.NewEvent(apiws.EventResultUpdated, "run:"+runID, fullRun))
	}

	if len(warnings) > 0 {
		httpx.JSON(w, http.StatusOK, map[string]interface{}{"status": "updated", "warnings": warnings})
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *Handler) triggerJiraWriteBack(r *http.Request, runID, testCaseID string) []models.RunResultUpdateWarning {
	cfg, err := h.store.GetJiraConfig()
	if err != nil || cfg == nil || !cfg.Enabled {
		return nil
	}

	links, err := h.store.ListDefectLinksByTestCase(testCaseID)
	if err != nil || len(links) == 0 {
		return nil
	}

	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	runURL := fmt.Sprintf("%s://%s/runs/%s", scheme, r.Host, runID)

	tc, _ := h.store.GetTestCase(testCaseID)
	testName := testCaseID
	if tc != nil {
		testName = tc.Name
	}

	var warnings []models.RunResultUpdateWarning
	for _, link := range links {
		if warn := h.store.WriteBackComment(cfg, testCaseID, link.JiraIssueKey, testName, runURL); warn != "" {
			warnings = append(warnings, models.RunResultUpdateWarning{
				JiraIssueKey: link.JiraIssueKey,
				Message:      warn,
			})
		}
	}
	return warnings
}

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

	result := req.ToRunResult(runID)
	if err := h.store.AddRunResult(result); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			httpx.JSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		slog.ErrorContext(r.Context(), "failed to add result to run", "run_id", runID, "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	if fullRun, err := h.store.GetTestRun(runID); err == nil && fullRun != nil && h.hub != nil {
		h.hub.Broadcast(apiws.NewEvent(apiws.EventResultUpdated, "run:"+runID, fullRun))
	}

	httpx.JSON(w, http.StatusCreated, result)
}

func (h *Handler) DeleteRunResult(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")
	resultID := r.PathValue("result_id")

	if err := h.store.DeleteRunResult(runID, resultID); err != nil {
		slog.ErrorContext(r.Context(), "failed to delete result from run", "result_id", resultID, "run_id", runID, "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	if fullRun, err := h.store.GetTestRun(runID); err == nil && fullRun != nil && h.hub != nil {
		h.hub.Broadcast(apiws.NewEvent(apiws.EventResultDeleted, "run:"+runID, fullRun))
	}

	w.WriteHeader(http.StatusNoContent)
}

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
	if req.Status == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "status is required"})
		return
	}
	if !models.IsValidExecutionStatus(req.Status) {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid status"})
		return
	}

	updateMap := map[string]interface{}{
		"status":     req.Status,
		"updated_at": time.Now(),
	}
	switch status := models.ExecutionStatus(req.Status); status {
	case models.StatusFail:
		if req.DefectType != "" {
			updateMap["defect_type"] = req.DefectType
		} else {
			updateMap["defect_type"] = "to_investigate"
		}
	default:
		updateMap["defect_type"] = ""
	}

	if err := h.store.BulkUpdateRunResults(runID, req.ResultIDs, updateMap); err != nil {
		slog.ErrorContext(r.Context(), "failed to bulk update results in run", "run_id", runID, "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	h.store.TouchTestRun(runID)

	var allWarnings []models.RunResultUpdateWarning
	if models.ExecutionStatus(req.Status) == models.StatusPass {
		for _, resultID := range req.ResultIDs {
			var rr models.RunResult
			if err := h.store.DB().Select("test_case_id").Where("id = ?", resultID).First(&rr).Error; err == nil && rr.TestCaseID != nil {
				allWarnings = append(allWarnings, h.triggerJiraWriteBack(r, runID, *rr.TestCaseID)...)
			}
		}
	}

	if fullRun, err := h.store.GetTestRun(runID); err == nil && fullRun != nil && h.hub != nil {
		h.hub.Broadcast(apiws.NewEvent(apiws.EventResultBulkUpdated, "run:"+runID, fullRun))
	}

	resp := map[string]interface{}{
		"status":  "updated",
		"updated": len(req.ResultIDs),
	}
	if len(allWarnings) > 0 {
		resp["warnings"] = allWarnings
	}
	httpx.JSON(w, http.StatusOK, resp)
}

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

	if fullRun, err := h.store.GetTestRun(runID); err == nil && fullRun != nil && h.hub != nil {
		h.hub.Broadcast(apiws.NewEvent(apiws.EventResultRetried, "run:"+runID, fullRun))
	}

	httpx.JSON(w, http.StatusCreated, map[string]interface{}{
		"id":             newResult.ID,
		"test_case_id":   newResult.TestCaseID,
		"attempt_number": newResult.AttemptNumber,
		"status":         newResult.Status,
	})
}
