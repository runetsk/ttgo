package tests

import (
	"net/http"
	"ttgo/internal/api/httpx"
)

func (h *Handler) GetQTestMapping(w http.ResponseWriter, r *http.Request) {
	testCaseID := r.PathValue("id")
	if testCaseID == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "test case ID is required"})
		return
	}

	mapping, err := h.store.GetQTestMappingByTestCase(testCaseID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if mapping == nil {
		httpx.JSON(w, http.StatusOK, map[string]interface{}{"linked": false})
		return
	}
	httpx.JSON(w, http.StatusOK, mapping)
}

func (h *Handler) UnlinkQTestMapping(w http.ResponseWriter, r *http.Request) {
	testCaseID := r.PathValue("id")
	if testCaseID == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "test case ID is required"})
		return
	}

	if err := h.store.DeleteQTestMapping(testCaseID); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "unlinked"})
}
