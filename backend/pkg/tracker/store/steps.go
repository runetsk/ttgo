package store

import (
	"ttgo/pkg/tracker/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AddTestStep adds a new step to a test case.
func (s *Store) AddTestStep(step *models.TestStep) error {
	if step.ID == "" {
		step.ID = uuid.New().String()
	}
	return s.db.Create(step).Error
}

// UpdateTestStep updates an existing test step.
func (s *Store) UpdateTestStep(step *models.TestStep) error {
	return s.db.Save(step).Error
}

// DeleteTestStep removes a test step by ID.
func (s *Store) DeleteTestStep(id string) error {
	return s.db.Delete(&models.TestStep{}, "id = ?", id).Error
}

// GetTestSteps returns all steps for a test case ordered by index.
func (s *Store) GetTestSteps(testCaseID string) ([]models.TestStep, error) {
	var steps []models.TestStep
	err := s.db.Where("test_case_id = ?", testCaseID).Order("order_index asc").Find(&steps).Error
	return steps, err
}

// UpdateTestStepsOrder updates the order of multiple steps in a transaction.
func (s *Store) UpdateTestStepsOrder(steps []models.TestStep) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		for _, step := range steps {
			if err := tx.Model(&models.TestStep{}).Where("id = ?", step.ID).Update("order_index", step.OrderIndex).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
