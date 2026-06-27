package requirements

import (
	"net/http"
	"strings"
	"ttgo/internal/api/httpx"
)

// handleTraceabilityMatrix godoc
// @Summary      Get the traceability matrix
// @Description  Returns all requirements with their linked test cases and an aggregate coverage summary. Optional ?q= text filter and ?uncovered=true gap filter are applied server-side for API consumers; the React UI filters client-side.
// @Tags         traceability
// @Produce      json
// @Param        q          query     string  false  "Case-insensitive substring filter on identifier and title"
// @Param        uncovered  query     boolean false  "If true, return only requirements with no linked test cases"
// @Success      200  {object}  object
// @Failure      500  {object}  map[string]string
// @Router       /traceability [get]
func (h *Handler) TraceabilityMatrix(w http.ResponseWriter, r *http.Request) {
	matrix, err := h.store.GetTraceabilityMatrix()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	// Optional server-side filters (for API consumers — React UI uses client-side filtering).
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	uncoveredOnly := r.URL.Query().Get("uncovered") == "true"

	if q != "" || uncoveredOnly {
		filtered := matrix.Rows[:0]
		for _, row := range matrix.Rows {
			if uncoveredOnly && row.Covered {
				continue
			}
			if q != "" {
				if !strings.Contains(strings.ToLower(row.Identifier), q) &&
					!strings.Contains(strings.ToLower(row.Title), q) {
					continue
				}
			}
			filtered = append(filtered, row)
		}
		matrix.Rows = filtered
	}

	httpx.JSON(w, http.StatusOK, matrix)
}
