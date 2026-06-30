package store

import (
	"time"

	"ttgo/pkg/tracker/models"
)

// buildDemoDefects returns native demo defects + links.
//
// Link targets are verified against the demo dataset:
//   - tc1 = demoID("tc:session-expires")   → run2 member at i=2 (i%3==2 → FAIL)
//   - tc2 = demoID("tc:checkout-valid-payment") → run2 member at i=5 (i%3==2 → FAIL)
//   - rr1 = demoID("rr:run2-"+tc1)         → enriched failure in assertionStory
//   - rr2 = demoID("rr:run2-"+tc2)         → enriched failure in timeoutStory
//   - tc3 = demoID("tc:category-filter")   → test case; link is case-scoped (closed defect)
func buildDemoDefects(now time.Time) ([]models.Defect, []models.DefectLink) {
	defects := []models.Defect{
		{
			ID: demoID("defect:1"), Title: "Dashboard header missing after login",
			Status: "open", Severity: "major",
			ExternalProvider: "Jira", ExternalKey: "ECOM-701",
			ExternalURL: "https://demo.atlassian.net/browse/ECOM-701",
			CreatedAt:   now, UpdatedAt: now,
		},
		{
			ID: demoID("defect:2"), Title: "Payment iframe times out on Firefox",
			Status: "open", Severity: "minor",
			CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: demoID("defect:3"), Title: "Category-filter SQL regression",
			Status: "closed", Severity: "major",
			CreatedAt: now, UpdatedAt: now,
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
	}

	return defects, links
}
