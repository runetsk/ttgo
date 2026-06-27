package store

import (
	"time"
	"ttgo/pkg/tracker/models"
)

// buildDemoDefectLinks returns a small set of DefectLink rows attached to
// failing RunResults so the demo exercises the Jira-integration surface
// (link listing, status categories, reverification flag, write-back queue).
func buildDemoDefectLinks(now time.Time) []models.DefectLink {
	synced := now.Add(-15 * time.Minute)

	links := []models.DefectLink{
		{
			ID:                demoID("defect:ECOM-701"),
			TestCaseID:        demoID("tc:session-expires"),
			RunResultID:       ptr(demoID("rr:run2-" + demoID("tc:session-expires"))),
			JiraIssueKey:      "ECOM-701",
			LastKnownSummary:  "Dashboard header missing after login on staging",
			LastKnownStatus:   "In Progress",
			LastKnownPriority: "High",
			LastKnownAssignee: "Priya Patel",
			LastKnownURL:      "https://demo.atlassian.net/browse/ECOM-701",
			StatusCategory:    "indeterminate",
			LastSyncedAt:      &synced,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			ID:                demoID("defect:ECOM-702"),
			TestCaseID:        demoID("tc:checkout-valid-payment"),
			RunResultID:       ptr(demoID("rr:run2-" + demoID("tc:checkout-valid-payment"))),
			JiraIssueKey:      "ECOM-702",
			LastKnownSummary:  "Payment iframe times out on Firefox",
			LastKnownStatus:   "To Do",
			LastKnownPriority: "Medium",
			LastKnownAssignee: "Alex Kim",
			LastKnownURL:      "https://demo.atlassian.net/browse/ECOM-702",
			StatusCategory:    "todo",
			LastSyncedAt:      &synced,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			ID:                 demoID("defect:ECOM-703"),
			TestCaseID:         demoID("tc:account-lockout"),
			RunResultID:        ptr(demoID("rr:run3-" + demoID("tc:account-lockout"))),
			JiraIssueKey:       "ECOM-703",
			LastKnownSummary:   "Account-lockout banner fails to render under load",
			LastKnownStatus:    "In Progress",
			LastKnownPriority:  "High",
			LastKnownAssignee:  "Marcus Lee",
			LastKnownURL:       "https://demo.atlassian.net/browse/ECOM-703",
			StatusCategory:     "indeterminate",
			CommentPending:     true,
			PendingCommentText: "TTGO run failed: staging · 2026-04-22 · https://demo.ttgo.test/runs/sprint1-edge",
			LastSyncedAt:       &synced,
			CreatedAt:          now,
			UpdatedAt:          now,
		},
		{
			ID:                demoID("defect:ECOM-650"),
			TestCaseID:        demoID("tc:category-filter"),
			RunResultID:       ptr(demoID("rr:run6-" + demoID("tc:category-filter"))),
			JiraIssueKey:      "ECOM-650",
			LastKnownSummary:  "Search service returns 504 under high concurrency",
			LastKnownStatus:   "Done",
			LastKnownPriority: "Medium",
			LastKnownAssignee: "Priya Patel",
			LastKnownURL:      "https://demo.atlassian.net/browse/ECOM-650",
			StatusCategory:    "done",
			LastSyncedAt:      &synced,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			ID:                demoID("defect:ECOM-712"),
			TestCaseID:        demoID("tc:category-filter"),
			JiraIssueKey:      "ECOM-712",
			LastKnownSummary:  "Category-filter SQL regression in v2026.04.5",
			LastKnownStatus:   "Done",
			LastKnownPriority: "High",
			LastKnownAssignee: "Alex Kim",
			LastKnownURL:      "https://demo.atlassian.net/browse/ECOM-712",
			StatusCategory:    "done",
			LastSyncedAt:      &synced,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
	}

	return links
}

// demoReverificationFlaggedTestCaseIDs lists test cases whose defects are all
// resolved ("done") so the demo can surface the reverification-needed flag.
func demoReverificationFlaggedTestCaseIDs() []string {
	// tc:category-filter has two linked defects and both are in StatusCategory=done.
	return []string{demoID("tc:category-filter")}
}
