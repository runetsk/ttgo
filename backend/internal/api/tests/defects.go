package tests

import (
	"encoding/json"
	"net/http"
	"ttgo/internal/api/httpx"
	"ttgo/pkg/tracker/models"
)

func (h *Handler) CreateJiraIssue(w http.ResponseWriter, r *http.Request) {
	testCaseID := r.PathValue("id")

	var req models.CreateJiraIssueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Summary == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "summary is required"})
		return
	}
	// Validate the test case exists before spending the org's Jira quota (F-009).
	if _, err := h.store.GetTestCase(testCaseID); err != nil {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "test case not found"})
		return
	}

	link, err := h.store.CreateJiraIssueAndLink(testCaseID, req)
	if err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	httpx.JSON(w, http.StatusCreated, link)
}

func (h *Handler) DismissReverification(w http.ResponseWriter, r *http.Request) {
	testCaseID := r.PathValue("id")
	if err := h.store.DismissReverification(testCaseID); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
