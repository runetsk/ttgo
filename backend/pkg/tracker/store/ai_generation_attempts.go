package store

import (
	"github.com/google/uuid"
	"ttgo/pkg/tracker/models"
)

// CreateGenerationAttempt appends one attempt row (best-effort at call sites).
func (s *Store) CreateGenerationAttempt(a *models.AIGenerationAttempt) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return s.db.Create(a).Error
}

// ListGenerationAttempts returns a run's attempts oldest-first.
func (s *Store) ListGenerationAttempts(runID string) ([]*models.AIGenerationAttempt, error) {
	var attempts []*models.AIGenerationAttempt
	err := s.db.Where("run_id = ?", runID).
		Order("created_at asc, rowid asc").Find(&attempts).Error
	return attempts, err
}
