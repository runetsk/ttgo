package tests

import (
	"net/http"
	"ttgo/internal/api/httpx"
)

func (h *Handler) ListTestExecutions(w http.ResponseWriter, r *http.Request) {
	testCaseID := r.PathValue("id")
	rows, err := h.store.ListRecentResultsForTestCase(testCaseID, 10)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, rows)
}
