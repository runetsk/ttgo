package store

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ttgo/pkg/tracker/models"
)

func TestAnalyticsQueryApplyRunResultFiltersUsesAliasSpecificJoin(t *testing.T) {
	q := newAnalyticsQuery("SELECT * FROM run_results rr")
	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	q.applyRunResultFilters("r2", "folder-123", start, end)
	sql, args := q.Build()

	assert.Contains(t, sql, "JOIN test_runs r2_tr ON r2.test_run_id = r2_tr.id")
	assert.Contains(t, sql, "r2.start_time >= ?")
	assert.Contains(t, sql, "r2.start_time < ?")
	assert.Contains(t, sql, "r2_tr.run_folder_id = ?")
	require.Len(t, args, 3)
	assert.Equal(t, start, args[0])
	assert.Equal(t, end, args[1])
	assert.Equal(t, "folder-123", args[2])
}

func seedRunResults(t *testing.T, s *Store) string {
	t.Helper()
	folderID := uuid.New().String()
	require.NoError(t, s.db.Exec("INSERT INTO folders (id, name, created_at, updated_at) VALUES (?, 'F', datetime('now'), datetime('now'))", folderID).Error)
	tc := models.TestCase{ID: uuid.New().String(), FolderID: folderID, Name: "Seeded Test"}
	require.NoError(t, s.db.Create(&tc).Error)

	categoryID := uuid.New().String()
	require.NoError(t, s.db.Exec("INSERT INTO suites (id, name, created_at, updated_at) VALUES (?, 'S', datetime('now'), datetime('now'))", categoryID).Error)
	run := models.TestRun{ID: uuid.New().String(), Name: "R1", CategoryID: &categoryID, Status: models.StatusPass, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	require.NoError(t, s.db.Create(&run).Error)

	now := time.Now()
	tc2 := models.TestCase{ID: uuid.New().String(), FolderID: folderID, Name: "Seeded Test 2"}
	require.NoError(t, s.db.Create(&tc2).Error)
	results := []models.RunResult{
		{ID: uuid.New().String(), TestRunID: run.ID, TestCaseID: &tc.ID, AttemptNumber: 1, TestNameSnapshot: "T", Status: models.StatusPass, StartTime: now},
		{ID: uuid.New().String(), TestRunID: run.ID, TestCaseID: &tc2.ID, AttemptNumber: 1, TestNameSnapshot: "T", Status: models.StatusFail, StartTime: now},
	}
	require.NoError(t, s.db.Create(&results).Error)
	return tc.ID
}

// T061: GetAnalyticsSummary returns correct pass_count and fail_count
func TestGetAnalyticsSummaryCorrectCounts(t *testing.T) {
	s, err := New(":memory:")
	require.NoError(t, err)
	seedRunResults(t, s)

	summary, err := s.GetAnalyticsSummary(time.Time{}, time.Time{}, "")
	require.NoError(t, err)
	assert.Equal(t, 2, summary.TotalRuns)
	assert.Equal(t, 1, summary.PassCount)
	assert.Equal(t, 1, summary.FailCount)
}

// T064: GetAnalyticsSummary on empty DB returns zeros (no NaN, no 500)
func TestGetAnalyticsSummaryEmptyDB(t *testing.T) {
	s, err := New(":memory:")
	require.NoError(t, err)

	summary, err := s.GetAnalyticsSummary(time.Time{}, time.Time{}, "")
	require.NoError(t, err)
	assert.Equal(t, 0, summary.TotalRuns)
	assert.Equal(t, 0.0, summary.PassRate, "pass_rate should be 0.0 not NaN on empty DB")
}

// T062: GetTrendData returns data points grouped by day
func TestGetTrendDataGroupsByDay(t *testing.T) {
	s, err := New(":memory:")
	require.NoError(t, err)
	seedRunResults(t, s)
	require.NoError(t, s.ComputeDailyMetrics(time.Now()))

	now := time.Now().UTC().Truncate(24 * time.Hour)
	points, err := s.GetTrendData(now.AddDate(0, 0, -30), now.AddDate(0, 0, 1), "")
	require.NoError(t, err)
	// Should have at most 1 data point for today
	assert.LessOrEqual(t, len(points), 1)
}

// DetectFlakyTestsSwitchMethod: counts PASS↔FAIL transitions over the lookback
// window, ignores SKIP results, and excludes tests that never switch.
func TestDetectFlakyTestsSwitchMethodCountsSwitches(t *testing.T) {
	s, err := New(":memory:")
	require.NoError(t, err)

	folderID := uuid.New().String()
	require.NoError(t, s.db.Exec("INSERT INTO folders (id, name, created_at, updated_at) VALUES (?, 'F', datetime('now'), datetime('now'))", folderID).Error)
	flaky := models.TestCase{ID: uuid.New().String(), FolderID: folderID, Name: "Flaky"}
	require.NoError(t, s.db.Create(&flaky).Error)
	stable := models.TestCase{ID: uuid.New().String(), FolderID: folderID, Name: "Stable"}
	require.NoError(t, s.db.Create(&stable).Error)

	run := models.TestRun{ID: uuid.New().String(), Name: "R", Status: models.StatusPass, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	require.NoError(t, s.db.Create(&run).Error)

	now := time.Now().UTC()
	attempt := 0
	mk := func(tcID string, status models.ExecutionStatus, name string, minutesAgo int) models.RunResult {
		attempt++
		return models.RunResult{
			ID: uuid.New().String(), TestRunID: run.ID, TestCaseID: &tcID,
			AttemptNumber: attempt, TestNameSnapshot: name, Status: status,
			StartTime: now.Add(-time.Duration(minutesAgo) * time.Minute),
		}
	}
	results := []models.RunResult{
		// Flaky, newest first: PASS, FAIL, PASS, PASS → 2 switches / 3 possible
		mk(flaky.ID, models.StatusPass, "Flaky latest", 1),
		mk(flaky.ID, models.StatusFail, "Flaky", 2),
		// SKIP row inside the window must not affect switch counting
		mk(flaky.ID, models.StatusSkip, "Flaky", 3),
		mk(flaky.ID, models.StatusPass, "Flaky", 4),
		mk(flaky.ID, models.StatusPass, "Flaky", 5),
		// Stable: PASS×3, no switches → excluded
		mk(stable.ID, models.StatusPass, "Stable", 1),
		mk(stable.ID, models.StatusPass, "Stable", 2),
		mk(stable.ID, models.StatusPass, "Stable", 3),
	}
	require.NoError(t, s.db.Create(&results).Error)

	windowStart := now.AddDate(0, 0, -30)
	windowEnd := now.AddDate(0, 0, 1)

	found, err := s.DetectFlakyTestsSwitchMethod(windowStart, windowEnd, "", 30, 20)
	require.NoError(t, err)
	require.Len(t, found, 1, "only the switching test is flaky")
	assert.Equal(t, flaky.ID, found[0].TestCaseID)
	assert.Equal(t, "Flaky latest", found[0].TestCaseName, "name comes from the newest result")
	assert.Equal(t, 2, found[0].SwitchCount)
	assert.Equal(t, 3, found[0].PossibleSwitches)
	assert.Equal(t, 4, found[0].TotalRuns, "SKIP results are excluded")
	assert.Equal(t, "PASS", found[0].CurrentStatus)
	assert.InDelta(t, 100.0*2.0/3.0, found[0].SwitchPercentage, 0.01)
	assert.WithinDuration(t, now.Add(-1*time.Minute), found[0].LastSwitchAt, 2*time.Second)

	// Lookback caps the sequence at the newest N results: PASS, FAIL → 1 switch / 1 possible
	found, err = s.DetectFlakyTestsSwitchMethod(windowStart, windowEnd, "", 2, 20)
	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Equal(t, 1, found[0].SwitchCount)
	assert.Equal(t, 1, found[0].PossibleSwitches)
	assert.Equal(t, 2, found[0].TotalRuns)
}

// T063: DetectFlakyTests excludes tests with fewer than 10 runs
func TestDetectFlakyTestsMinimumRuns(t *testing.T) {
	s, err := New(":memory:")
	require.NoError(t, err)
	seedRunResults(t, s) // only 2 runs, below threshold of 10

	stats, err := s.DetectFlakyTests(0.20)
	require.NoError(t, err)
	// No flaky tests should be detected — not enough runs
	assert.Empty(t, stats, "tests with fewer than 10 runs should not be flagged as flaky")
}
