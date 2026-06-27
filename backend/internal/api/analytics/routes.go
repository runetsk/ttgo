package analytics

import (
	"ttgo/internal/api/routing"

	"github.com/go-pkgz/routegroup"
)

func Mount(api *routegroup.Bundle, h *Handler, requireAuth routing.AuthMiddleware) {
	api.HandleFunc("GET /analytics/summary", requireAuth("read", h.Summary))
	api.HandleFunc("GET /analytics/trend", requireAuth("read", h.Trend))
	api.HandleFunc("GET /analytics/flaky", requireAuth("read", h.Flaky))
	api.HandleFunc("GET /analytics/most-failed", requireAuth("read", h.MostFailed))
	api.HandleFunc("GET /analytics/duration", requireAuth("read", h.Duration))
	api.HandleFunc("GET /analytics/duration/top", requireAuth("read", h.DurationTop))
	api.HandleFunc("GET /analytics/component-health", requireAuth("read", h.ComponentHealth))
	api.HandleFunc("GET /analytics/growth", requireAuth("read", h.Growth))
	api.HandleFunc("GET /analytics/passing-rate", requireAuth("read", h.PassingRate))
	api.HandleFunc("GET /analytics/compare-runs", requireAuth("read", h.CompareRuns))
	api.HandleFunc("GET /analytics/unique-bugs", requireAuth("read", h.UniqueBugs))
	api.HandleFunc("GET /analytics/activity", requireAuth("read", h.Activity))
}
