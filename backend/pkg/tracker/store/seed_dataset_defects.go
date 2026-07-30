package store

import (
	"time"

	"ttgo/pkg/tracker/models"
)

// staleDemoDefectAgeDays backdates defect:1's updated_at so the register's
// "Stale · 7d+" tile has something in it. Comfortably past the frontend's 7-day
// threshold (utils/defectQueue.js STALE_DAYS) without looking abandoned.
const staleDemoDefectAgeDays = 11

// buildDemoDefects returns native demo defects + links.
//
// The four defects are chosen so the Defects register's triage tiles and status
// tabs are all non-empty on a fresh seed — an all-unassigned, all-open set makes
// Owner, In progress and Fixed look like dead features to anyone trying the demo:
//
//   - defect:1 open, unassigned, critical → Needs triage (and Critical open)
//   - defect:2 open, assigned             → In progress
//   - defect:3 closed                      → Closed
//   - defect:4 fixed, assigned             → Fixed, awaiting retest
//
// assigneeID is the user resolved at seed time (see resolveSeedUser); when no user
// exists yet it is empty and every defect simply seeds unassigned.
//
// Link targets are verified against the demo dataset:
//   - tc1 = demoID("tc:session-expires")   → run2 member at i=2 (i%3==2 → FAIL)
//   - tc2 = demoID("tc:checkout-valid-payment") → run2 member at i=5 (i%3==2 → FAIL)
//   - rr1 = demoID("rr:run2-"+tc1)         → enriched failure in assertionStory
//   - rr2 = demoID("rr:run2-"+tc2)         → enriched failure in timeoutStory
//   - tc3 = demoID("tc:category-filter")   → test case; link is case-scoped (closed defect)
//
// defect:4 is linked to tc2, which also carries the still-open defect:2, so that
// test case stays un-flagged for reverification: the flag means *every* linked
// defect is fixed-or-closed, and pinning that here keeps the demo honest.
func buildDemoDefects(now time.Time, assigneeID string) ([]models.Defect, []models.DefectLink) {
	var assignee *string
	if assigneeID != "" {
		assignee = &assigneeID
	}

	defects := []models.Defect{
		{
			ID: demoID("defect:1"), Title: "Dashboard header missing after login",
			Status: "open", Severity: "critical",
			ExternalProvider: "Jira", ExternalKey: "ECOM-701",
			ExternalURL: "https://demo.atlassian.net/browse/ECOM-701",
			CreatedAt:   now, UpdatedAt: now,
		},
		{
			ID: demoID("defect:2"), Title: "Payment iframe times out on Firefox",
			Status: "open", Severity: "minor", AssigneeID: assignee,
			CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: demoID("defect:3"), Title: "Category-filter SQL regression",
			Status: "closed", Severity: "major",
			CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: demoID("defect:4"), Title: "Checkout total rounds down on multi-currency carts",
			Status: "fixed", Severity: "major", AssigneeID: assignee,
			ExternalProvider: "Jira", ExternalKey: "ECOM-844",
			ExternalURL: "https://demo.atlassian.net/browse/ECOM-844",
			CreatedAt:   now, UpdatedAt: now,
		},
	}

	tc1 := demoID("tc:session-expires")
	tc2 := demoID("tc:checkout-valid-payment")
	tc3 := demoID("tc:category-filter")
	rr1 := demoID("rr:run2-" + tc1)
	rr2 := demoID("rr:run2-" + tc2)

	links := []models.DefectLink{
		// result-scoped: open defect linked to tc:session-expires failure in run2
		{ID: demoID("dl:1"), DefectID: defects[0].ID, TestCaseID: &tc1, RunResultID: &rr1, CreatedAt: now},
		// result-scoped: open defect linked to tc:checkout-valid-payment failure in run2
		{ID: demoID("dl:2"), DefectID: defects[1].ID, TestCaseID: &tc2, RunResultID: &rr2, CreatedAt: now},
		// case-scoped: closed defect linked to tc:category-filter (triggers reverification flag)
		{ID: demoID("dl:3"), DefectID: defects[2].ID, TestCaseID: &tc3, CreatedAt: now},
		// result-scoped: fixed defect on the same result as dl:2, so tc2 carries one
		// open and one fixed defect and stays un-flagged for reverification
		{ID: demoID("dl:4"), DefectID: defects[3].ID, TestCaseID: &tc2, RunResultID: &rr2, CreatedAt: now},
	}

	return defects, links
}
