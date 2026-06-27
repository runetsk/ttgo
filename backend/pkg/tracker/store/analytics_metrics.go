package store

import (
	"time"
	"ttgo/pkg/tracker/models"

	"github.com/google/uuid"
)

// DetectFlakyTests finds tests with failure_rate > threshold and >= 10 runs.
func (s *Store) DetectFlakyTests(threshold float64) ([]models.FlakyStat, error) {
	err := s.db.Exec(`
		INSERT INTO flaky_stats (id, test_case_id, total_runs, fail_count, failure_rate, is_flaky, computed_at, created_at)
		SELECT
			lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))),2) || '-' || substr('89ab',abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))),2) || '-' || lower(hex(randomblob(6))) as id,
			test_case_id,
			min(COUNT(*), 50) as total_runs,
			SUM(CASE WHEN status IN ('FAIL','ERROR') THEN 1 ELSE 0 END) as fail_count,
			CAST(SUM(CASE WHEN status IN ('FAIL','ERROR') THEN 1 ELSE 0 END) AS REAL) / min(COUNT(*), 50) as failure_rate,
			CASE WHEN CAST(SUM(CASE WHEN status IN ('FAIL','ERROR') THEN 1 ELSE 0 END) AS REAL) / min(COUNT(*), 50) > ? AND COUNT(*) >= 10 THEN 1 ELSE 0 END as is_flaky,
			datetime('now') as computed_at,
			datetime('now') as created_at
		FROM (
			SELECT test_case_id, status, ROW_NUMBER() OVER (PARTITION BY test_case_id ORDER BY start_time DESC) as rn
			FROM run_results WHERE test_case_id IS NOT NULL
		) recent
		WHERE rn <= 50
		GROUP BY test_case_id
		HAVING COUNT(*) >= 10
		ON CONFLICT(test_case_id) DO UPDATE SET
			total_runs=excluded.total_runs,
			fail_count=excluded.fail_count,
			failure_rate=excluded.failure_rate,
			is_flaky=excluded.is_flaky,
			computed_at=excluded.computed_at
	`, threshold).Error
	if err != nil {
		return nil, err
	}

	var stats []models.FlakyStat
	if err := s.db.Where("is_flaky = ?", true).Order("failure_rate DESC").Find(&stats).Error; err != nil {
		return nil, err
	}
	return stats, nil
}

// ComputeDailyMetrics precomputes RunMetric rows for the given date.
func (s *Store) ComputeDailyMetrics(date time.Time) error {
	day := date.UTC().Truncate(24 * time.Hour)

	type row struct {
		TestCaseID string
		Total      int
		Pass       int
		Fail       int
		Skip       int
		AvgDur     int64
	}

	var rows []row
	err := s.db.Raw(`
		SELECT
			test_case_id,
			COUNT(*) as total,
			SUM(CASE WHEN status = 'PASS' THEN 1 ELSE 0 END) as pass,
			SUM(CASE WHEN status IN ('FAIL','ERROR') THEN 1 ELSE 0 END) as fail,
			SUM(CASE WHEN status IN ('SKIP','PENDING') THEN 1 ELSE 0 END) as skip,
			COALESCE(AVG(duration_ms),0) as avg_dur
		FROM run_results
		WHERE test_case_id IS NOT NULL
		  AND date(start_time) = date(?)
		GROUP BY test_case_id
	`, day).Scan(&rows).Error
	if err != nil {
		return err
	}

	for _, r := range rows {
		passRate := 0.0
		failRate := 0.0
		if r.Total > 0 {
			passRate = float64(r.Pass) / float64(r.Total)
			failRate = float64(r.Fail) / float64(r.Total)
		}

		metric := models.RunMetric{
			ID:            uuid.New().String(),
			TestCaseID:    r.TestCaseID,
			Date:          day,
			TotalRuns:     r.Total,
			PassCount:     r.Pass,
			FailCount:     r.Fail,
			SkipCount:     r.Skip,
			PassRate:      passRate,
			FailRate:      failRate,
			AvgDurationMs: r.AvgDur,
			ComputedAt:    time.Now(),
		}
		if err := s.db.Where("test_case_id = ? AND date = ?", r.TestCaseID, day).
			Assign(metric).
			FirstOrCreate(&metric).Error; err != nil {
			return err
		}
	}

	return nil
}
