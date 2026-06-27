package store

import (
	"time"
	"ttgo/pkg/tracker/models"
)

// GetDurationTrend returns daily aggregate duration data.
func (s *Store) GetDurationTrend(startDate, endDate time.Time, folderID string) ([]models.DurationTrendPoint, error) {
	q := newAnalyticsQuery(`
		SELECT
			strftime('%Y-%m-%d', rr.start_time) as date,
			SUM(rr.duration_ms) as total_duration_ms,
			AVG(rr.duration_ms) as avg_duration_ms,
			COUNT(*) as run_count
		FROM run_results rr`)
	q.applyRunResultFilters("rr", folderID, startDate, endDate)
	q.GroupBy("strftime('%Y-%m-%d', rr.start_time)").OrderBy("date ASC")

	query, args := q.Build()
	var results []models.DurationTrendPoint
	if err := s.db.Raw(query, args...).Scan(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

// GetMostTimeConsumingTestCases returns top N test cases ranked by duration from most recent run.
func (s *Store) GetMostTimeConsumingTestCases(startDate, endDate time.Time, folderID string, limit int) ([]models.MostTimeConsumingTestCase, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	innerQ := newAnalyticsQuery(`
			SELECT
				rr.test_case_id,
				COALESCE(rr.test_name_snapshot, rr.test_case_id) as test_case_name,
				rr.status,
				rr.duration_ms,
				rr.start_time,
				ROW_NUMBER() OVER (PARTITION BY rr.test_case_id ORDER BY rr.start_time DESC) as rn
			FROM run_results rr`)
	innerQ.applyRunResultFilters("rr", folderID, startDate, endDate)
	innerQ.Where("rr.test_case_id IS NOT NULL")
	innerSQL, args := innerQ.Build()

	query := `
		SELECT
			sub.test_case_id,
			sub.test_case_name,
			sub.status,
			sub.duration_ms,
			sub.start_time
		FROM (` + innerSQL + `
		) sub
		WHERE sub.rn = 1
		ORDER BY sub.duration_ms DESC
		LIMIT ?
	`
	args = append(args, limit)

	var results []models.MostTimeConsumingTestCase
	if err := s.db.Raw(query, args...).Scan(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

// GetComponentHealth returns pass/fail/skip statistics per run folder.
func (s *Store) GetComponentHealth(startDate, endDate time.Time, folderID string) ([]models.ComponentHealthRecord, error) {
	q := newAnalyticsQuery(`
		SELECT
			tr.run_folder_id as folder_id,
			COALESCE(rf.name, 'Ungrouped') as folder_name,
			COUNT(*) as total_tests,
			SUM(CASE WHEN rr.status = 'PASS' THEN 1 ELSE 0 END) as passed_count,
			SUM(CASE WHEN rr.status IN ('FAIL','ERROR') THEN 1 ELSE 0 END) as failed_count,
			SUM(CASE WHEN rr.status IN ('SKIP','PENDING') THEN 1 ELSE 0 END) as skipped_count,
			CASE WHEN COUNT(*) > 0 THEN CAST(SUM(CASE WHEN rr.status = 'PASS' THEN 1 ELSE 0 END) AS REAL) / COUNT(*) * 100 ELSE 0 END as passing_rate
		FROM run_results rr
		JOIN test_runs tr ON rr.test_run_id = tr.id
		LEFT JOIN run_folders rf ON tr.run_folder_id = rf.id`)

	if !startDate.IsZero() {
		q.Where("rr.start_time >= ?", startDate)
	}
	if !endDate.IsZero() {
		q.Where("rr.start_time < ?", endDate)
	}
	if folderID != "" {
		q.Where("tr.run_folder_id = ?", folderID)
	}
	q.GroupBy("tr.run_folder_id").OrderBy("passing_rate ASC")

	query, args := q.Build()
	var results []models.ComponentHealthRecord
	if err := s.db.Raw(query, args...).Scan(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

// GetTestCaseGrowth returns daily deltas and cumulative totals for test case creation.
func (s *Store) GetTestCaseGrowth(startDate, endDate time.Time) ([]models.GrowthDataPoint, error) {
	q := newAnalyticsQuery(`
		SELECT
			strftime('%Y-%m-%d', created_at) as date,
			COUNT(*) as delta
		FROM test_cases`)

	if !startDate.IsZero() {
		q.Where("date(created_at) >= date(?)", startDate)
	}
	if !endDate.IsZero() {
		q.Where("date(created_at) < date(?)", endDate)
	}
	q.GroupBy("strftime('%Y-%m-%d', created_at)").OrderBy("date ASC")

	query, args := q.Build()

	var rawPoints []struct {
		Date  string `gorm:"column:date"`
		Delta int    `gorm:"column:delta"`
	}
	if err := s.db.Raw(query, args...).Scan(&rawPoints).Error; err != nil {
		return nil, err
	}

	var priorCount int64
	if !startDate.IsZero() {
		s.db.Raw("SELECT COUNT(*) FROM test_cases WHERE created_at < ?", startDate).Scan(&priorCount)
	}

	results := make([]models.GrowthDataPoint, 0, len(rawPoints))
	cumulative := int(priorCount)
	for _, rp := range rawPoints {
		cumulative += rp.Delta
		results = append(results, models.GrowthDataPoint{
			Date:       rp.Date,
			TotalCount: cumulative,
			Delta:      rp.Delta,
		})
	}
	return results, nil
}

// GetPassingRatePerFolder returns pass rate for the most recent run in each folder.
func (s *Store) GetPassingRatePerFolder(startDate, endDate time.Time, folderID string, excludeSkipped bool) ([]models.PassingRatePerFolder, error) {
	innerQ := newAnalyticsQuery(`
		SELECT tr.run_folder_id, MAX(tr.created_at) as max_created
		FROM test_runs tr`)

	if !startDate.IsZero() {
		innerQ.Where("tr.created_at >= ?", startDate)
	}
	if !endDate.IsZero() {
		innerQ.Where("tr.created_at < ?", endDate)
	}
	if folderID != "" {
		innerQ.Where("tr.run_folder_id = ?", folderID)
	} else {
		innerQ.Where("tr.run_folder_id IS NOT NULL")
	}
	innerQ.GroupBy("tr.run_folder_id")
	latestRunQuery, args := innerQ.Build()

	query := `
		SELECT
			lr.run_folder_id as folder_id,
			COALESCE(rf.name, 'Ungrouped') as folder_name,
			SUM(CASE WHEN rr.status = 'PASS' THEN 1 ELSE 0 END) as passed_count,
			SUM(CASE WHEN rr.status IN ('FAIL','ERROR') THEN 1 ELSE 0 END) as failed_count,
			SUM(CASE WHEN rr.status IN ('SKIP','PENDING') THEN 1 ELSE 0 END) as skipped_count,
			COUNT(*) as total_count
		FROM (` + latestRunQuery + `) lr
		JOIN test_runs tr2 ON tr2.run_folder_id = lr.run_folder_id AND tr2.created_at = lr.max_created
		JOIN run_results rr ON rr.test_run_id = tr2.id
		LEFT JOIN run_folders rf ON lr.run_folder_id = rf.id
		GROUP BY lr.run_folder_id
	`

	var rawResults []struct {
		FolderID     *string `gorm:"column:folder_id"`
		FolderName   string  `gorm:"column:folder_name"`
		PassedCount  int     `gorm:"column:passed_count"`
		FailedCount  int     `gorm:"column:failed_count"`
		SkippedCount int     `gorm:"column:skipped_count"`
		TotalCount   int     `gorm:"column:total_count"`
	}
	if err := s.db.Raw(query, args...).Scan(&rawResults).Error; err != nil {
		return nil, err
	}

	results := make([]models.PassingRatePerFolder, 0, len(rawResults))
	for _, r := range rawResults {
		denom := r.PassedCount + r.FailedCount
		if !excludeSkipped {
			denom += r.SkippedCount
		}

		rate := 0.0
		if denom > 0 {
			rate = (float64(r.PassedCount) / float64(denom)) * 100
		}

		fid := ""
		if r.FolderID != nil {
			fid = *r.FolderID
		}

		results = append(results, models.PassingRatePerFolder{
			FolderID:        fid,
			FolderName:      r.FolderName,
			PassedCount:     r.PassedCount,
			FailedCount:     r.FailedCount,
			SkippedCount:    r.SkippedCount,
			TotalCount:      r.TotalCount,
			PassingRate:     rate,
			IncludesSkipped: !excludeSkipped,
		})
	}

	return results, nil
}
