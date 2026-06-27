package store

import "time"

// AnalyticsSummary holds aggregate pass/fail statistics.
type AnalyticsSummary struct {
	TotalRuns int     `json:"total_runs"`
	PassCount int     `json:"pass_count"`
	FailCount int     `json:"fail_count"`
	SkipCount int     `json:"skip_count"`
	PassRate  float64 `json:"pass_rate"`
}

// TrendPoint holds daily pass/fail data for trend charts.
type TrendPoint struct {
	Date      string  `json:"date"`
	TotalRuns int     `json:"total_runs"`
	PassCount int     `json:"pass_count"`
	FailCount int     `json:"fail_count"`
	SkipCount int     `json:"skip_count"`
	PassRate  float64 `json:"pass_rate"`
}

// GetAnalyticsSummary returns aggregate pass/fail totals across all run results.
func (s *Store) GetAnalyticsSummary(startDate, endDate time.Time, folderID string) (*AnalyticsSummary, error) {
	var summary struct {
		Total int `gorm:"column:total"`
		Pass  int `gorm:"column:pass"`
		Fail  int `gorm:"column:fail"`
		Skip  int `gorm:"column:skip"`
	}

	q := newAnalyticsQuery(`
		SELECT
			COUNT(*) as total,
			SUM(CASE WHEN rr.status IN ('PASS') THEN 1 ELSE 0 END) as pass,
			SUM(CASE WHEN rr.status IN ('FAIL','ERROR') THEN 1 ELSE 0 END) as fail,
			SUM(CASE WHEN rr.status IN ('SKIP','PENDING') THEN 1 ELSE 0 END) as skip
		FROM run_results rr`)

	if folderID != "" {
		q.Join("JOIN test_runs tr ON rr.test_run_id = tr.id")
	}
	if !startDate.IsZero() {
		q.Where("rr.start_time >= ?", startDate)
	}
	if !endDate.IsZero() {
		q.Where("rr.start_time < ?", endDate)
	}
	if folderID != "" {
		q.Where("tr.run_folder_id = ?", folderID)
	}

	query, args := q.Build()
	if err := s.db.Raw(query, args...).Scan(&summary).Error; err != nil {
		return nil, err
	}

	passRate := 0.0
	if summary.Total > 0 {
		passRate = float64(summary.Pass) / float64(summary.Total)
	}

	return &AnalyticsSummary{
		TotalRuns: summary.Total,
		PassCount: summary.Pass,
		FailCount: summary.Fail,
		SkipCount: summary.Skip,
		PassRate:  passRate,
	}, nil
}

// GetTrendData returns daily pass/fail data from run_results rows.
func (s *Store) GetTrendData(startDate, endDate time.Time, folderID string) ([]TrendPoint, error) {
	var rows []struct {
		Date  string `gorm:"column:date"`
		Total int    `gorm:"column:total"`
		Pass  int    `gorm:"column:pass"`
		Fail  int    `gorm:"column:fail"`
		Skip  int    `gorm:"column:skip"`
	}

	q := newAnalyticsQuery(`
		SELECT
			strftime('%Y-%m-%d', rr.start_time) as date,
			COUNT(*) as total,
			SUM(CASE WHEN rr.status IN ('PASS') THEN 1 ELSE 0 END) as pass,
			SUM(CASE WHEN rr.status IN ('FAIL','ERROR') THEN 1 ELSE 0 END) as fail,
			SUM(CASE WHEN rr.status IN ('SKIP','PENDING') THEN 1 ELSE 0 END) as skip
		FROM run_results rr`)

	if folderID != "" {
		q.Join("JOIN test_runs tr ON rr.test_run_id = tr.id")
	}
	if !startDate.IsZero() {
		q.Where("rr.start_time >= ?", startDate)
	}
	if !endDate.IsZero() {
		q.Where("rr.start_time < ?", endDate)
	}
	if folderID != "" {
		q.Where("tr.run_folder_id = ?", folderID)
	}
	q.GroupBy("strftime('%Y-%m-%d', rr.start_time)").OrderBy("date ASC")

	query, args := q.Build()
	if err := s.db.Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}

	points := make([]TrendPoint, 0, len(rows))
	for _, r := range rows {
		passRate := 0.0
		if r.Total > 0 {
			passRate = float64(r.Pass) / float64(r.Total)
		}
		points = append(points, TrendPoint{
			Date:      r.Date,
			TotalRuns: r.Total,
			PassCount: r.Pass,
			FailCount: r.Fail,
			SkipCount: r.Skip,
			PassRate:  passRate,
		})
	}

	return points, nil
}
