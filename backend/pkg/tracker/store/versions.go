package store

import (
	"encoding/json"
	"fmt"
	"ttgo/pkg/tracker/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// buildSnapshot serializes the versioned fields of a TestCase (name, description, steps,
// categories, custom_values) to the JSON snapshot format. The test case and all associations
// must be preloaded (Steps, Categories, CustomValues.CustomFieldDef) before calling.
func buildSnapshot(tc *models.TestCase) (string, error) {
	snap := models.VersionSnapshot{
		Name:        tc.Name,
		Description: tc.Description,
	}
	for _, s := range tc.Steps {
		snap.Steps = append(snap.Steps, models.VersionStep{
			ID:             s.ID,
			Action:         s.Action,
			ExpectedResult: s.ExpectedResult,
			OrderIndex:     s.OrderIndex,
		})
	}
	for _, category := range tc.Categories {
		snap.Categories = append(snap.Categories, models.VersionCategory{
			ID:   category.ID,
			Name: category.Name,
		})
	}
	for _, cv := range tc.CustomValues {
		sv := models.VersionCustomValue{
			FieldID: cv.CustomFieldID,
			Value:   string(cv.Value),
		}
		if cv.CustomFieldDef != nil {
			sv.FieldName = cv.CustomFieldDef.Name
			sv.FieldType = string(cv.CustomFieldDef.Type)
		}
		snap.CustomValues = append(snap.CustomValues, sv)
	}
	b, err := json.Marshal(snap)
	if err != nil {
		return "", fmt.Errorf("buildSnapshot: %w", err)
	}
	return string(b), nil
}

// createVersionTx inserts a new TestCaseVersion record within an existing transaction.
func createVersionTx(tx *gorm.DB, v *models.TestCaseVersion) error {
	if v.ID == "" {
		v.ID = uuid.New().String()
	}
	return tx.Create(v).Error
}

// DeleteVersionsByTestCase deletes all version records for a test case.
// Must be called within an existing transaction (tx).
func DeleteVersionsByTestCase(tx *gorm.DB, testCaseID string) error {
	return tx.Where("test_case_id = ?", testCaseID).Delete(&models.TestCaseVersion{}).Error
}

// ListVersions returns all version entries for a test case, ordered newest first.
func (s *Store) ListVersions(testCaseID string) ([]*models.TestCaseVersion, error) {
	var versions []*models.TestCaseVersion
	if err := s.db.
		Where("test_case_id = ?", testCaseID).
		Order("created_at DESC").
		Find(&versions).Error; err != nil {
		return nil, err
	}
	return versions, nil
}

// GetVersion returns a single version entry, validating that it belongs to the given test case.
func (s *Store) GetVersion(testCaseID, versionID string) (*models.TestCaseVersion, error) {
	var v models.TestCaseVersion
	if err := s.db.
		Where("id = ? AND test_case_id = ?", versionID, testCaseID).
		First(&v).Error; err != nil {
		return nil, err
	}
	return &v, nil
}

// RestoreTestCase replaces the current test case content with the snapshot from the given version.
// It creates a new "restore" version entry referencing the source version and returns the updated TestCase.
func (s *Store) RestoreTestCase(testCaseID, versionID, userID, userName string) (*models.TestCase, error) {
	var restored *models.TestCase
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// 1. Fetch the target version.
		var src models.TestCaseVersion
		if err := tx.Where("id = ? AND test_case_id = ?", versionID, testCaseID).First(&src).Error; err != nil {
			return fmt.Errorf("version not found: %w", err)
		}

		// 2. Deserialize the snapshot.
		var snap models.VersionSnapshot
		if err := json.Unmarshal([]byte(src.Snapshot), &snap); err != nil {
			return fmt.Errorf("failed to parse snapshot: %w", err)
		}

		// 3. Update the test case's name and description.
		if err := tx.Model(&models.TestCase{}).Where("id = ?", testCaseID).Updates(map[string]interface{}{
			"name":        snap.Name,
			"description": snap.Description,
		}).Error; err != nil {
			return err
		}

		// 4. Rebuild steps from the snapshot (delete-then-insert so existing step
		//    content is always overwritten, not just membership-managed).
		if err := tx.Where("test_case_id = ?", testCaseID).Delete(&models.TestStep{}).Error; err != nil {
			return err
		}
		for _, ss := range snap.Steps {
			id := ss.ID
			if id == "" {
				id = uuid.New().String()
			}
			if err := tx.Create(&models.TestStep{
				ID:             id,
				TestCaseID:     testCaseID,
				Action:         ss.Action,
				ExpectedResult: ss.ExpectedResult,
				OrderIndex:     ss.OrderIndex,
			}).Error; err != nil {
				return err
			}
		}

		// 4b. Rebuild custom values from the snapshot (delete-then-insert).
		if err := tx.Where("test_case_id = ?", testCaseID).Delete(&models.CustomFieldValue{}).Error; err != nil {
			return err
		}
		for _, cv := range snap.CustomValues {
			if cv.FieldID == "" {
				continue
			}
			if err := tx.Create(&models.CustomFieldValue{
				ID:            uuid.New().String(),
				TestCaseID:    testCaseID,
				CustomFieldID: cv.FieldID,
				Value:         json.RawMessage(cv.Value),
			}).Error; err != nil {
				return err
			}
		}

		// 4c. Restore category associations from the snapshot.
		categoryList := make([]*models.Category, 0, len(snap.Categories))
		for _, vs := range snap.Categories {
			categoryList = append(categoryList, &models.Category{ID: vs.ID})
		}
		tc := &models.TestCase{ID: testCaseID}
		if err := tx.Model(tc).Association("Categories").Replace(categoryList); err != nil {
			return err
		}

		// 5. Re-fetch the full updated test case with all associations for the snapshot.
		var full models.TestCase
		if err := tx.Preload("Steps").Preload("Categories").
			Preload("CustomValues.CustomFieldDef").
			First(&full, "id = ?", testCaseID).Error; err != nil {
			return err
		}

		// 6. Record the restore event as a new version entry.
		snapJSON, err := buildSnapshot(&full)
		if err != nil {
			return err
		}
		restoreVersion := &models.TestCaseVersion{
			TestCaseID:            testCaseID,
			EventType:             "restore",
			RestoredFromVersionID: versionID,
			UserID:                userID,
			UserName:              userName,
			Snapshot:              snapJSON,
		}
		if err := createVersionTx(tx, restoreVersion); err != nil {
			return err
		}

		restored = &full
		return nil
	})
	return restored, err
}
