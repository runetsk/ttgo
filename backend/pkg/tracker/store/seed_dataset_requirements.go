package store

import (
	"time"
	"ttgo/pkg/tracker/models"
)

type demoRequirementData struct {
	Requirements     []models.Requirement
	RequirementLinks []models.RequirementTestCaseLink
}

func buildDemoRequirements(now time.Time, catalog demoCatalogData) demoRequirementData {
	req1 := demoID("req:ec-001")
	req2 := demoID("req:ec-002")
	req3 := demoID("req:ec-003")
	req4 := demoID("req:ec-004")
	req5 := demoID("req:ec-005")
	req6 := demoID("req:ec-006")
	req7 := demoID("req:ec-007")
	req8 := demoID("req:ec-008")
	req9 := demoID("req:ec-009")
	req10 := demoID("req:ec-010")

	requirements := []models.Requirement{
		{ID: req1, Identifier: "EC-001", Title: "User Authentication", Description: "Users must be able to log in and out of the application securely using email and password.", SourceType: "jira", SourceKey: "ECOM-101", SourceURL: "https://demo.atlassian.net/browse/ECOM-101", CreatedAt: now, UpdatedAt: now},
		{ID: req2, Identifier: "EC-002", Title: "Form Input Validation", Description: "All login and registration inputs must be validated before submission, with clear error messages.", SourceType: "jira", SourceKey: "ECOM-102", SourceURL: "https://demo.atlassian.net/browse/ECOM-102", CreatedAt: now, UpdatedAt: now},
		{ID: req3, Identifier: "EC-003", Title: "Session Management", Description: "User sessions must expire after a period of inactivity and handle concurrent logins per policy.", SourceType: "jira", SourceKey: "ECOM-103", SourceURL: "https://demo.atlassian.net/browse/ECOM-103", CreatedAt: now, UpdatedAt: now},
		{ID: req4, Identifier: "EC-004", Title: "Account Security", Description: "Accounts must support self-service password reset and be locked after repeated failed login attempts.", CreatedAt: now, UpdatedAt: now},
		{ID: req5, Identifier: "EC-005", Title: "Shopping Cart", Description: "Authenticated users can add, view, and remove items from their persistent shopping cart.", CreatedAt: now, UpdatedAt: now},
		{ID: req6, Identifier: "EC-006", Title: "Payment Processing", Description: "The checkout flow must process valid card payments, apply discount codes, and block purchase of out-of-stock items.", SourceType: "confluence", SourceKey: "12345001", SourceURL: "https://demo.atlassian.net/wiki/spaces/ECOM/pages/12345001", CreatedAt: now, UpdatedAt: now},
		{ID: req7, Identifier: "EC-007", Title: "SEO & Product Discoverability", Description: "All key pages must include well-formed meta tags (title, description, og:*) for search engine indexing.", SourceType: "confluence", SourceKey: "12345002", SourceURL: "https://demo.atlassian.net/wiki/spaces/ECOM/pages/12345002", CreatedAt: now, UpdatedAt: now},
		{ID: req8, Identifier: "EC-008", Title: "Product Search & Filtering", Description: "Users can search by keyword and narrow results by category using the search interface.", CreatedAt: now, UpdatedAt: now},
		{ID: req9, Identifier: "EC-009", Title: "Error Handling & Edge Cases", Description: "The application must display a custom 404 page for unknown routes and handle special-character queries gracefully.", CreatedAt: now, UpdatedAt: now},
		{ID: req10, Identifier: "EC-010", Title: "Performance Benchmarks", Description: "The homepage must load and become interactive within the defined performance budget on all supported devices.", CreatedAt: now, UpdatedAt: now},
	}

	mkLink := func(reqID, tcKey string) models.RequirementTestCaseLink {
		tcID := catalog.TestCasesByKey[tcKey].ID
		return models.RequirementTestCaseLink{
			ID:            demoID("link:" + reqID + ":" + tcID),
			RequirementID: reqID,
			TestCaseID:    tcID,
			CreatedAt:     now,
		}
	}

	requirementLinks := []models.RequirementTestCaseLink{
		mkLink(req1, "tc5"), mkLink(req1, "tc8"),
		mkLink(req2, "tc6"), mkLink(req2, "tc7"),
		mkLink(req3, "tc9"), mkLink(req3, "tc12"),
		mkLink(req4, "tc10"), mkLink(req4, "tc11"),
		mkLink(req5, "tc13"), mkLink(req5, "tc14"), mkLink(req5, "tc16"),
		mkLink(req6, "tc15"), mkLink(req6, "tc17"), mkLink(req6, "tc18"), mkLink(req6, "tc19"),
		mkLink(req8, "tc20"), mkLink(req8, "tc21"), mkLink(req8, "tc23"),
		mkLink(req9, "tc4"), mkLink(req9, "tc22"), mkLink(req9, "tc24"),
	}

	return demoRequirementData{
		Requirements:     requirements,
		RequirementLinks: requirementLinks,
	}
}
