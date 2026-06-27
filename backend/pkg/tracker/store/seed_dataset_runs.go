package store

import (
	"time"
	"ttgo/pkg/tracker/models"
)

type demoRunData struct {
	RunFolders []models.RunFolder
	TestRuns   []models.TestRun
	RunResults []models.RunResult
}

func buildDemoRuns(now time.Time, catalog demoCatalogData) demoRunData {
	rf1ID := demoID("rf:sprint-1")
	rf2ID := demoID("rf:sprint-2")

	runFolders := []models.RunFolder{
		{ID: rf1ID, Name: "Sprint 1", DisplayOrder: 1, CreatedAt: now, UpdatedAt: now},
		{ID: rf2ID, Name: "Sprint 2", DisplayOrder: 2, CreatedAt: now, UpdatedAt: now},
	}

	run1ID := demoID("run:sprint1-smoke")
	run2ID := demoID("run:sprint1-regression")
	run3ID := demoID("run:sprint1-edge")
	run4ID := demoID("run:sprint2-smoke")
	run5ID := demoID("run:sprint2-regression")
	run6ID := demoID("run:sprint2-full")

	smokeID := catalog.CategoryIDs["smoke"]
	regressionID := catalog.CategoryIDs["regression"]
	edgeID := catalog.CategoryIDs["edge"]

	testRuns := []models.TestRun{
		{ID: run1ID, Name: "Sprint 1 – Smoke Run", CategoryID: &smokeID, RunFolderID: &rf1ID, Status: models.StatusPass, CreatedAt: now, UpdatedAt: now},
		{ID: run2ID, Name: "Sprint 1 – Regression Run", CategoryID: &regressionID, RunFolderID: &rf1ID, Status: models.StatusFail, CreatedAt: now, UpdatedAt: now},
		{ID: run3ID, Name: "Sprint 1 – Edge Cases Run", CategoryID: &edgeID, RunFolderID: &rf1ID, Status: models.StatusFail, CreatedAt: now, UpdatedAt: now},
		{ID: run4ID, Name: "Sprint 2 – Smoke Run", CategoryID: &smokeID, RunFolderID: &rf2ID, Status: models.StatusPass, CreatedAt: now, UpdatedAt: now},
		{ID: run5ID, Name: "Sprint 2 – Regression Run", CategoryID: &regressionID, RunFolderID: &rf2ID, Status: models.StatusFail, CreatedAt: now, UpdatedAt: now},
		{ID: run6ID, Name: "Sprint 2 – Full Suite Run", CategoryID: &regressionID, RunFolderID: &rf2ID, Status: models.StatusFail, CreatedAt: now, UpdatedAt: now},
	}

	smokeMembers := demoMembers(catalog, "tc5", "tc8", "tc13", "tc15", "tc20")
	regressionMembers := demoMembers(catalog, "tc5", "tc6", "tc9", "tc10", "tc14", "tc15", "tc17", "tc19", "tc21", "tc23")
	edgeMembers := demoMembers(catalog, "tc6", "tc7", "tc11", "tc12", "tc16", "tc18", "tc22", "tc24")

	startTime := now.Add(-10 * time.Minute)
	endTime := now.Add(-1 * time.Minute)

	var runResults []models.RunResult

	for i, m := range smokeMembers {
		id := m.ID
		runResults = append(runResults, models.RunResult{
			ID:               demoID("rr:run1-" + m.ID),
			TestRunID:        run1ID,
			TestCaseID:       &id,
			TestNameSnapshot: m.Name,
			Status:           models.StatusPass,
			DurationMs:       int64(300 + i*50),
			StartTime:        startTime,
			EndTime:          endTime,
		})
	}

	defectTypesRun2 := []string{"product_bug", "to_investigate", "automation_bug"}
	for i, m := range regressionMembers {
		id := m.ID
		status := models.StatusPass
		errMsg := ""
		failureType := ""
		defectType := ""
		if i%3 == 2 {
			status = models.StatusFail
			errMsg = "Assertion failed: expected element to be visible"
			failureType = "assertion"
			defectType = defectTypesRun2[(i/3)%len(defectTypesRun2)]
		}
		runResults = append(runResults, models.RunResult{
			ID:               demoID("rr:run2-" + m.ID),
			TestRunID:        run2ID,
			TestCaseID:       &id,
			TestNameSnapshot: m.Name,
			Status:           status,
			DurationMs:       int64(200 + i*70),
			StartTime:        startTime,
			EndTime:          endTime,
			ErrorMessage:     errMsg,
			FailureType:      failureType,
			DefectType:       defectType,
		})
	}

	defectTypesRun3 := []string{"system_issue", "to_investigate"}
	for i, m := range edgeMembers {
		id := m.ID
		status := models.StatusPass
		errMsg := ""
		failureType := ""
		defectType := ""
		switch i % 3 {
		case 1:
			status = models.StatusSkip
		case 2:
			status = models.StatusFail
			errMsg = "Timeout: element not found within 5000ms"
			failureType = "timeout"
			defectType = defectTypesRun3[(i/3)%len(defectTypesRun3)]
		}
		runResults = append(runResults, models.RunResult{
			ID:               demoID("rr:run3-" + m.ID),
			TestRunID:        run3ID,
			TestCaseID:       &id,
			TestNameSnapshot: m.Name,
			Status:           status,
			DurationMs:       int64(150 + i*60),
			StartTime:        startTime,
			EndTime:          endTime,
			ErrorMessage:     errMsg,
			FailureType:      failureType,
			DefectType:       defectType,
		})
	}

	for i, m := range smokeMembers {
		id := m.ID
		runResults = append(runResults, models.RunResult{
			ID:               demoID("rr:run4-" + m.ID),
			TestRunID:        run4ID,
			TestCaseID:       &id,
			TestNameSnapshot: m.Name,
			Status:           models.StatusPass,
			DurationMs:       int64(280 + i*40),
			StartTime:        startTime,
			EndTime:          endTime,
		})
	}

	for i, m := range regressionMembers {
		id := m.ID
		status := models.StatusPass
		errMsg := ""
		failureType := ""
		defectType := ""
		if i%4 == 3 {
			status = models.StatusFail
			errMsg = "Element not interactable: button is disabled"
			failureType = "assertion"
			defectType = "product_bug"
		}
		runResults = append(runResults, models.RunResult{
			ID:               demoID("rr:run5-" + m.ID),
			TestRunID:        run5ID,
			TestCaseID:       &id,
			TestNameSnapshot: m.Name,
			Status:           status,
			DurationMs:       int64(220 + i*55),
			StartTime:        startTime,
			EndTime:          endTime,
			ErrorMessage:     errMsg,
			FailureType:      failureType,
			DefectType:       defectType,
		})
	}

	for i, m := range regressionMembers {
		id := m.ID
		status := models.StatusPass
		errMsg := ""
		failureType := ""
		defectType := ""
		switch i % 3 {
		case 1:
			status = models.StatusSkip
		case 2:
			status = models.StatusFail
			errMsg = "Network error: request timed out after 30s"
			failureType = "network"
			defectType = "system_issue"
		}
		runResults = append(runResults, models.RunResult{
			ID:               demoID("rr:run6-" + m.ID),
			TestRunID:        run6ID,
			TestCaseID:       &id,
			TestNameSnapshot: m.Name,
			Status:           status,
			DurationMs:       int64(190 + i*65),
			StartTime:        startTime,
			EndTime:          endTime,
			ErrorMessage:     errMsg,
			FailureType:      failureType,
			DefectType:       defectType,
		})
	}

	runResults = enrichDemoRunResults(runResults)
	runResults = append(runResults, buildDemoRetryChain(now, run5ID, regressionMembers, startTime, endTime)...)

	return demoRunData{
		RunFolders: runFolders,
		TestRuns:   testRuns,
		RunResults: runResults,
	}
}

func demoMembers(catalog demoCatalogData, keys ...string) []demoTestCaseRef {
	refs := make([]demoTestCaseRef, 0, len(keys))
	for _, key := range keys {
		refs = append(refs, catalog.TestCasesByKey[key])
	}
	return refs
}

// runResultEnrichment carries the rich-failure artefacts applied to selected
// failing demo RunResults. It keeps enrichDemoRunResults readable.
type runResultEnrichment struct {
	Screenshots string
	Video       string
	TraceURL    string
	LogText     string
	StackTrace  string
	Steps       string
	Browser     string
	OS          string
	Environment string
	AppVersion  string
}

// enrichDemoRunResults fills in rich failure artefacts (screenshots, video, trace
// URL, log text, per-step breakdown) and execution context (browser/OS/environment)
// on selected failing results so the demo showcases the full RunResult schema.
func enrichDemoRunResults(results []models.RunResult) []models.RunResult {
	assertionStory := runResultEnrichment{
		Screenshots: `["https://demo.ttgo.test/artifacts/run2/login-before.png","https://demo.ttgo.test/artifacts/run2/login-failure.png"]`,
		Video:       "https://demo.ttgo.test/artifacts/run2/session-expires.mp4",
		TraceURL:    "https://demo.ttgo.test/artifacts/run2/session-expires.zip",
		LogText: "2026-04-22T09:14:02.103Z INFO  launching chromium\n" +
			"2026-04-22T09:14:02.894Z INFO  page.goto https://demo.shop.test/login\n" +
			"2026-04-22T09:14:07.911Z ERROR assertion failed: dashboard-header not visible\n",
		StackTrace: "AssertionError: expected element [data-testid=dashboard-header] to be visible\n" +
			"    at Object.<anonymous> (tests/auth/login.spec.ts:42:18)\n" +
			"    at processTicksAndRejections (node:internal/process/task_queues:95:5)",
		Steps: `[{"name":"Navigate to login page","status":"PASS","duration_ms":210},` +
			`{"name":"Enter valid credentials","status":"PASS","duration_ms":340},` +
			`{"name":"Assert dashboard is visible","status":"FAIL","duration_ms":5012,` +
			`"error":"expected element [data-testid=dashboard-header] to be visible"}]`,
		Browser: "Chrome 124", OS: "macOS 14.4", Environment: "staging", AppVersion: "2026.04.3",
	}
	timeoutStory := runResultEnrichment{
		Screenshots: `["https://demo.ttgo.test/artifacts/run2/checkout-timeout.png"]`,
		TraceURL:    "https://demo.ttgo.test/artifacts/run2/coupon.zip",
		LogText: "2026-04-22T09:18:10.001Z INFO  page.goto https://demo.shop.test/checkout\n" +
			"2026-04-22T09:18:15.002Z ERROR waitForSelector(#payment-frame) timed out after 5000ms\n",
		StackTrace: "TimeoutError: page.waitForSelector: Timeout 5000ms exceeded.\n" +
			"    at Page.waitForSelector (node_modules/playwright/lib/client/page.ts:880:15)\n" +
			"    at tests/checkout/payment.spec.ts:58:22",
		Steps: `[{"name":"Open checkout page","status":"PASS","duration_ms":180},` +
			`{"name":"Wait for payment iframe","status":"FAIL","duration_ms":5000,` +
			`"error":"Timeout 5000ms exceeded waiting for selector #payment-frame"}]`,
		Browser: "Firefox 125", OS: "Ubuntu 22.04", Environment: "staging", AppVersion: "2026.04.3",
	}
	lockoutStory := runResultEnrichment{
		Screenshots: `["https://demo.ttgo.test/artifacts/run3/account-lockout.png"]`,
		TraceURL:    "https://demo.ttgo.test/artifacts/run3/account-lockout.zip",
		LogText: "2026-04-22T09:18:10.001Z INFO  page.goto https://demo.shop.test/login\n" +
			"2026-04-22T09:18:15.002Z ERROR waitForSelector(#lockout-banner) timed out after 5000ms\n",
		StackTrace: "TimeoutError: page.waitForSelector: Timeout 5000ms exceeded.\n" +
			"    at Page.waitForSelector (node_modules/playwright/lib/client/page.ts:880:15)\n" +
			"    at tests/auth/lockout.spec.ts:31:22",
		Steps: `[{"name":"Submit invalid credentials 5x","status":"PASS","duration_ms":2100},` +
			`{"name":"Wait for lockout banner","status":"FAIL","duration_ms":5000,` +
			`"error":"Timeout 5000ms exceeded waiting for selector #lockout-banner"}]`,
		Browser: "Chrome 124", OS: "Windows 11", Environment: "staging", AppVersion: "2026.04.3",
	}
	networkStory := runResultEnrichment{
		Screenshots: `["https://demo.ttgo.test/artifacts/run6/search-network.png"]`,
		TraceURL:    "https://demo.ttgo.test/artifacts/run6/checkout.zip",
		LogText: "2026-04-22T09:22:30.000Z INFO  page.click [data-testid=search-submit]\n" +
			"2026-04-22T09:23:00.004Z ERROR network request timed out after 30000ms\n",
		StackTrace: "FetchError: request to https://api.demo.test/search?q=shoes failed, reason: ETIMEDOUT\n" +
			"    at ClientRequest.<anonymous> (node_modules/node-fetch/lib/index.js:1483:11)",
		Steps: `[{"name":"Submit search query","status":"PASS","duration_ms":150},` +
			`{"name":"Fetch results","status":"FAIL","duration_ms":30000,` +
			`"error":"net::ERR_CONNECTION_TIMED_OUT at /api/search?q=shoes"}]`,
		Browser: "Safari 17", OS: "macOS 14.4", Environment: "production-mirror", AppVersion: "2026.04.5",
	}

	// Keyed by deterministic RunResult ID so the enrichment survives any loop
	// reordering in buildDemoRuns. These IDs correspond to FAIL results in the
	// respective runs (see buildDemoRuns for the i%3==2 / i%4==3 patterns).
	byID := map[string]runResultEnrichment{
		demoID("rr:run2-" + demoID("tc:session-expires")):        assertionStory,
		demoID("rr:run2-" + demoID("tc:checkout-valid-payment")): timeoutStory,
		demoID("rr:run3-" + demoID("tc:account-lockout")):        lockoutStory,
		demoID("rr:run6-" + demoID("tc:category-filter")):        networkStory,
	}

	for i := range results {
		rich, ok := byID[results[i].ID]
		if !ok {
			continue
		}
		results[i].Screenshots = rich.Screenshots
		results[i].Video = rich.Video
		results[i].TraceURL = rich.TraceURL
		results[i].LogText = rich.LogText
		results[i].StackTrace = rich.StackTrace
		results[i].Steps = []byte(rich.Steps)
		results[i].Browser = rich.Browser
		results[i].OS = rich.OS
		results[i].Environment = rich.Environment
		results[i].AppVersion = rich.AppVersion
	}
	return results
}

// buildDemoRetryChain emits a second attempt for a failing test case in the
// regression run so the demo exercises AttemptNumber>1 and the retry stats
// surfaced on TestRun (RetriedCount/TotalAttempts).
func buildDemoRetryChain(now time.Time, runID string, members []demoTestCaseRef, startTime, endTime time.Time) []models.RunResult {
	// In run5 the failing regression tests are members at i%4==3, i.e. indices 3 and 7.
	// Add a successful retry for the test at index 3 (tc10 – password reset).
	if len(members) < 4 {
		return nil
	}
	retryMember := members[3]
	id := retryMember.ID
	return []models.RunResult{
		{
			ID:               demoID("rr:run5-" + retryMember.ID + ":attempt2"),
			TestRunID:        runID,
			TestCaseID:       &id,
			AttemptNumber:    2,
			TestNameSnapshot: retryMember.Name,
			Status:           models.StatusPass,
			DurationMs:       320,
			StartTime:        startTime.Add(30 * time.Second),
			EndTime:          endTime,
			Browser:          "Chrome 124",
			OS:               "macOS 14.4",
			Environment:      "staging",
			AppVersion:       "2026.04.3",
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}
}
