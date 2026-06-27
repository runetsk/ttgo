package store

import (
	"fmt"
	"time"
)

// GetRecentActivity returns the most recent audit log entries.
func (s *Store) GetRecentActivity(startDate, endDate time.Time, limit int) ([]map[string]interface{}, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}

	q := newAnalyticsQuery(`
		SELECT
			a.id,
			a.test_case_id,
			COALESCE(tc.name, a.test_case_id) as test_case_name,
			a.action,
			a.diff,
			a.user_id,
			a.timestamp
		FROM audit_logs a
		LEFT JOIN test_cases tc ON a.test_case_id = tc.id`)

	if !startDate.IsZero() {
		q.Where("a.timestamp >= ?", startDate)
	}
	if !endDate.IsZero() {
		q.Where("a.timestamp < ?", endDate)
	}
	q.OrderBy("a.timestamp DESC").Limit(limit)

	query, args := q.Build()

	var results []map[string]interface{}
	rows, err := s.db.Raw(query, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id, testCaseID, testCaseName, action, diff, userID, timestamp string
		if err := rows.Scan(&id, &testCaseID, &testCaseName, &action, &diff, &userID, &timestamp); err != nil {
			return nil, err
		}
		results = append(results, map[string]interface{}{
			"id":             id,
			"test_case_id":   testCaseID,
			"test_case_name": testCaseName,
			"action":         action,
			"diff":           diff,
			"user_id":        userID,
			"timestamp":      timestamp,
		})
	}
	if results == nil {
		results = []map[string]interface{}{}
	}
	return results, nil
}

// GetUniqueBugs returns unique Jira defect links with linked test case count.
func (s *Store) GetUniqueBugs(startDate, endDate time.Time, limit int) ([]map[string]interface{}, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	q := newAnalyticsQuery(`
		SELECT
			dl.jira_issue_key,
			dl.last_known_summary,
			dl.last_known_status,
			dl.last_known_priority,
			dl.last_known_assignee,
			dl.last_known_url,
			dl.status_category,
			COUNT(DISTINCT dl.test_case_id) as linked_test_count,
			MIN(dl.created_at) as first_linked_at,
			MAX(dl.updated_at) as last_updated_at
		FROM defect_links dl`)

	if !startDate.IsZero() {
		q.Where("dl.created_at >= ?", startDate)
	}
	if !endDate.IsZero() {
		q.Where("dl.created_at < ?", endDate)
	}
	q.GroupBy("dl.jira_issue_key").OrderBy("linked_test_count DESC, dl.jira_issue_key ASC").Limit(limit)

	query, args := q.Build()

	var results []map[string]interface{}
	rows, err := s.db.Raw(query, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var jiraKey, summary, status, priority, assignee, url, statusCat string
		var linkedCount int
		var firstLinked, lastUpdated string
		if err := rows.Scan(&jiraKey, &summary, &status, &priority, &assignee, &url, &statusCat, &linkedCount, &firstLinked, &lastUpdated); err != nil {
			return nil, err
		}
		results = append(results, map[string]interface{}{
			"jira_issue_key":    jiraKey,
			"summary":           summary,
			"status":            status,
			"priority":          priority,
			"assignee":          assignee,
			"url":               url,
			"status_category":   statusCat,
			"linked_test_count": linkedCount,
			"first_linked_at":   firstLinked,
			"last_updated_at":   lastUpdated,
		})
	}
	if results == nil {
		results = []map[string]interface{}{}
	}
	return results, nil
}

// CompareRuns returns side-by-side statistics for two test runs.
func (s *Store) CompareRuns(runID1, runID2 string) (map[string]interface{}, error) {
	type runStats struct {
		RunID      string
		RunName    string
		TotalTests int
		Passed     int
		Failed     int
		Skipped    int
		PassRate   float64
		TotalDurMs int64
		CreatedAt  string
	}

	fetchStats := func(runID string) (*runStats, error) {
		var run struct {
			ID        string `gorm:"column:id"`
			Name      string `gorm:"column:name"`
			CreatedAt string `gorm:"column:created_at"`
		}
		if err := s.db.Raw("SELECT id, name, created_at FROM test_runs WHERE id = ?", runID).Scan(&run).Error; err != nil {
			return nil, err
		}
		if run.ID == "" {
			return nil, fmt.Errorf("run not found: %s", runID)
		}

		var agg struct {
			Total   int   `gorm:"column:total"`
			Pass    int   `gorm:"column:pass"`
			Fail    int   `gorm:"column:fail"`
			Skip    int   `gorm:"column:skip"`
			TotalMs int64 `gorm:"column:total_ms"`
		}
		err := s.db.Raw(`
			SELECT
				COUNT(*) as total,
				SUM(CASE WHEN status = 'PASS' THEN 1 ELSE 0 END) as pass,
				SUM(CASE WHEN status IN ('FAIL','ERROR') THEN 1 ELSE 0 END) as fail,
				SUM(CASE WHEN status IN ('SKIP','PENDING') THEN 1 ELSE 0 END) as skip,
				COALESCE(SUM(duration_ms), 0) as total_ms
			FROM run_results WHERE test_run_id = ?
		`, runID).Scan(&agg).Error
		if err != nil {
			return nil, err
		}

		rate := 0.0
		if agg.Total > 0 {
			rate = float64(agg.Pass) / float64(agg.Total) * 100
		}

		return &runStats{
			RunID:      run.ID,
			RunName:    run.Name,
			TotalTests: agg.Total,
			Passed:     agg.Pass,
			Failed:     agg.Fail,
			Skipped:    agg.Skip,
			PassRate:   rate,
			TotalDurMs: agg.TotalMs,
			CreatedAt:  run.CreatedAt,
		}, nil
	}

	s1, err := fetchStats(runID1)
	if err != nil {
		return nil, err
	}
	s2, err := fetchStats(runID2)
	if err != nil {
		return nil, err
	}

	toMap := func(st *runStats) map[string]interface{} {
		return map[string]interface{}{
			"run_id":       st.RunID,
			"run_name":     st.RunName,
			"total_tests":  st.TotalTests,
			"passed":       st.Passed,
			"failed":       st.Failed,
			"skipped":      st.Skipped,
			"pass_rate":    st.PassRate,
			"total_dur_ms": st.TotalDurMs,
			"created_at":   st.CreatedAt,
		}
	}

	return map[string]interface{}{
		"run1": toMap(s1),
		"run2": toMap(s2),
	}, nil
}
