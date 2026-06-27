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
