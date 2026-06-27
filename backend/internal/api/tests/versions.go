package tests

import (
	"log"
	"net/http"
	"ttgo/internal/api/authctx"
	"ttgo/internal/api/httpx"
)

// handleListVersions returns all version history entries for a test case, newest first.
// GET /api/tests/{id}/versions
func (h *Handler) ListVersions(w http.ResponseWriter, r *http.Request) {
	testCaseID := r.PathValue("id")
	versions, err := h.store.ListVersions(testCaseID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	log.Printf("versions list: testCaseID=%s count=%d", testCaseID, len(versions))
	httpx.JSON(w, http.StatusOK, versions)
}

// handleGetVersion returns a single version entry for a test case.
// GET /api/tests/{id}/versions/{vid}
func (h *Handler) GetVersion(w http.ResponseWriter, r *http.Request) {
	testCaseID := r.PathValue("id")
	versionID := r.PathValue("vid")
	v, err := h.store.GetVersion(testCaseID, versionID)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, err)
		return
	}
	httpx.JSON(w, http.StatusOK, v)
}

// handleRestoreVersion restores a test case to a previous version snapshot.
// POST /api/tests/{id}/versions/{vid}/restore
func (h *Handler) RestoreVersion(w http.ResponseWriter, r *http.Request) {
	testCaseID := r.PathValue("id")
	versionID := r.PathValue("vid")

	var userID, userName string
	if u := authctx.UserFromRequest(r); u != nil {
		userID = u.ID
		userName = u.DisplayName
	}

	updated, err := h.store.RestoreTestCase(testCaseID, versionID, userID, userName)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	log.Printf("versions restore: testCaseID=%s versionID=%s userID=%s", testCaseID, versionID, userID)
	httpx.JSON(w, http.StatusOK, updated)
}
