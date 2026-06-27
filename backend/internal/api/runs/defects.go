package runs

import (
	"encoding/json"
	"errors"
	"net/http"
	"ttgo/internal/api/httpx"
	apijira "ttgo/internal/api/integrations/jira"
	"ttgo/pkg/tracker/models"
)

func (h *Handler) ListRunDefectLinks(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")
	rows, err := h.store.ListDefectLinksByRun(runID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, rows)
}

func (h *Handler) ListResultDefectLinks(w http.ResponseWriter, r *http.Request) {
	resultID := r.PathValue("result_id")

	links, err := h.store.ListDefectLinksByRunResult(resultID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"links":         links,
		"run_result_id": resultID,
	})
}

func (h *Handler) LinkResultDefect(w http.ResponseWriter, r *http.Request) {
	resultID := r.PathValue("result_id")

	var rr models.RunResult
	if err := h.store.DB().Select("test_case_id").Where("id = ?", resultID).First(&rr).Error; err != nil {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "result not found"})
		return
	}
	testCaseID := ""
	if rr.TestCaseID != nil {
		testCaseID = *rr.TestCaseID
	}

	var req models.LinkDefectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.JiraIssueKey == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "jira_issue_key is required"})
		return
	}
	// Validate the key shape before it drives a server-side Jira fetch (F-069).
	if !apijira.ValidJiraKey(req.JiraIssueKey) {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid jira_issue_key format (expected e.g. PROJ-123)"})
		return
	}

	cfg, err := h.store.GetJiraConfig()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	var issue models.JiraIssueSummary
	if cfg != nil && cfg.Enabled {
		issue, err = h.store.GetJiraIssueSummary(cfg, req.JiraIssueKey)
		if err != nil {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
	} else {
		issue = models.JiraIssueSummary{Key: req.JiraIssueKey}
	}

	link, err := h.store.CreateDefectLinkForResult(resultID, testCaseID, req.JiraIssueKey, issue)
	if err != nil {
		if errors.Is(err, models.ErrDuplicateDefectLink) {
			httpx.JSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, link)
}

func (h *Handler) UnlinkResultDefect(w http.ResponseWriter, r *http.Request) {
	resultID := r.PathValue("result_id")
	jiraKey := r.PathValue("jiraKey")

	if err := h.store.DeleteDefectLinkByResult(resultID, jiraKey); err != nil {
		if err.Error() == "defect link not found" {
			httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "defect link not found"})
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
