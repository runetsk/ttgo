package store

import (
	"time"

	"ttgo/pkg/tracker/models"
)

// AIGenProviderStats compares providers/models over a report window.
type AIGenProviderStats struct {
	ProviderLabel string  `json:"provider_label"`
	ModelName     string  `json:"model_name"`
	Runs          int     `json:"runs"`
	CompletedRuns int     `json:"completed_runs"`
	ParseFailures int     `json:"parse_failures"`
	TotalTokens   int64   `json:"total_tokens"`
	CostUSD       float64 `json:"cost_usd"`
	AvgDurationMs int64   `json:"avg_duration_ms"`
}

// AIGenerationReport aggregates lifecycle outcomes for the observability
// measures in the design spec.
type AIGenerationReport struct {
	Runs struct {
		Total               int     `json:"total"`
		Completed           int     `json:"completed"`
		Failed              int     `json:"failed"`
		Cancelled           int     `json:"cancelled"`
		ParseFailures       int     `json:"parse_failures"`
		RetriedRuns         int     `json:"retried_runs"`
		TotalTokens         int64   `json:"total_tokens"`
		TotalCostUSD        float64 `json:"total_cost_usd"`
		AvgDurationMs       int64   `json:"avg_duration_ms"`
		P50DurationMs       int64   `json:"p50_duration_ms"`
		P95DurationMs       int64   `json:"p95_duration_ms"`
		AvgDecisionSeconds  float64 `json:"avg_decision_seconds"`  // completed_at -> accept/reject event
		AvgUncoveredTargets float64 `json:"avg_uncovered_targets"` // per completed run with a coverage report
	} `json:"runs"`
	Drafts struct {
		Generated              int `json:"generated"`
		AcceptedUnchanged      int `json:"accepted_unchanged"`
		AcceptedEdited         int `json:"accepted_edited"`
		Rejected               int `json:"rejected"`
		Superseded             int `json:"superseded"`
		Pending                int `json:"pending"`
		AcceptedWithDupWarning int `json:"accepted_with_duplicate_warning"` // possible-duplicate override count
	} `json:"drafts"`
	RejectionReasons map[string]int       `json:"rejection_reasons"`
	Providers        []AIGenProviderStats `json:"providers"`
}

// GetAIGenerationReport aggregates runs, draft outcomes, rejection reasons,
// and provider comparisons in [start, end).
func (s *Store) GetAIGenerationReport(start, end time.Time) (*AIGenerationReport, error) {
	rep := &AIGenerationReport{
		RejectionReasons: map[string]int{},
		Providers:        []AIGenProviderStats{},
	}

	var runAgg struct {
		Total, Completed, Failed, Cancelled, ParseFailures, RetriedRuns int
		TotalTokens                                                     int64
		TotalCost                                                       float64
		AvgDur                                                          float64
	}
	if err := s.db.Raw(`SELECT
			COUNT(*) AS total,
			SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) AS completed,
			SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) AS failed,
			SUM(CASE WHEN status = 'cancelled' THEN 1 ELSE 0 END) AS cancelled,
			SUM(CASE WHEN error_category = 'parse' THEN 1 ELSE 0 END) AS parse_failures,
			SUM(CASE WHEN retry_count > 0 THEN 1 ELSE 0 END) AS retried_runs,
			COALESCE(SUM(total_tokens), 0) AS total_tokens,
			COALESCE(SUM(estimated_cost), 0) AS total_cost,
			COALESCE(AVG(CASE WHEN status = 'completed' THEN duration_ms END), 0) AS avg_dur
		FROM ai_generation_runs
		WHERE created_at >= ? AND created_at < ?`, start, end).Scan(&runAgg).Error; err != nil {
		return nil, err
	}
	rep.Runs.Total = runAgg.Total
	rep.Runs.Completed = runAgg.Completed
	rep.Runs.Failed = runAgg.Failed
	rep.Runs.Cancelled = runAgg.Cancelled
	rep.Runs.ParseFailures = runAgg.ParseFailures
	rep.Runs.RetriedRuns = runAgg.RetriedRuns
	rep.Runs.TotalTokens = runAgg.TotalTokens
	rep.Runs.TotalCostUSD = runAgg.TotalCost
	rep.Runs.AvgDurationMs = int64(runAgg.AvgDur)

	// Nearest-rank percentiles over completed run durations (median + p95).
	if runAgg.Completed > 0 {
		percentile := func(p int) int64 {
			offset := (runAgg.Completed*p + 99) / 100
			if offset > 0 {
				offset--
			}
			var v int64
			if err := s.db.Raw(`SELECT duration_ms FROM ai_generation_runs
				WHERE status = 'completed' AND created_at >= ? AND created_at < ?
				ORDER BY duration_ms LIMIT 1 OFFSET ?`, start, end, offset).Scan(&v).Error; err != nil {
				return 0
			}
			return v
		}
		rep.Runs.P50DurationMs = percentile(50)
		rep.Runs.P95DurationMs = percentile(95)
	}

	// Reviewer decision latency: completed_at -> accept/reject event (seconds).
	if err := s.db.Raw(`SELECT COALESCE(AVG((julianday(e.created_at) - julianday(r.completed_at)) * 86400.0), 0)
		FROM ai_generation_events e
		JOIN ai_generation_runs r ON r.id = e.run_id
		WHERE e.event_type IN ('accepted', 'rejected') AND r.completed_at IS NOT NULL
		  AND r.created_at >= ? AND r.created_at < ?`, start, end).
		Scan(&rep.Runs.AvgDecisionSeconds).Error; err != nil {
		return nil, err
	}

	// Coverage recall proxy: average uncovered targets per completed run
	// (JSON1 json_extract over the stored coverage report).
	if err := s.db.Raw(`SELECT COALESCE(AVG(CAST(json_extract(coverage_json, '$.uncovered_count') AS REAL)), 0)
		FROM ai_generation_runs
		WHERE status = 'completed' AND coverage_json != '' AND created_at >= ? AND created_at < ?`,
		start, end).Scan(&rep.Runs.AvgUncoveredTargets).Error; err != nil {
		return nil, err
	}

	// Possible-duplicate override rate numerator: accepted despite candidates.
	if err := s.db.Raw(`SELECT COUNT(*)
		FROM ai_generated_drafts d
		JOIN ai_generation_runs r ON r.id = d.run_id
		WHERE d.status = 'accepted' AND d.duplicates_json NOT IN ('', '[]', 'null')
		  AND r.created_at >= ? AND r.created_at < ?`, start, end).
		Scan(&rep.Drafts.AcceptedWithDupWarning).Error; err != nil {
		return nil, err
	}

	var draftRows []struct {
		Status string
		Edited bool
		N      int
	}
	if err := s.db.Raw(`SELECT d.status AS status, d.edited AS edited, COUNT(*) AS n
		FROM ai_generated_drafts d
		JOIN ai_generation_runs r ON r.id = d.run_id
		WHERE r.created_at >= ? AND r.created_at < ?
		GROUP BY d.status, d.edited`, start, end).Scan(&draftRows).Error; err != nil {
		return nil, err
	}
	for _, row := range draftRows {
		rep.Drafts.Generated += row.N
		switch row.Status {
		case models.AIDraftStatusAccepted:
			if row.Edited {
				rep.Drafts.AcceptedEdited += row.N
			} else {
				rep.Drafts.AcceptedUnchanged += row.N
			}
		case models.AIDraftStatusRejected:
			rep.Drafts.Rejected += row.N
		case models.AIDraftStatusSuperseded:
			rep.Drafts.Superseded += row.N
		case models.AIDraftStatusPending:
			rep.Drafts.Pending += row.N
		}
	}

	var reasonRows []struct {
		Reason string
		N      int
	}
	if err := s.db.Raw(`SELECT e.reason AS reason, COUNT(*) AS n
		FROM ai_generation_events e
		JOIN ai_generation_runs r ON r.id = e.run_id
		WHERE e.event_type = 'rejected' AND r.created_at >= ? AND r.created_at < ?
		GROUP BY e.reason`, start, end).Scan(&reasonRows).Error; err != nil {
		return nil, err
	}
	for _, row := range reasonRows {
		if row.Reason != "" {
			rep.RejectionReasons[row.Reason] = row.N
		}
	}

	var provRows []struct {
		ProviderLabel string
		ModelName     string
		Runs          int
		CompletedRuns int
		ParseFailures int
		TotalTokens   int64
		Cost          float64
		AvgDur        float64
	}
	if err := s.db.Raw(`SELECT provider_label, model_name,
			COUNT(*) AS runs,
			SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) AS completed_runs,
			SUM(CASE WHEN error_category = 'parse' THEN 1 ELSE 0 END) AS parse_failures,
			COALESCE(SUM(total_tokens), 0) AS total_tokens,
			COALESCE(SUM(estimated_cost), 0) AS cost,
			COALESCE(AVG(CASE WHEN status = 'completed' THEN duration_ms END), 0) AS avg_dur
		FROM ai_generation_runs
		WHERE created_at >= ? AND created_at < ?
		GROUP BY provider_label, model_name
		ORDER BY runs DESC`, start, end).Scan(&provRows).Error; err != nil {
		return nil, err
	}
	for _, row := range provRows {
		rep.Providers = append(rep.Providers, AIGenProviderStats{
			ProviderLabel: row.ProviderLabel, ModelName: row.ModelName,
			Runs: row.Runs, CompletedRuns: row.CompletedRuns, ParseFailures: row.ParseFailures,
			TotalTokens: row.TotalTokens, CostUSD: row.Cost, AvgDurationMs: int64(row.AvgDur),
		})
	}
	return rep, nil
}
