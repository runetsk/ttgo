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

// SumEstimatedCostSince totals configured run costs since t (monthly budget).
func (s *Store) SumEstimatedCostSince(t time.Time) (float64, error) {
	var sum float64
	err := s.db.Raw(`SELECT COALESCE(SUM(estimated_cost), 0) FROM ai_generation_runs
		WHERE created_at >= ? AND estimated_cost IS NOT NULL`, t).Scan(&sum).Error
	return sum, err
}
