package models

import "time"

// RunMetric stores precomputed daily pass/fail analytics per test case.
type RunMetric struct {
	ID            string    `json:"id" gorm:"primaryKey"`
	TestCaseID    string    `json:"test_case_id" gorm:"uniqueIndex:idx_run_metrics_test_date;not null"`
	Date          time.Time `json:"date" gorm:"uniqueIndex:idx_run_metrics_test_date;not null"`
	TotalRuns     int       `json:"total_runs"`
	PassCount     int       `json:"pass_count"`
	FailCount     int       `json:"fail_count"`
	SkipCount     int       `json:"skip_count"`
	PassRate      float64   `json:"pass_rate"`
	FailRate      float64   `json:"fail_rate"`
	AvgDurationMs int64     `json:"avg_duration_ms"`
	ComputedAt    time.Time `json:"computed_at"`
}

// FlakyStat stores the current flakiness assessment for a test case.
type FlakyStat struct {
	ID               string    `json:"id" gorm:"primaryKey"`
	TestCaseID       string    `json:"test_case_id" gorm:"uniqueIndex:idx_flaky_stats_test_id;not null"`
	TotalRuns        int       `json:"total_runs"`
	FailCount        int       `json:"fail_count"`
	FailureRate      float64   `json:"failure_rate"`
	IsFlaky          bool      `json:"is_flaky" gorm:"index"`
	SwitchCount      int       `json:"switch_count"`
	PossibleSwitches int       `json:"possible_switches"`
	SwitchPercentage float64   `json:"switch_percentage"`
	CurrentStatus    string    `json:"current_status"`
	LastSwitchAt     time.Time `json:"last_switch_at"`
	ComputedAt       time.Time `json:"computed_at"`
	CreatedAt        time.Time `json:"created_at"`
}

// MostFailedTestCase is a query-result struct (not persisted).
type MostFailedTestCase struct {
	TestCaseID    string  `json:"test_case_id"`
	TestCaseName  string  `json:"test_case_name"`
	FailedCount   int     `json:"failed_count"`
	TotalRuns     int     `json:"total_runs"`
	FailureRate   float64 `json:"failure_rate"`
	LastFailureAt string  `json:"last_failure_at"`
}

// FlakyTestCase is a query-result struct (not persisted).
type FlakyTestCase struct {
	TestCaseID       string    `json:"test_case_id"`
	TestCaseName     string    `json:"test_case_name"`
	SwitchCount      int       `json:"switch_count"`
	PossibleSwitches int       `json:"possible_switches"`
	SwitchPercentage float64   `json:"switch_percentage"`
	TotalRuns        int       `json:"total_runs"`
	CurrentStatus    string    `json:"current_status"`
	LastSwitchAt     time.Time `json:"last_switch_at"`
}

// DurationTrendPoint is a query-result struct (not persisted).
type DurationTrendPoint struct {
	Date            string  `json:"date"`
	TotalDurationMs int64   `json:"total_duration_ms"`
	AvgDurationMs   float64 `json:"avg_duration_ms"`
	RunCount        int     `json:"run_count"`
}

// MostTimeConsumingTestCase is a query-result struct (not persisted).
type MostTimeConsumingTestCase struct {
	TestCaseID   string    `json:"test_case_id"`
	TestCaseName string    `json:"test_case_name"`
	Status       string    `json:"status"`
	DurationMs   int64     `json:"duration_ms"`
	StartTime    time.Time `json:"start_time"`
}

// ComponentHealthRecord is a query-result struct (not persisted).
type ComponentHealthRecord struct {
	FolderID     *string `json:"folder_id"`
	FolderName   string  `json:"folder_name"`
	TotalTests   int     `json:"total_tests"`
	PassedCount  int     `json:"passed_count"`
	FailedCount  int     `json:"failed_count"`
	SkippedCount int     `json:"skipped_count"`
	PassingRate  float64 `json:"passing_rate"`
}

// GrowthDataPoint is a query-result struct (not persisted).
type GrowthDataPoint struct {
	Date       string `json:"date"`
	TotalCount int    `json:"total_count"`
	Delta      int    `json:"delta"`
}

// PassingRatePerFolder is a query-result struct (not persisted).
type PassingRatePerFolder struct {
	FolderID        string  `json:"folder_id"`
	FolderName      string  `json:"folder_name"`
	PassedCount     int     `json:"passed_count"`
	FailedCount     int     `json:"failed_count"`
	SkippedCount    int     `json:"skipped_count"`
	TotalCount      int     `json:"total_count"`
	PassingRate     float64 `json:"passing_rate"`
	IncludesSkipped bool    `json:"includes_skipped"`
}
