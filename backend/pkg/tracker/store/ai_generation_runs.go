package store

import (
	"errors"
	"time"
	"ttgo/pkg/tracker/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Sentinel errors for the generation lifecycle (mapped to HTTP codes in the API layer).
var (
	ErrDraftNotPending = errors.New("draft is not in pending status")
	ErrDraftInvalid    = errors.New("draft has validation errors")
	ErrUnknownDrafts   = errors.New("one or more drafts do not belong to this run")
)

// CreateGenerationRun inserts run, OR returns the existing run holding the same
// idempotency key (created==false). The unique index on idempotency_key is the
// real guard against concurrent duplicate inserts: the loser re-reads the winner.
func (s *Store) CreateGenerationRun(run *models.AIGenerationRun) (*models.AIGenerationRun, bool, error) {
	if run.ID == "" {
		run.ID = uuid.New().String()
	}
	if run.IdempotencyKey == "" {
		run.IdempotencyKey = run.ID
	}
	if run.Status == "" {
		run.Status = models.AIGenerationRunStatusPending
	}
	existingByKey := func() (*models.AIGenerationRun, error) {
		var r models.AIGenerationRun
		if err := s.db.First(&r, "idempotency_key = ?", run.IdempotencyKey).Error; err != nil {
			return nil, err
		}
		return &r, nil
	}
	if existing, err := existingByKey(); err == nil {
		return existing, false, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}
	if err := s.db.Create(run).Error; err != nil {
		if existing, e2 := existingByKey(); e2 == nil {
			return existing, false, nil
		}
		return nil, false, err
	}
	return run, true, nil
}

// GetGenerationRunByKey returns the run holding an idempotency key, or (nil, nil).
func (s *Store) GetGenerationRunByKey(key string) (*models.AIGenerationRun, error) {
	var run models.AIGenerationRun
	err := s.db.First(&run, "idempotency_key = ?", key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// GetGenerationRun fetches a run by ID.
func (s *Store) GetGenerationRun(id string) (*models.AIGenerationRun, error) {
	var run models.AIGenerationRun
	if err := s.db.First(&run, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &run, nil
}

// GetGenerationRunWithDrafts fetches a run and populates Drafts ordered by position.
func (s *Store) GetGenerationRunWithDrafts(id string) (*models.AIGenerationRun, error) {
	run, err := s.GetGenerationRun(id)
	if err != nil {
		return nil, err
	}
	if err := s.db.Where("run_id = ?", id).Order("position asc").Find(&run.Drafts).Error; err != nil {
		return nil, err
	}
	return run, nil
}

// ListGenerationRuns returns a requirement's runs, newest first.
func (s *Store) ListGenerationRuns(requirementID string, limit int) ([]*models.AIGenerationRun, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var runs []*models.AIGenerationRun
	err := s.db.Where("requirement_id = ?", requirementID).
		Order("created_at DESC").Limit(limit).Find(&runs).Error
	return runs, err
}

// MarkGenerationRunRunning transitions the run to running and stamps StartedAt.
func (s *Store) MarkGenerationRunRunning(id string) error {
	now := time.Now()
	return s.db.Model(&models.AIGenerationRun{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     models.AIGenerationRunStatusRunning,
			"started_at": &now,
		}).Error
}

// UpdateGenerationRun persists all fields of run (single-writer synchronous flow).
func (s *Store) UpdateGenerationRun(run *models.AIGenerationRun) error {
	return s.db.Save(run).Error
}
