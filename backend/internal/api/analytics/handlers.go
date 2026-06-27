package analytics

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"
	"ttgo/internal/api/httpx"
	"ttgo/pkg/tracker/store"
)

// Handler serves analytics endpoints.
type Handler struct {
	store *store.Store
}

func NewHandler(s *store.Store) *Handler {
	return &Handler{store: s}
}

// analyticsFilters holds parsed analytics query parameters.
type analyticsFilters struct {
	StartDate time.Time
	EndDate   time.Time
	FolderID  string
}

// parseAnalyticsFilters extracts start_date, end_date, and folder_id from query params.
// Defaults: start_date = 30 days ago, end_date = today, folder_id = "" (all).
func parseAnalyticsFilters(r *http.Request) analyticsFilters {
	q := r.URL.Query()
	now := time.Now().UTC().Truncate(24 * time.Hour)

	f := analyticsFilters{
		StartDate: now.AddDate(0, 0, -30),
		EndDate:   now.AddDate(0, 0, 1), // end of today
		FolderID:  q.Get("folder_id"),
	}

	if sd := q.Get("start_date"); sd != "" {
		if t, err := time.Parse("2006-01-02", sd); err == nil {
			f.StartDate = t
		}
	}
	if ed := q.Get("end_date"); ed != "" {
		if t, err := time.Parse("2006-01-02", ed); err == nil {
			f.EndDate = t.AddDate(0, 0, 1) // inclusive end
		}
	}

	return f
}

// handleAnalyticsSummary handles GET /api/analytics/summary
//
// @Summary      Get analytics summary
// @Description  Returns an aggregated summary of test run analytics
// @Tags         analytics
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "Analytics summary object"
// @Failure      500  {object}  map[string]interface{}
// @Router       /analytics/summary [get]
func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	f := parseAnalyticsFilters(r)
	summary, err := h.store.GetAnalyticsSummary(f.StartDate, f.EndDate, f.FolderID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get analytics summary", "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, summary)
}

// handleAnalyticsTrend handles GET /api/analytics/trend?days=<n>
//
// @Summary      Get trend data
// @Description  Returns time-series trend data for test runs over a given number of days
// @Tags         analytics
// @Produce      json
// @Param        days  query     int  false  "Number of days to include in the trend"  default(30)
// @Success      200   {object}  map[string]interface{}  "Trend points array and days count"
// @Failure      500   {object}  map[string]interface{}
// @Router       /analytics/trend [get]
func (h *Handler) Trend(w http.ResponseWriter, r *http.Request) {
	f := parseAnalyticsFilters(r)

	// Support legacy `days` param as shortcut if no explicit dates given
	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v > 0 {
			days = v
		}
	}
	// If no explicit start_date was provided, use days param
	if r.URL.Query().Get("start_date") == "" {
		now := time.Now().UTC().Truncate(24 * time.Hour)
		f.StartDate = now.AddDate(0, 0, -days)
	}

	points, err := h.store.GetTrendData(f.StartDate, f.EndDate, f.FolderID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get trend data", "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"points": points,
		"days":   days,
	})
}

// handleAnalyticsFlaky handles GET /api/analytics/flaky
// 012-analytics-refactor: Refactored to use status-switch methodology.
//
// @Summary      Detect flaky tests (switch method)
// @Description  Returns tests ranked by status-switch percentage
// @Tags         analytics
// @Produce      json
// @Param        lookback  query     int  false  "Number of recent runs to analyze per test"  default(30)
// @Param        limit     query     int  false  "Max results"  default(20)
// @Success      200       {object}  map[string]interface{}
// @Failure      500       {object}  map[string]interface{}
// @Router       /analytics/flaky [get]
func (h *Handler) Flaky(w http.ResponseWriter, r *http.Request) {
	f := parseAnalyticsFilters(r)
	lookback := 30
	if lb := r.URL.Query().Get("lookback"); lb != "" {
		if v, err := strconv.Atoi(lb); err == nil && v > 0 {
			lookback = v
		}
	}
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 50 {
			limit = v
		}
	}

	results, err := h.store.DetectFlakyTestsSwitchMethod(f.StartDate, f.EndDate, f.FolderID, lookback, limit)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to detect flaky tests", "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"flaky_tests": results,
		"total":       len(results),
	})
}

// ── 012-analytics-refactor: New handlers ──────────────────────────────────

// handleAnalyticsMostFailed handles GET /api/analytics/most-failed
func (h *Handler) MostFailed(w http.ResponseWriter, r *http.Request) {
	f := parseAnalyticsFilters(r)
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 50 {
			limit = v
		}
	}

	results, err := h.store.GetMostFailedTestCases(f.StartDate, f.EndDate, f.FolderID, limit)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get most failed test cases", "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"test_cases": results,
		"total":      len(results),
	})
}

// handleAnalyticsDuration handles GET /api/analytics/duration
func (h *Handler) Duration(w http.ResponseWriter, r *http.Request) {
	f := parseAnalyticsFilters(r)
	results, err := h.store.GetDurationTrend(f.StartDate, f.EndDate, f.FolderID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get duration trend", "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"points": results,
	})
}

// handleAnalyticsDurationTop handles GET /api/analytics/duration/top
func (h *Handler) DurationTop(w http.ResponseWriter, r *http.Request) {
	f := parseAnalyticsFilters(r)
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 50 {
			limit = v
		}
	}

	results, err := h.store.GetMostTimeConsumingTestCases(f.StartDate, f.EndDate, f.FolderID, limit)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get most time consuming test cases", "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"test_cases": results,
		"total":      len(results),
	})
}

// handleAnalyticsComponentHealth handles GET /api/analytics/component-health
func (h *Handler) ComponentHealth(w http.ResponseWriter, r *http.Request) {
	f := parseAnalyticsFilters(r)
	threshold := 80.0
	if t := r.URL.Query().Get("threshold"); t != "" {
		if v, err := strconv.ParseFloat(t, 64); err == nil && v >= 50 && v <= 100 {
			threshold = v
		}
	}

	results, err := h.store.GetComponentHealth(f.StartDate, f.EndDate, f.FolderID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get component health", "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	// Compute totals
	var totals struct {
		TotalTests   int     `json:"total_tests"`
		PassedCount  int     `json:"passed_count"`
		FailedCount  int     `json:"failed_count"`
		SkippedCount int     `json:"skipped_count"`
		PassingRate  float64 `json:"passing_rate"`
	}
	for _, c := range results {
		totals.TotalTests += c.TotalTests
		totals.PassedCount += c.PassedCount
		totals.FailedCount += c.FailedCount
		totals.SkippedCount += c.SkippedCount
	}
	if totals.TotalTests > 0 {
		totals.PassingRate = float64(totals.PassedCount) / float64(totals.TotalTests) * 100
	}

	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"components": results,
		"threshold":  threshold,
		"totals":     totals,
	})
}

// handleAnalyticsGrowth handles GET /api/analytics/growth
func (h *Handler) Growth(w http.ResponseWriter, r *http.Request) {
	f := parseAnalyticsFilters(r)
	// Note: folder_id is ignored for growth (test cases aren't folder-scoped)
	results, err := h.store.GetTestCaseGrowth(f.StartDate, f.EndDate)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get test case growth", "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"points": results,
	})
}

// handleAnalyticsPassingRate handles GET /api/analytics/passing-rate
func (h *Handler) PassingRate(w http.ResponseWriter, r *http.Request) {
	f := parseAnalyticsFilters(r)
	excludeSkipped := r.URL.Query().Get("exclude_skipped") == "true"

	results, err := h.store.GetPassingRatePerFolder(f.StartDate, f.EndDate, f.FolderID, excludeSkipped)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get passing rate per folder", "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"folders": results,
	})
}

// handleAnalyticsActivity handles GET /api/analytics/activity
func (h *Handler) Activity(w http.ResponseWriter, r *http.Request) {
	f := parseAnalyticsFilters(r)
	limit := 30
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}

	results, err := h.store.GetRecentActivity(f.StartDate, f.EndDate, limit)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get activity log", "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"activities": results,
		"total":      len(results),
	})
}

// handleAnalyticsUniqueBugs handles GET /api/analytics/unique-bugs
func (h *Handler) UniqueBugs(w http.ResponseWriter, r *http.Request) {
	f := parseAnalyticsFilters(r)
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}

	results, err := h.store.GetUniqueBugs(f.StartDate, f.EndDate, limit)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get unique bugs", "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"bugs":  results,
		"total": len(results),
	})
}

// handleAnalyticsCompareRuns handles GET /api/analytics/compare-runs?run1=ID&run2=ID
func (h *Handler) CompareRuns(w http.ResponseWriter, r *http.Request) {
	run1 := r.URL.Query().Get("run1")
	run2 := r.URL.Query().Get("run2")
	if run1 == "" || run2 == "" {
		httpx.Error(w, http.StatusBadRequest, fmt.Errorf("both run1 and run2 query params are required"))
		return
	}

	result, err := h.store.CompareRuns(run1, run2)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to compare runs", "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}
