package store

import (
	"ttgo/pkg/tracker/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *Store) CreateCategory(name, description string) (*models.Category, error) {
	category := &models.Category{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
	}
	if err := s.db.Create(category).Error; err != nil {
		return nil, err
	}
	return category, nil
}

func (s *Store) ListCategories(limit, offset int, search string) ([]*models.Category, int64, error) {
	var categories []*models.Category
	var total int64
	base := s.db.Model(&models.Category{})
	if search != "" {
		like := "%" + search + "%"
		base = base.Where("name LIKE ? OR description LIKE ?", like, like)
	}
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	query := base.Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit).Offset(offset)
	}
	if err := query.Find(&categories).Error; err != nil {
		return nil, 0, err
	}
	return categories, total, nil
}

func (s *Store) AssignCategoryToTest(categoryID, testCaseID string) error {
	// Use clauses.OnConflict to ignore if already exists?
	// SQLite support for ON CONFLICT DO NOTHING
	return s.db.Exec("INSERT OR IGNORE INTO suite_test_cases (suite_id, test_case_id) VALUES (?, ?)", categoryID, testCaseID).Error
}

func (s *Store) RemoveCategoryFromTest(categoryID, testCaseID string) error {
	return s.db.Delete(&models.CategoryTestCase{}, "suite_id = ? AND test_case_id = ?", categoryID, testCaseID).Error
}

func (s *Store) DeleteCategory(id string) error {
	return s.DeleteCategories([]string{id})
}

func (s *Store) DeleteCategories(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&models.Category{}, "id IN ?", ids).Error; err != nil {
			return err
		}
		if err := tx.Delete(&models.CategoryTestCase{}, "suite_id IN ?", ids).Error; err != nil {
			return err
		}
		return nil
	})
}
