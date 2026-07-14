package ai

import (
	"net/http"
	"time"

	"ttgo/internal/api/httpx"
)

// AIGenerationReportEndpoint serves the generation outcome/cost report.
//
// @Summary      AI generation summary report
// @Description  Aggregated run outcomes, draft feedback (accepted unchanged/edited, rejected with reasons, superseded), token/cost totals, and provider comparisons for a date window (default last 30 days).
// @Tags         ai-generations
// @Produce      json
// @Param        start_date  query  string  false  "YYYY-MM-DD"
// @Param        end_date    query  string  false  "YYYY-MM-DD (inclusive)"
// @Success      200  {object}  map[string]interface{}
// @Router       /ai-generations/reports/summary [get]
// @Security     BearerAuth
func (h *Handler) AIGenerationReportEndpoint(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	now := time.Now().UTC().Truncate(24 * time.Hour)
	start := now.AddDate(0, 0, -30)
	end := now.AddDate(0, 0, 1)
	if sd := q.Get("start_date"); sd != "" {
		if t, err := time.Parse("2006-01-02", sd); err == nil {
			start = t
		}
	}
	if ed := q.Get("end_date"); ed != "" {
		if t, err := time.Parse("2006-01-02", ed); err == nil {
			end = t.AddDate(0, 0, 1)
		}
	}
	rep, err := h.store.GetAIGenerationReport(start, end)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, rep)
}
