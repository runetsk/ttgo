package store

import (
	"time"
	"ttgo/pkg/tracker/models"

	"github.com/google/uuid"
)

// RecordAnalysisCostEvent appends one billable failure-analysis call to the cost ledger.
// CreatedAt defaults to now: spend is dated when the call was made.
func (s *Store) RecordAnalysisCostEvent(e *models.AIAnalysisCostEvent) error {
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}
	return s.db.Create(e).Error
}

// ListAnalysisCostEventsForRun returns a test run's cost events, oldest first.
func (s *Store) ListAnalysisCostEventsForRun(runID string) ([]*models.AIAnalysisCostEvent, error) {
	var out []*models.AIAnalysisCostEvent
	err := s.db.Where("run_id = ?", runID).Order("created_at asc, rowid asc").Find(&out).Error
	return out, err
}

// SumAnalysisCostSince totals the priced failure-analysis spend since t (monthly budget).
func (s *Store) SumAnalysisCostSince(t time.Time) (float64, error) {
	var sum float64
	err := s.db.Raw(`SELECT COALESCE(SUM(estimated_cost), 0) FROM ai_analysis_cost_events
		WHERE created_at >= ? AND estimated_cost IS NOT NULL`, t).Scan(&sum).Error
	return sum, err
}

// AnalysisCostSummary is the failure-analysis spend of a report window, by engine, with the
// part spent on later explanations called out. Events counts unpriced calls too.
type AnalysisCostSummary struct {
	CostUSD         float64 `json:"cost_usd"`
	TypeSafeCostUSD float64 `json:"typesafe_cost_usd"`
	LLMCostUSD      float64 `json:"llm_cost_usd"`
	ExplainCostUSD  float64 `json:"explain_cost_usd"`
	Events          int     `json:"events"`
}

// AnalysisCostReport sums the cost events created in [start, end).
func (s *Store) AnalysisCostReport(start, end time.Time) (AnalysisCostSummary, error) {
	var row struct {
		Events                                  int
		TotalCost, TsCost, LlmCost, ExplainCost float64
	}
	err := s.db.Raw(`SELECT COUNT(*) AS events,
			COALESCE(SUM(estimated_cost), 0) AS total_cost,
			COALESCE(SUM(CASE WHEN engine = 'typesafe' THEN estimated_cost END), 0) AS ts_cost,
			COALESCE(SUM(CASE WHEN engine = 'llm' THEN estimated_cost END), 0) AS llm_cost,
			COALESCE(SUM(CASE WHEN kind = 'explain' THEN estimated_cost END), 0) AS explain_cost
		FROM ai_analysis_cost_events
		WHERE created_at >= ? AND created_at < ?`, start, end).Scan(&row).Error
	if err != nil {
		return AnalysisCostSummary{}, err
	}
	return AnalysisCostSummary{CostUSD: row.TotalCost, TypeSafeCostUSD: row.TsCost, LLMCostUSD: row.LlmCost,
		ExplainCostUSD: row.ExplainCost, Events: row.Events}, nil
}
