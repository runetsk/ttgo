package store

import (
	"time"
	"ttgo/pkg/tracker/models"
)

// buildDemoComments returns a handful of Comment rows attached to demo runs
// and failing run results so the demo exercises the collaboration/triage UI.
// userID is the author resolved at seed time (no deterministic user exists in
// the demo dataset — the seeder picks an existing admin).
func buildDemoComments(now time.Time, userID string) []models.Comment {
	if userID == "" {
		return nil
	}

	return []models.Comment{
		{
			ID:         demoID("comment:run2:triage"),
			TargetType: "run",
			TargetID:   demoID("run:sprint1-regression"),
			UserID:     userID,
			Content:    "Three failures in this run — auth and checkout. Filed ECOM-701 and ECOM-702. Rerunning checkout after payments vendor deploy.",
			CreatedAt:  now.Add(-40 * time.Minute),
			UpdatedAt:  now.Add(-40 * time.Minute),
		},
		{
			ID:         demoID("comment:rr:run2:tc9"),
			TargetType: "result",
			TargetID:   demoID("rr:run2-" + demoID("tc:session-expires")),
			UserID:     userID,
			Content:    "Reproduced locally. Dashboard header unmounts when session cookie is refreshed mid-render. Assigning to @priya.",
			CreatedAt:  now.Add(-35 * time.Minute),
			UpdatedAt:  now.Add(-35 * time.Minute),
		},
		{
			ID:         demoID("comment:rr:run3:tc11"),
			TargetType: "result",
			TargetID:   demoID("rr:run3-" + demoID("tc:account-lockout")),
			UserID:     userID,
			Content:    "AI flagged this as flaky — confirmed in v2 analysis. Bumping selector timeout in next PR.",
			CreatedAt:  now.Add(-12 * time.Minute),
			UpdatedAt:  now.Add(-12 * time.Minute),
		},
		{
			ID:         demoID("comment:run6:followup"),
			TargetType: "run",
			TargetID:   demoID("run:sprint2-full"),
			UserID:     userID,
			Content:    "Search cluster outage confirmed by infra — not a product regression. Closed ECOM-650 and ECOM-712.",
			CreatedAt:  now.Add(-5 * time.Minute),
			UpdatedAt:  now.Add(-5 * time.Minute),
		},
	}
}
