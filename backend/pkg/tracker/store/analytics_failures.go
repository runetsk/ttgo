package store

import (
	"sort"
	"strings"
	"time"
	"ttgo/pkg/tracker/models"
)

// GetMostFailedTestCases returns top N test cases ranked by failure count.
func (s *Store) GetMostFailedTestCases(startDate, endDate time.Time, folderID string, limit int) ([]models.MostFailedTestCase, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	outerQ := newAnalyticsQuery("")
	outerQ.applyRunResultFilters("rr", folderID, startDate, endDate)
	outerQ.Where("rr.status IN ('FAIL','ERROR')")

	totalQ := newAnalyticsQuery("(SELECT COUNT(*) FROM run_results r2")
	totalQ.applyRunResultFilters("r2", folderID, startDate, endDate)
	totalQ.Where("r2.test_case_id = rr.test_case_id")
	totalSQL, totalArgs := totalQ.Build()
	totalSubquery := totalSQL + ")"

	var sb strings.Builder
	sb.WriteString(`
		SELECT
			rr.test_case_id,
			COALESCE(rr.test_name_snapshot, rr.test_case_id) as test_case_name,
			COUNT(*) as failed_count,
			`)
	sb.WriteString(totalSubquery)
	sb.WriteString(` as total_runs,
			CAST(COUNT(*) AS REAL) / MAX(`)
	sb.WriteString(totalSubquery)
	sb.WriteString(`, 1) as failure_rate,
			MAX(rr.start_time) as last_failure_at
		FROM run_results rr`)
	for _, j := range outerQ.joins {
		sb.WriteString(" ")
		sb.WriteString(j)
	}
	if len(outerQ.wheres) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(outerQ.wheres, " AND "))
	}
	sb.WriteString(`
		GROUP BY rr.test_case_id
		ORDER BY failed_count DESC
		LIMIT ?
	`)

	args := make([]interface{}, 0, len(totalArgs)*2+len(outerQ.args)+1)
	args = append(args, totalArgs...)
	args = append(args, totalArgs...)
	args = append(args, outerQ.args...)
	args = append(args, limit)

	var results []models.MostFailedTestCase
	if err := s.db.Raw(sb.String(), args...).Scan(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

// DetectFlakyTestsSwitchMethod detects flaky tests using status-switch counting.
// For each test case, analyzes the last `lookback` runs and counts status transitions.
func (s *Store) DetectFlakyTestsSwitchMethod(startDate, endDate time.Time, folderID string, lookback, limit int) ([]models.FlakyTestCase, error) {
	if lookback <= 0 {
		lookback = 30
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	q := newAnalyticsQuery("SELECT DISTINCT rr.test_case_id FROM run_results rr")
	q.applyRunResultFilters("rr", folderID, startDate, endDate)
	q.Where("rr.test_case_id IS NOT NULL")
	testCaseQuery, args := q.Build()

	var testCaseIDs []string
	if err := s.db.Raw(testCaseQuery, args...).Scan(&testCaseIDs).Error; err != nil {
		return nil, err
	}

	var results []models.FlakyTestCase
	for _, tcID := range testCaseIDs {
		var runs []struct {
			Status    string    `gorm:"column:status"`
			StartTime time.Time `gorm:"column:start_time"`
			TestName  string    `gorm:"column:test_name_snapshot"`
		}
		if err := s.db.Raw(`
			SELECT status, start_time, COALESCE(test_name_snapshot, test_case_id) as test_name_snapshot
			FROM run_results
			WHERE test_case_id = ? AND status IN ('PASS','FAIL','ERROR')
			ORDER BY start_time DESC
			LIMIT ?
		`, tcID, lookback).Scan(&runs).Error; err != nil {
			continue
		}

		if len(runs) < 2 {
			continue
		}

		switchCount := 0
		var lastSwitchAt time.Time
		for i := 0; i < len(runs)-1; i++ {
			currPass := runs[i].Status == "PASS"
			nextPass := runs[i+1].Status == "PASS"
			if currPass != nextPass {
				switchCount++
				if lastSwitchAt.IsZero() || runs[i].StartTime.After(lastSwitchAt) {
					lastSwitchAt = runs[i].StartTime
				}
			}
		}

		if switchCount == 0 {
			continue
		}

		possibleSwitches := len(runs) - 1
		switchPct := 0.0
		if possibleSwitches > 0 {
			switchPct = (float64(switchCount) / float64(possibleSwitches)) * 100
		}

		results = append(results, models.FlakyTestCase{
			TestCaseID:       tcID,
			TestCaseName:     runs[0].TestName,
			SwitchCount:      switchCount,
			PossibleSwitches: possibleSwitches,
			SwitchPercentage: switchPct,
			TotalRuns:        len(runs),
			CurrentStatus:    runs[0].Status,
			LastSwitchAt:     lastSwitchAt,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].SwitchPercentage > results[j].SwitchPercentage
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}
