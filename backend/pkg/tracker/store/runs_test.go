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
