package store

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"ttgo/pkg/tracker/models"
)

const aiBudgetSingletonID = "singleton"

// GetOrCreateAIBudgetSettings reads the budget singleton, seeding zeros.
func (s *Store) GetOrCreateAIBudgetSettings() (*models.AIBudgetSettings, error) {
	var cfg models.AIBudgetSettings
	err := s.db.First(&cfg, "id = ?", aiBudgetSingletonID).Error
	if err == nil {
		return &cfg, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	cfg = models.AIBudgetSettings{ID: aiBudgetSingletonID}
	if err := s.db.Create(&cfg).Error; err != nil {
		return nil, err
	}
	return &cfg, nil
}

// UpdateAIBudgetSettings writes via map so zero values persist.
func (s *Store) UpdateAIBudgetSettings(updates map[string]interface{}) (*models.AIBudgetSettings, error) {
	if _, err := s.GetOrCreateAIBudgetSettings(); err != nil {
		return nil, err
	}
	updates["updated_at"] = time.Now()
	if err := s.db.Model(&models.AIBudgetSettings{}).
		Where("id = ?", aiBudgetSingletonID).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetOrCreateAIBudgetSettings()
}

// MonthStartUTC is the start of the calendar month (UTC) the monthly budget counts from.
func MonthStartUTC(now time.Time) time.Time {
	now = now.UTC()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// SumGenerationCostSince totals per-attempt configured generation cost since t.
// Attempts are the append-only cost ledger, stamped when each call occurs — so
// regenerating an OLD run correctly counts toward the current month.
func (s *Store) SumGenerationCostSince(t time.Time) (float64, error) {
	var sum float64
	err := s.db.Raw(`SELECT COALESCE(SUM(estimated_cost), 0) FROM ai_generation_attempts
		WHERE created_at >= ? AND estimated_cost IS NOT NULL`, t).Scan(&sum).Error
	return sum, err
}

// SumEstimatedCostSince is the spend the monthly budget counts: generation attempts plus
// failure-analysis cost events, both dated when each call was made.
func (s *Store) SumEstimatedCostSince(t time.Time) (float64, error) {
	gen, err := s.SumGenerationCostSince(t)
	if err != nil {
		return 0, err
	}
	analysis, err := s.SumAnalysisCostSince(t)
	if err != nil {
		return 0, err
	}
	return gen + analysis, nil
}
