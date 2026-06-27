package store

import (
	"encoding/json"
	"ttgo/pkg/tracker/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CreateCustomFieldDefinition creates a new global custom field.
func (s *Store) CreateCustomFieldDefinition(def *models.CustomFieldDefinition) error {
	if def.ID == "" {
		def.ID = uuid.New().String()
	}
	return s.db.Create(def).Error
}

// ListCustomFieldDefinitions returns all custom field definitions.
func (s *Store) ListCustomFieldDefinitions() ([]models.CustomFieldDefinition, error) {
	var defs []models.CustomFieldDefinition
	err := s.db.Find(&defs).Error
	return defs, err
}

// DeleteCustomFieldDefinition deletes a custom field definition and, atomically,
// every CustomFieldValue that referenced it. There is no DB-level FK on
// custom_field_values.custom_field_id, so without this cascade the values would
// be silently orphaned (dangling refs, broken lookups) (F-013).
func (s *Store) DeleteCustomFieldDefinition(id string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("custom_field_id = ?", id).Delete(&models.CustomFieldValue{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.CustomFieldDefinition{}, "id = ?", id).Error
	})
}

// GetCustomFieldValues returns values for a test case.
func (s *Store) GetCustomFieldValues(testCaseID string) ([]models.CustomFieldValue, error) {
	var values []models.CustomFieldValue
	err := s.db.Preload("CustomFieldDef").Where("test_case_id = ?", testCaseID).Find(&values).Error
	return values, err
}

// GetTestCaseByCustomField looks up a test case by custom field name and value.
// The value column is json.RawMessage with gorm:"type:json". GORM/SQLite double-encodes
// the value on write (json.Marshal is called on already-valid JSON bytes), so a string
// "46260677" ends up stored as "\"46260677\"" in the DB. We compare against all possible
// encodings to be safe. Returns gorm.ErrRecordNotFound when no matching test case exists.
func (s *Store) GetTestCaseByCustomField(fieldName string, fieldValue string) (*models.TestCase, error) {
	return s.getTestCaseByCustomFieldTx(s.db, fieldName, fieldValue)
}

func (s *Store) getTestCaseByCustomFieldTx(tx *gorm.DB, fieldName string, fieldValue string) (*models.TestCase, error) {
	jsonOnce, _ := json.Marshal(fieldValue)        // "46260677"
	jsonTwice, _ := json.Marshal(string(jsonOnce)) // "\"46260677\""

	var cv models.CustomFieldValue
	query := tx.
		Joins("JOIN custom_field_definitions ON custom_field_definitions.id = custom_field_values.custom_field_id").
		Where("custom_field_definitions.name = ? AND (CAST(custom_field_values.value AS TEXT) = ? OR CAST(custom_field_values.value AS TEXT) = ? OR CAST(custom_field_values.value AS TEXT) = ?)",
			fieldName, fieldValue, string(jsonOnce), string(jsonTwice)).
		Limit(1).
		Find(&cv)
	if query.Error != nil {
		return nil, query.Error
	}
	if query.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	var test models.TestCase
	if err := tx.Preload("Categories").Preload("Steps").Preload("CustomValues").First(&test, "id = ?", cv.TestCaseID).Error; err != nil {
		return nil, err
	}
	return &test, nil
}
