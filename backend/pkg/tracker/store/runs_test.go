package store

import (
	"encoding/json"
	"testing"
	"time"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunResultRichFields(t *testing.T) {
	s := newTestStore(t)
	category, _ := s.CreateCategory("Regression", "Full Suite")
	// Create Run using struct
	run := &models.TestRun{
		CategoryID: &category.ID,
		Name:       "Daily Run",
	}
	err := s.CreateTestRun(run)
	require.NoError(t, err)

	// Create a test case to link
	folder, _ := s.CreateFolder("Root", nil)
	tc := &models.TestCase{Name: "Login", FolderID: folder.ID}
	_ = s.CreateTestCase(tc)

	// Create a Result with RICH fields
	result := &models.RunResult{
		TestRunID:        run.ID,
		TestCaseID:       &tc.ID,
		TestNameSnapshot: tc.Name,
		Status:           models.StatusFail,

		// Timing
		DurationMs: 1250,
		StartTime:  time.Now().Add(-2 * time.Second),
		EndTime:    time.Now(),

		// Failure
		ErrorMessage: "Element #submit not found",
		StackTrace:   "Error: at page.click (login.js:20:10)",
		FailureType:  "TimeoutError",

		// Context
		Browser:     "Chrome 120",
		OS:          "macOS",
		Environment: "Staging",
		AppVersion:  "v1.5.0",

		// Artifacts
		Screenshots: `["https://s3.bucket/scr.png"]`,
		Video:       "https://s3.bucket/vid.mp4",
		TraceURL:    "https://trace.playwright.dev/123",
		LogText:     "[INFO] Starting\n[ERROR] Failed",

		// Steps
		Steps: json.RawMessage(`[{"action":"Open","status":"PASS"},{"action":"Click","status":"FAIL"}]`),
	}

	err = s.AddRunResult(result)
	require.NoError(t, err)

	// Verify Retrieval
	fetchedRun, err := s.GetTestRun(run.ID)
	require.NoError(t, err)
	require.Len(t, fetchedRun.RunResults, 1)

	r := fetchedRun.RunResults[0]
	assert.Equal(t, "Chrome 120", r.Browser)
	assert.Equal(t, "v1.5.0", r.AppVersion)
	assert.Equal(t, int64(1250), r.DurationMs)
	assert.Equal(t, "TimeoutError", r.FailureType)
	assert.Equal(t, `["https://s3.bucket/scr.png"]`, r.Screenshots)
	assert.Contains(t, r.LogText, "[ERROR] Failed")
	assert.JSONEq(t, `[{"action":"Open","status":"PASS"},{"action":"Click","status":"FAIL"}]`, string(r.Steps))
}

// TestGetTestRunPreloadsTestCaseSuites verifies that GetTestRun eagerly loads
// RunResults → TestCase → Suites so the frontend can derive run-level categories.
func TestGetTestRunPreloadsTestCaseCategories(t *testing.T) {
	s := newTestStore(t)

	category1, _ := s.CreateCategory("Smoke", "")
	category2, _ := s.CreateCategory("Regression", "")
	run := &models.TestRun{CategoryID: &category1.ID, Name: "Preload Run"}
	require.NoError(t, s.CreateTestRun(run))

	folder, _ := s.CreateFolder("Root", nil)
	tc := &models.TestCase{Name: "Login", FolderID: folder.ID}
	require.NoError(t, s.CreateTestCase(tc))

	// Assign both categories to the test case
	tc.Categories = []*models.Category{category1, category2}
	require.NoError(t, s.UpdateTestCase(tc))

	// Add the test case as a result
	result := &models.RunResult{
		TestRunID:        run.ID,
		TestCaseID:       &tc.ID,
		TestNameSnapshot: tc.Name,
		Status:           models.StatusPass,
	}
	require.NoError(t, s.AddRunResult(result))

	// Fetch the run and verify TestCase.Categories are preloaded
	fetched, err := s.GetTestRun(run.ID)
	require.NoError(t, err)
	require.Len(t, fetched.RunResults, 1)

	r := fetched.RunResults[0]
	require.NotNil(t, r.TestCase, "TestCase should be preloaded")
	assert.Equal(t, tc.ID, r.TestCase.ID)
	assert.Len(t, r.TestCase.Categories, 2, "Categories should be preloaded via TestCase")

	categoryNames := []string{r.TestCase.Categories[0].Name, r.TestCase.Categories[1].Name}
	assert.ElementsMatch(t, []string{"Smoke", "Regression"}, categoryNames)
}

func TestRunResultAttemptNumberDefault(t *testing.T) {
	s := newTestStore(t)
	folder, _ := s.CreateFolder("Root", nil)
	tc := &models.TestCase{Name: "Login", FolderID: folder.ID}
	_ = s.CreateTestCase(tc)

	run := &models.TestRun{Name: "Retry Test Run"}
	require.NoError(t, s.CreateTestRun(run))

	result := &models.RunResult{
		TestRunID:        run.ID,
		TestCaseID:       &tc.ID,
		TestNameSnapshot: tc.Name,
		Status:           models.StatusPending,
	}
	require.NoError(t, s.AddRunResult(result))

	// Reload and verify default
	got, err := s.GetTestRun(run.ID)
	require.NoError(t, err)
	require.Len(t, got.RunResults, 1)
	assert.Equal(t, 1, got.RunResults[0].AttemptNumber)
}

func TestUpdateRunResultByPK(t *testing.T) {
	s := newTestStore(t)
	folder, _ := s.CreateFolder("Root", nil)
	tc := &models.TestCase{Name: "Login", FolderID: folder.ID}
	_ = s.CreateTestCase(tc)

	run := &models.TestRun{Name: "PK Update Test"}
	require.NoError(t, s.CreateTestRun(run))

	result := &models.RunResult{
		TestRunID:        run.ID,
		TestCaseID:       &tc.ID,
		TestNameSnapshot: tc.Name,
		Status:           models.StatusPending,
	}
	require.NoError(t, s.AddRunResult(result))

	// Update by result ID
	err := s.UpdateRunResult(run.ID, result.ID, map[string]interface{}{
		"status": string(models.StatusPass),
	})
	require.NoError(t, err)

	got, _ := s.GetTestRun(run.ID)
	assert.Equal(t, models.StatusPass, got.RunResults[0].Status)
}

func TestDeleteRunResultByPK(t *testing.T) {
	s := newTestStore(t)
	folder, _ := s.CreateFolder("Root", nil)
	tc := &models.TestCase{Name: "Login", FolderID: folder.ID}
	_ = s.CreateTestCase(tc)

	run := &models.TestRun{Name: "PK Delete Test"}
	require.NoError(t, s.CreateTestRun(run))

	result := &models.RunResult{
		TestRunID:        run.ID,
		TestCaseID:       &tc.ID,
		TestNameSnapshot: tc.Name,
		Status:           models.StatusPending,
	}
	require.NoError(t, s.AddRunResult(result))

	err := s.DeleteRunResult(run.ID, result.ID)
	require.NoError(t, err)

	got, _ := s.GetTestRun(run.ID)
	assert.Empty(t, got.RunResults)
}

func TestRetryRunResult(t *testing.T) {
	s := newTestStore(t)
	folder, _ := s.CreateFolder("Root", nil)
	tc := &models.TestCase{Name: "Login", FolderID: folder.ID}
	_ = s.CreateTestCase(tc)

	run := &models.TestRun{Name: "Retry Run"}
	require.NoError(t, s.CreateTestRun(run))

	result := &models.RunResult{
		TestRunID:        run.ID,
		TestCaseID:       &tc.ID,
		TestNameSnapshot: tc.Name,
		Status:           models.StatusFail,
	}
	require.NoError(t, s.AddRunResult(result))

	newResult, err := s.RetryRunResult(run.ID, result.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, newResult.AttemptNumber)
	assert.Equal(t, models.StatusPending, newResult.Status)
	assert.Equal(t, run.ID, newResult.TestRunID)
	assert.Equal(t, tc.ID, *newResult.TestCaseID)
	assert.Equal(t, "Login", newResult.TestNameSnapshot)
	assert.NotEqual(t, result.ID, newResult.ID)

	got, _ := s.GetTestRun(run.ID)
	require.Len(t, got.RunResults, 2)
}

func TestRetryRunResultOrphanedReturnsError(t *testing.T) {
	s := newTestStore(t)

	run := &models.TestRun{Name: "Orphan Retry Run"}
	require.NoError(t, s.CreateTestRun(run))

	result := &models.RunResult{
		TestRunID:        run.ID,
		TestNameSnapshot: "Deleted Test",
		Status:           models.StatusFail,
	}
	require.NoError(t, s.AddRunResult(result))

	_, err := s.RetryRunResult(run.ID, result.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "orphaned")
}

func TestRetryRunResultMultipleRetries(t *testing.T) {
	s := newTestStore(t)
	folder, _ := s.CreateFolder("Root", nil)
	tc := &models.TestCase{Name: "Login", FolderID: folder.ID}
	_ = s.CreateTestCase(tc)

	run := &models.TestRun{Name: "Multi Retry"}
	require.NoError(t, s.CreateTestRun(run))

	result := &models.RunResult{
		TestRunID:        run.ID,
		TestCaseID:       &tc.ID,
		TestNameSnapshot: tc.Name,
		Status:           models.StatusFail,
	}
	require.NoError(t, s.AddRunResult(result))

	r2, err := s.RetryRunResult(run.ID, result.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, r2.AttemptNumber)

	r3, err := s.RetryRunResult(run.ID, r2.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, r3.AttemptNumber)

	got, _ := s.GetTestRun(run.ID)
	assert.Len(t, got.RunResults, 3)
}

func TestRetryRunResultRevertsCompletedRunToRunning(t *testing.T) {
	s := newTestStore(t)
	folder, _ := s.CreateFolder("Root", nil)
	tc := &models.TestCase{Name: "Login", FolderID: folder.ID}
	_ = s.CreateTestCase(tc)

	run := &models.TestRun{Name: "Completed Run"}
	require.NoError(t, s.CreateTestRun(run))

	r1 := &models.RunResult{TestRunID: run.ID, TestCaseID: &tc.ID, TestNameSnapshot: tc.Name, Status: models.StatusPass}
	require.NoError(t, s.AddRunResult(r1))

	completed, _, err := s.CompleteRun(run.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusPass, completed.Status)

	_, err = s.RetryRunResult(run.ID, r1.ID)
	require.NoError(t, err)

	got, _ := s.GetTestRun(run.ID)
	assert.Equal(t, models.StatusRunning, got.Status)
}

func TestAddRunResultAutoIncrementAttempt(t *testing.T) {
	s := newTestStore(t)
	folder, _ := s.CreateFolder("Root", nil)
	tc := &models.TestCase{Name: "Login", FolderID: folder.ID}
	_ = s.CreateTestCase(tc)

	run := &models.TestRun{Name: "Auto Increment Run"}
	require.NoError(t, s.CreateTestRun(run))

	// First result
	r1 := &models.RunResult{
		TestRunID:        run.ID,
		TestCaseID:       &tc.ID,
		TestNameSnapshot: tc.Name,
		Status:           models.StatusFail,
	}
	require.NoError(t, s.AddRunResult(r1))
	assert.Equal(t, 1, r1.AttemptNumber)

	// Second result with same test_case_id — should auto-increment
	r2 := &models.RunResult{
		TestRunID:        run.ID,
		TestCaseID:       &tc.ID,
		TestNameSnapshot: tc.Name,
		Status:           models.StatusPass,
	}
	require.NoError(t, s.AddRunResult(r2))
	assert.Equal(t, 2, r2.AttemptNumber)
}

func TestAddRunResultExplicitAttemptConflict(t *testing.T) {
	s := newTestStore(t)
	folder, _ := s.CreateFolder("Root", nil)
	tc := &models.TestCase{Name: "Login", FolderID: folder.ID}
	_ = s.CreateTestCase(tc)

	run := &models.TestRun{Name: "Conflict Run"}
	require.NoError(t, s.CreateTestRun(run))

	r1 := &models.RunResult{
		TestRunID:        run.ID,
		TestCaseID:       &tc.ID,
		TestNameSnapshot: tc.Name,
		Status:           models.StatusFail,
	}
	require.NoError(t, s.AddRunResult(r1))

	// Explicit attempt_number=1 should fail (already exists)
	r2 := &models.RunResult{
		TestRunID:        run.ID,
		TestCaseID:       &tc.ID,
		TestNameSnapshot: tc.Name,
		Status:           models.StatusPass,
		AttemptNumber:    1,
	}
	err := s.AddRunResult(r2)
	assert.Error(t, err)
}

func TestGetTestRunsAggregationLatestAttemptOnly(t *testing.T) {
	s := newTestStore(t)
	folder, _ := s.CreateFolder("Root", nil)
	tc := &models.TestCase{Name: "Login", FolderID: folder.ID}
	_ = s.CreateTestCase(tc)

	run := &models.TestRun{Name: "Agg Test"}
	require.NoError(t, s.CreateTestRun(run))

	// Attempt 1: FAIL
	r1 := &models.RunResult{
		TestRunID:        run.ID,
		TestCaseID:       &tc.ID,
		TestNameSnapshot: tc.Name,
		Status:           models.StatusFail,
	}
	require.NoError(t, s.AddRunResult(r1))

	// Attempt 2: PASS (retry)
	r2, err := s.RetryRunResult(run.ID, r1.ID)
	require.NoError(t, err)
	require.NoError(t, s.UpdateRunResult(run.ID, r2.ID, map[string]interface{}{
		"status": string(models.StatusPass),
	}))

	// Get runs — should show 1 passed, 0 failed (latest attempt wins)
	runs, _, err := s.GetTestRuns(RunFilter{Limit: 50})
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, 1, runs[0].PassedResults)
	assert.Equal(t, 0, runs[0].FailedResults)
	assert.Equal(t, 1, runs[0].TotalResults)
}

func TestGetTestRunsDefectTypeCounts(t *testing.T) {
	s := newTestStore(t)
	folder, _ := s.CreateFolder("Root", nil)
	tcA := &models.TestCase{Name: "A", FolderID: folder.ID}
	_ = s.CreateTestCase(tcA)
	tcB := &models.TestCase{Name: "B", FolderID: folder.ID}
	_ = s.CreateTestCase(tcB)
	tcC := &models.TestCase{Name: "C", FolderID: folder.ID}
	_ = s.CreateTestCase(tcC)

	run := &models.TestRun{Name: "Defect Counts"}
	require.NoError(t, s.CreateTestRun(run))

	// tcA: attempt 1 FAIL/product_bug superseded by attempt 2 FAIL/automation_bug
	a1 := &models.RunResult{TestRunID: run.ID, TestCaseID: &tcA.ID, TestNameSnapshot: tcA.Name, Status: models.StatusFail, DefectType: "product_bug"}
	require.NoError(t, s.AddRunResult(a1))
	a2, err := s.RetryRunResult(run.ID, a1.ID)
	require.NoError(t, err)
	require.NoError(t, s.UpdateRunResult(run.ID, a2.ID, map[string]interface{}{
		"status": string(models.StatusFail), "defect_type": "automation_bug",
	}))

	// tcB: FAIL with empty defect_type counts as to_investigate
	require.NoError(t, s.AddRunResult(&models.RunResult{TestRunID: run.ID, TestCaseID: &tcB.ID, TestNameSnapshot: tcB.Name, Status: models.StatusFail}))

	// tcC: PASS, in no defect bucket
	require.NoError(t, s.AddRunResult(&models.RunResult{TestRunID: run.ID, TestCaseID: &tcC.ID, TestNameSnapshot: tcC.Name, Status: models.StatusPass}))

	// Orphan result (no test case): ERROR/system_issue is always included
	require.NoError(t, s.AddRunResult(&models.RunResult{TestRunID: run.ID, TestNameSnapshot: "orphan", Status: models.StatusError, DefectType: "system_issue"}))

	runs, _, err := s.GetTestRuns(RunFilter{Limit: 50})
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, 4, runs[0].TotalResults, "latest attempts only: tcA(1) + tcB + tcC + orphan")
	assert.Equal(t, 1, runs[0].PassedResults)
	assert.Equal(t, 3, runs[0].FailedResults, "FAIL, FAIL, ERROR")
	assert.Equal(t, 0, runs[0].ProductBug, "attempt 1 defect type superseded by retry")
	assert.Equal(t, 1, runs[0].AutomationBug)
	assert.Equal(t, 1, runs[0].SystemIssue)
	assert.Equal(t, 1, runs[0].ToInvestigate, "empty defect_type on failed result")
	assert.Equal(t, 1, runs[0].RetriedCount)
	assert.Equal(t, 4, runs[0].TotalAttempts, "tcA×2 + tcB + tcC; orphans excluded")
}

func TestGetTestRunRetriedCountAndTotalAttempts(t *testing.T) {
	s := newTestStore(t)
	folder, _ := s.CreateFolder("Root", nil)
	tc1 := &models.TestCase{Name: "Login", FolderID: folder.ID}
	_ = s.CreateTestCase(tc1)
	tc2 := &models.TestCase{Name: "Logout", FolderID: folder.ID}
	_ = s.CreateTestCase(tc2)

	run := &models.TestRun{Name: "Retry Count Test"}
	require.NoError(t, s.CreateTestRun(run))

	// tc1: 2 attempts (retried)
	r1 := &models.RunResult{TestRunID: run.ID, TestCaseID: &tc1.ID, TestNameSnapshot: tc1.Name, Status: models.StatusFail}
	require.NoError(t, s.AddRunResult(r1))
	_, err := s.RetryRunResult(run.ID, r1.ID)
	require.NoError(t, err)

	// tc2: 1 attempt (not retried)
	r2 := &models.RunResult{TestRunID: run.ID, TestCaseID: &tc2.ID, TestNameSnapshot: tc2.Name, Status: models.StatusPass}
	require.NoError(t, s.AddRunResult(r2))

	got, err := s.GetTestRun(run.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, got.RetriedCount)  // 1 test case was retried
	assert.Equal(t, 3, got.TotalAttempts) // 3 total result rows
}

func TestCopyTestRunOnlyCopiesLatestAttempt(t *testing.T) {
	s := newTestStore(t)
	folder, _ := s.CreateFolder("Root", nil)
	tc := &models.TestCase{Name: "Login", FolderID: folder.ID}
	_ = s.CreateTestCase(tc)

	run := &models.TestRun{Name: "Source"}
	require.NoError(t, s.CreateTestRun(run))

	r1 := &models.RunResult{TestRunID: run.ID, TestCaseID: &tc.ID, TestNameSnapshot: tc.Name, Status: models.StatusFail}
	require.NoError(t, s.AddRunResult(r1))

	// Create a retry (attempt 2)
	_, err := s.RetryRunResult(run.ID, r1.ID)
	require.NoError(t, err)

	// Source now has 2 results for same test case
	source, _ := s.GetTestRun(run.ID)
	require.Len(t, source.RunResults, 2)

	// Copy — should only have 1 result (latest attempt, reset to PENDING with attempt_number=1)
	copied, err := s.CopyTestRun(run.ID, "Copy", nil)
	require.NoError(t, err)

	copiedRun, _ := s.GetTestRun(copied.ID)
	require.Len(t, copiedRun.RunResults, 1)
	assert.Equal(t, 1, copiedRun.RunResults[0].AttemptNumber)
	assert.Equal(t, models.StatusPending, copiedRun.RunResults[0].Status)
}

func TestListLatestFailingResultsExcludesStaleAttempts(t *testing.T) {
	s := newTestStore(t)
	folder, _ := s.CreateFolder("Root", nil)
	tc := &models.TestCase{Name: "Login", FolderID: folder.ID}
	require.NoError(t, s.CreateTestCase(tc))

	run := &models.TestRun{Name: "Retry Run"}
	require.NoError(t, s.CreateTestRun(run))

	// Attempt 1: FAIL
	fail := &models.RunResult{
		TestRunID: run.ID, TestCaseID: &tc.ID, TestNameSnapshot: tc.Name,
		AttemptNumber: 1, Status: models.StatusFail, ErrorMessage: "boom",
	}
	require.NoError(t, s.AddRunResult(fail))

	// Attempt 2: PASS (latest)
	pass := &models.RunResult{
		TestRunID: run.ID, TestCaseID: &tc.ID, TestNameSnapshot: tc.Name,
		AttemptNumber: 2, Status: models.StatusPass,
	}
	require.NoError(t, s.AddRunResult(pass))

	got, err := s.ListLatestFailingResults(run.ID)
	require.NoError(t, err)
	require.Len(t, got, 0, "latest attempt is PASS, so no failures should be returned")
}

func TestListLatestFailingResultsIncludesOrphans(t *testing.T) {
	s := newTestStore(t)
	run := &models.TestRun{Name: "Orphan Run"}
	require.NoError(t, s.CreateTestRun(run))

	orphan := &models.RunResult{
		TestRunID: run.ID, TestCaseID: nil, TestNameSnapshot: "adhoc",
		AttemptNumber: 1, Status: models.StatusFail, ErrorMessage: "orphan fail",
	}
	require.NoError(t, s.AddRunResult(orphan))

	got, err := s.ListLatestFailingResults(run.ID)
	require.NoError(t, err)
	require.Len(t, got, 1, "orphan FAIL should be included")
}

func TestGetTestRunsFilterByCategoryIDs(t *testing.T) {
	s := newTestStore(t)
	catA, err := s.CreateCategory("Smoke", "")
	require.NoError(t, err)
	catB, err := s.CreateCategory("Regression", "")
	require.NoError(t, err)

	r1 := &models.TestRun{Name: "Run A", CategoryID: &catA.ID}
	r2 := &models.TestRun{Name: "Run B", CategoryID: &catB.ID}
	r3 := &models.TestRun{Name: "Run C"} // no category
	require.NoError(t, s.CreateTestRun(r1))
	require.NoError(t, s.CreateTestRun(r2))
	require.NoError(t, s.CreateTestRun(r3))

	runs, total, err := s.GetTestRuns(RunFilter{CategoryIDs: []string{catA.ID, catB.ID}, Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	require.Len(t, runs, 2)
	names := []string{runs[0].Name, runs[1].Name}
	assert.ElementsMatch(t, []string{"Run A", "Run B"}, names)
}

func TestGetTestRunsFilterByCreatedRange(t *testing.T) {
	s := newTestStore(t)
	old := &models.TestRun{Name: "Old"}
	require.NoError(t, s.CreateTestRun(old))
	// Force an old created_at directly via the DB.
	require.NoError(t, s.DB().Model(old).Update("created_at", time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)).Error)

	recent := &models.TestRun{Name: "Recent"}
	require.NoError(t, s.CreateTestRun(recent)) // created_at ~ now

	// Only runs created on/after 2021-01-01 → just "Recent".
	runs, total, err := s.GetTestRuns(RunFilter{CreatedFrom: "2021-01-01", Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, runs, 1)
	assert.Equal(t, "Recent", runs[0].Name)

	// Inclusive upper bound: created on/before 2020-01-01 → just "Old".
	runs, total, err = s.GetTestRuns(RunFilter{CreatedTo: "2020-01-01", Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, runs, 1)
	assert.Equal(t, "Old", runs[0].Name)
}

func TestGetTestRunSummary(t *testing.T) {
	s := newTestStore(t)
	run := &models.TestRun{Name: "Summary Run"}
	require.NoError(t, s.CreateTestRun(run))

	folder, _ := s.CreateFolder("Root", nil)
	tc1 := &models.TestCase{Name: "Retried then passed", FolderID: folder.ID}
	require.NoError(t, s.CreateTestCase(tc1))
	tc2 := &models.TestCase{Name: "Failing", FolderID: folder.ID}
	require.NoError(t, s.CreateTestCase(tc2))
	tc3 := &models.TestCase{Name: "Pending", FolderID: folder.ID}
	require.NoError(t, s.CreateTestCase(tc3))

	// tc1: FAIL attempt 1, PASS attempt 2 — latest-attempt counters must see one PASS.
	require.NoError(t, s.AddRunResult(&models.RunResult{TestRunID: run.ID, TestCaseID: &tc1.ID, Status: models.StatusFail, DefectType: "product_bug"}))
	require.NoError(t, s.AddRunResult(&models.RunResult{TestRunID: run.ID, TestCaseID: &tc1.ID, Status: models.StatusPass}))
	// tc2: FAIL with default defect type.
	require.NoError(t, s.AddRunResult(&models.RunResult{TestRunID: run.ID, TestCaseID: &tc2.ID, Status: models.StatusFail, DefectType: "to_investigate"}))
	// tc3: PENDING.
	require.NoError(t, s.AddRunResult(&models.RunResult{TestRunID: run.ID, TestCaseID: &tc3.ID, Status: models.StatusPending}))

	summary, err := s.GetTestRunSummary(run.ID)
	require.NoError(t, err)
	require.NotNil(t, summary)

	assert.Equal(t, run.ID, summary.ID)
	assert.Empty(t, summary.RunResults, "summary must not load result rows")
	assert.Equal(t, 3, summary.TotalResults, "latest attempts only")
	assert.Equal(t, 1, summary.PassedResults)
	assert.Equal(t, 1, summary.FailedResults)
	assert.Equal(t, 1, summary.PendingResults)
	assert.Equal(t, 0, summary.SkippedResults)
	assert.Equal(t, 1, summary.ToInvestigate)
	assert.Equal(t, 0, summary.ProductBug, "superseded FAIL attempt must not count")
	assert.Equal(t, 1, summary.RetriedCount)
	assert.Equal(t, 4, summary.TotalAttempts)

	missing, err := s.GetTestRunSummary("nope")
	require.NoError(t, err)
	assert.Nil(t, missing)
}

func TestGetRunResultsByIDs(t *testing.T) {
	s := newTestStore(t)
	run := &models.TestRun{Name: "Delta Run"}
	require.NoError(t, s.CreateTestRun(run))
	otherRun := &models.TestRun{Name: "Other Run"}
	require.NoError(t, s.CreateTestRun(otherRun))

	folder, _ := s.CreateFolder("Root", nil)
	tc := &models.TestCase{Name: "Cased", FolderID: folder.ID}
	require.NoError(t, s.CreateTestCase(tc))

	r1 := &models.RunResult{TestRunID: run.ID, TestCaseID: &tc.ID, Status: models.StatusFail}
	require.NoError(t, s.AddRunResult(r1))
	r2 := &models.RunResult{TestRunID: run.ID, TestNameSnapshot: "orphan", Status: models.StatusPass}
	require.NoError(t, s.AddRunResult(r2))
	foreign := &models.RunResult{TestRunID: otherRun.ID, TestNameSnapshot: "foreign", Status: models.StatusPass}
	require.NoError(t, s.AddRunResult(foreign))

	rows, err := s.GetRunResultsByIDs(run.ID, []string{r1.ID, r2.ID, foreign.ID})
	require.NoError(t, err)
	require.Len(t, rows, 2, "foreign-run id must be excluded")

	byID := map[string]*models.RunResult{}
	for _, r := range rows {
		byID[r.ID] = r
	}
	require.NotNil(t, byID[r1.ID])
	require.NotNil(t, byID[r1.ID].TestCase, "TestCase must be preloaded")
	assert.Equal(t, "Cased", byID[r1.ID].TestCase.Name)
	require.NotNil(t, byID[r2.ID])
	assert.Nil(t, byID[r2.ID].TestCase)
}

// The status read behind bulk triage mode. Foreign ids must be absent from the map rather than
// present with a zero value, so the caller's IsFailureStatus partition drops them instead of
// silently counting them as updated.
func TestListRunResultStatuses(t *testing.T) {
	s := newTestStore(t)
	run := &models.TestRun{Name: "Triage Run"}
	require.NoError(t, s.CreateTestRun(run))
	otherRun := &models.TestRun{Name: "Other Run"}
	require.NoError(t, s.CreateTestRun(otherRun))

	failed := &models.RunResult{TestRunID: run.ID, TestNameSnapshot: "failed", Status: models.StatusFail}
	require.NoError(t, s.AddRunResult(failed))
	errored := &models.RunResult{TestRunID: run.ID, TestNameSnapshot: "errored", Status: models.StatusError}
	require.NoError(t, s.AddRunResult(errored))
	passed := &models.RunResult{TestRunID: run.ID, TestNameSnapshot: "passed", Status: models.StatusPass}
	require.NoError(t, s.AddRunResult(passed))
	foreign := &models.RunResult{TestRunID: otherRun.ID, TestNameSnapshot: "foreign", Status: models.StatusFail}
	require.NoError(t, s.AddRunResult(foreign))

	got, err := s.ListRunResultStatuses(run.ID, []string{failed.ID, errored.ID, passed.ID, foreign.ID, "no-such-id"})
	require.NoError(t, err)
	assert.Equal(t, map[string]models.ExecutionStatus{
		failed.ID:  models.StatusFail,
		errored.ID: models.StatusError,
		passed.ID:  models.StatusPass,
	}, got, "ids outside the run must be absent, not zero-valued")

	empty, err := s.ListRunResultStatuses(run.ID, nil)
	require.NoError(t, err)
	assert.Empty(t, empty, "an empty id list must not become an unbounded query")
}

func TestListRecentFailuresByTestCase(t *testing.T) {
	s := newTestStore(t)
	folder, _ := s.CreateFolder("Root", nil)
	tc := &models.TestCase{Name: "Login", FolderID: folder.ID}
	require.NoError(t, s.CreateTestCase(tc))

	now := time.Now()
	// seedFail creates a FAIL result for tc in its own run at the given start time.
	seedFail := func(name string, start time.Time) *models.RunResult {
		run := &models.TestRun{Name: name}
		require.NoError(t, s.CreateTestRun(run))
		rr := &models.RunResult{
			TestRunID: run.ID, TestCaseID: &tc.ID, TestNameSnapshot: tc.Name,
			Status: models.StatusFail, StartTime: start, ErrorMessage: name,
		}
		require.NoError(t, s.AddRunResult(rr))
		return rr
	}

	fresh := seedFail("fresh", now.Add(-1*time.Hour))
	mid := seedFail("mid", now.Add(-2*time.Hour))
	old := seedFail("old", now.Add(-3*time.Hour))
	seedFail("stale", now.Add(-40*24*time.Hour)) // outside the 30-day window

	since := now.Add(-30 * 24 * time.Hour)

	// Full window, generous limit: newest-first, stale row excluded by the window.
	got, err := s.ListRecentFailuresByTestCase(tc.ID, since, 5, "")
	require.NoError(t, err)
	require.Len(t, got, 3, "the 40-day-old failure is outside the 30-day window")
	assert.Equal(t, fresh.ID, got[0].ID, "start_time DESC: newest first")
	assert.Equal(t, mid.ID, got[1].ID)
	assert.Equal(t, old.ID, got[2].ID)

	// LIMIT caps the set to the two newest.
	limited, err := s.ListRecentFailuresByTestCase(tc.ID, since, 2, "")
	require.NoError(t, err)
	require.Len(t, limited, 2, "LIMIT 2 returns only the two newest")
	assert.Equal(t, fresh.ID, limited[0].ID)
	assert.Equal(t, mid.ID, limited[1].ID)

	// Guards: empty tcID and non-positive limit both short-circuit to (nil, nil).
	none, err := s.ListRecentFailuresByTestCase("", since, 5, "")
	require.NoError(t, err)
	assert.Nil(t, none, "empty tcID returns nil")
	none, err = s.ListRecentFailuresByTestCase(tc.ID, since, 0, "")
	require.NoError(t, err)
	assert.Nil(t, none, "non-positive limit returns nil")
}

func TestListRecentFailuresByTestCaseExclusions(t *testing.T) {
	s := newTestStore(t)
	folder, _ := s.CreateFolder("Root", nil)
	tc := &models.TestCase{Name: "Login", FolderID: folder.ID}
	require.NoError(t, s.CreateTestCase(tc))
	other := &models.TestCase{Name: "Logout", FolderID: folder.ID}
	require.NoError(t, s.CreateTestCase(other))

	now := time.Now()
	since := now.Add(-30 * 24 * time.Hour)

	// Current run — must be excluded via excludeRunID.
	curRun := &models.TestRun{Name: "current"}
	require.NoError(t, s.CreateTestRun(curRun))
	require.NoError(t, s.AddRunResult(&models.RunResult{
		TestRunID: curRun.ID, TestCaseID: &tc.ID, TestNameSnapshot: tc.Name,
		Status: models.StatusFail, StartTime: now.Add(-10 * time.Minute),
	}))

	// Prior run: one ERROR (expected back) plus non-fail rows (excluded by status)
	// and a failure for a different test case (excluded by test_case_id).
	priorRun := &models.TestRun{Name: "prior"}
	require.NoError(t, s.CreateTestRun(priorRun))
	priorFail := &models.RunResult{
		TestRunID: priorRun.ID, TestCaseID: &tc.ID, TestNameSnapshot: tc.Name,
		Status: models.StatusError, StartTime: now.Add(-1 * time.Hour),
	}
	require.NoError(t, s.AddRunResult(priorFail))
	require.NoError(t, s.AddRunResult(&models.RunResult{
		TestRunID: priorRun.ID, TestCaseID: &tc.ID, TestNameSnapshot: tc.Name,
		Status: models.StatusPass, StartTime: now.Add(-2 * time.Hour),
	}))
	require.NoError(t, s.AddRunResult(&models.RunResult{
		TestRunID: priorRun.ID, TestCaseID: &tc.ID, TestNameSnapshot: tc.Name,
		Status: models.StatusSkip, StartTime: now.Add(-3 * time.Hour),
	}))
	require.NoError(t, s.AddRunResult(&models.RunResult{
		TestRunID: priorRun.ID, TestCaseID: &other.ID, TestNameSnapshot: other.Name,
		Status: models.StatusFail, StartTime: now.Add(-1 * time.Hour),
	}))

	got, err := s.ListRecentFailuresByTestCase(tc.ID, since, 10, curRun.ID)
	require.NoError(t, err)
	require.Len(t, got, 1, "only the prior FAIL/ERROR for this test case survives the filters")
	assert.Equal(t, priorFail.ID, got[0].ID)
}
