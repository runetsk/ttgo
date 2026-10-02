package store

import (
	"ttgo/pkg/tracker/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TestCaseFilter struct {
	FolderIDs  []string
	CategoryID *string
	// ListView skips Preload("Steps") and Preload("CustomValues") and populates
	// TestCase.StepsCount via a single aggregate query instead. Use this when the
	// caller only needs step counts (e.g. the test grid UI).
	ListView bool
}

func (s *Store) CreateTestCase(test *models.TestCase) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		return s.createTestCaseTx(tx, test)
	})
}

func (s *Store) createTestCaseTx(tx *gorm.DB, test *models.TestCase) error {
	if test.ID == "" {
		test.ID = uuid.New().String()
	}
	// Ensure IDs for steps if present
	for _, step := range test.Steps {
		if step.ID == "" {
			step.ID = uuid.New().String()
		}
		step.TestCaseID = test.ID
	}
	// Ensure IDs for custom values
	for _, cv := range test.CustomValues {
		if cv.ID == "" {
			cv.ID = uuid.New().String()
		}
		cv.TestCaseID = test.ID
	}
	if err := tx.Create(test).Error; err != nil {
		return err
	}
	// Re-fetch with all associations so the snapshot captures categories and custom fields.
	var full models.TestCase
	if err := tx.Preload("Steps").Preload("Categories").
		Preload("CustomValues.CustomFieldDef").
		First(&full, "id = ?", test.ID).Error; err != nil {
		return err
	}
	snapJSON, err := buildSnapshot(&full)
	if err != nil {
		return err
	}
	return createVersionTx(tx, &models.TestCaseVersion{
		TestCaseID: test.ID,
		EventType:  "create",
		Snapshot:   snapJSON,
	})
}

func (s *Store) UpdateTestCase(test *models.TestCase) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		return s.updateTestCaseTx(tx, test)
	})
}

func (s *Store) updateTestCaseTx(tx *gorm.DB, test *models.TestCase) error {
	if err := tx.Model(test).Updates(test).Error; err != nil {
		return err
	}

	// If steps are provided, replace them
	if test.Steps != nil {
		for _, step := range test.Steps {
			if step.ID == "" {
				step.ID = uuid.New().String()
			}
			step.TestCaseID = test.ID
		}
		if err := tx.Model(test).Association("Steps").Replace(test.Steps); err != nil {
			return err
		}
	}

	// If custom values are provided, replace them.
	// We use a manual delete-then-insert rather than Association.Replace so
	// that the `value` column of already-existing records is actually updated
	// (GORM's Replace only manages FK membership, not field updates).
	if test.CustomValues != nil {
		if err := tx.Where("test_case_id = ?", test.ID).Delete(&models.CustomFieldValue{}).Error; err != nil {
			return err
		}
		for _, cv := range test.CustomValues {
			if cv.ID == "" {
				cv.ID = uuid.New().String()
			}
			cv.TestCaseID = test.ID
			if err := tx.Create(cv).Error; err != nil {
				return err
			}
		}
	}

	// If categories are provided, replace them
	if test.Categories != nil {
		if err := tx.Model(test).Association("Categories").Replace(test.Categories); err != nil {
			return err
		}
	}

	// Re-fetch with all associations so the snapshot captures categories and custom fields.
	var full models.TestCase
	if err := tx.Preload("Steps").Preload("Categories").
		Preload("CustomValues.CustomFieldDef").
		First(&full, "id = ?", test.ID).Error; err != nil {
		return err
	}
	// Merge updated top-level fields into the full record for snapshotting.
	full.Name = test.Name
	full.Description = test.Description
	if test.Steps != nil {
		full.Steps = test.Steps
	}

	snapJSON, err := buildSnapshot(&full)
	if err != nil {
		return err
	}
	return createVersionTx(tx, &models.TestCaseVersion{
		TestCaseID: test.ID,
		EventType:  "edit",
		Snapshot:   snapJSON,
	})
}

func (s *Store) ListTestCases(filter TestCaseFilter) ([]*models.TestCase, error) {
	query := s.db.Model(&models.TestCase{})

	if len(filter.FolderIDs) > 0 {
		var allDescendants []string
		for _, fid := range filter.FolderIDs {
			if fid == "" {
				continue
			}
			descendants, err := s.GetFolderDescendants(fid)
			if err != nil {
				return nil, err
			}
			allDescendants = append(allDescendants, descendants...)
		}

		// Uniq
		uMap := make(map[string]bool)
		var uDesc []string
		for _, d := range allDescendants {
			if !uMap[d] {
				uMap[d] = true
				uDesc = append(uDesc, d)
			}
		}

		if len(uDesc) > 0 {
			query = query.Where("folder_id IN ?", uDesc)
		}
	}

	if filter.CategoryID != nil && *filter.CategoryID != "" {
		query = query.Joins("JOIN suite_test_cases ON suite_test_cases.test_case_id = test_cases.id").
			Where("suite_test_cases.suite_id = ?", *filter.CategoryID)
	}

	var tests []*models.TestCase
	if err := query.Find(&tests).Error; err != nil {
		return nil, err
	}
	if err := s.attachTestCaseAssociations(tests, !filter.ListView); err != nil {
		return nil, err
	}

	if len(tests) > 0 {
		// Collect test case IDs for bulk lookups.
		tcIDs := make([]string, len(tests))
		tcMap := make(map[string]*models.TestCase, len(tests))
		for i, tc := range tests {
			tcIDs[i] = tc.ID
			tcMap[tc.ID] = tc
		}

		// Populate linked requirements in bulk. This and the counts below go in id chunks too.
		type reqLink struct {
			TestCaseID string
			models.Requirement
		}
		links, _ := gatherInChunks(tcIDs, func(chunk []string, part *[]reqLink) error {
			return s.db.Model(&models.Requirement{}).
				Select("requirement_test_case_links.test_case_id, requirements.*").
				Joins("JOIN requirement_test_case_links ON requirement_test_case_links.requirement_id = requirements.id").
				Where("requirement_test_case_links.test_case_id IN ?", chunk).
				Order("requirements.identifier").
				Scan(part).Error
		})
		for _, l := range links {
			tc := tcMap[l.TestCaseID]
			if tc == nil {
				continue
			}
			req := l.Requirement
			tc.LinkedRequirements = append(tc.LinkedRequirements, &req)
		}

		// List view: populate StepsCount in one aggregate query instead of preloading full step rows.
		if filter.ListView {
			type stepCount struct {
				TestCaseID string
				Count      int
			}
			stepCounts, _ := gatherInChunks(tcIDs, func(chunk []string, part *[]stepCount) error {
				return s.db.Model(&models.TestStep{}).
					Where("test_case_id IN ?", chunk).
					Select("test_case_id, count(*) as count").
					Group("test_case_id").
					Scan(part).Error
			})
			for _, sc := range stepCounts {
				if tc := tcMap[sc.TestCaseID]; tc != nil {
					tc.StepsCount = sc.Count
				}
			}
		}

		// Populate open/closed defect counts per test case.
		type cnt struct {
			TestCaseID string
			Status     string
			N          int
		}
		counts, _ := gatherInChunks(tcIDs, func(chunk []string, part *[]cnt) error {
			return s.db.Raw(`
				SELECT dl.test_case_id, d.status, COUNT(DISTINCT d.id) as n
				FROM defect_links dl JOIN defects d ON d.id = dl.defect_id
				WHERE dl.test_case_id IN ? GROUP BY dl.test_case_id, d.status`, chunk).Scan(part).Error
		})
		openByTC, closedByTC := map[string]int{}, map[string]int{}
		for _, c := range counts {
			if c.Status == "closed" {
				closedByTC[c.TestCaseID] += c.N
			} else {
				openByTC[c.TestCaseID] += c.N
			}
		}
		for i := range tests {
			tests[i].OpenDefectCount = openByTC[tests[i].ID]
			tests[i].ClosedDefectCount = closedByTC[tests[i].ID]
		}
	}

	return tests, nil
}

// attachTestCaseAssociations loads the listing's associations: Categories always, Steps and
// CustomValues for the full view. They are preloaded onto id-only stand-ins in id chunks and
// copied across, rather than preloaded with the listing itself: GORM binds every key of a preload
// in one IN, so a listing of more test cases than SQLite binds in a statement failed outright.
func (s *Store) attachTestCaseAssociations(tests []*models.TestCase, full bool) error {
	ids := make([]string, len(tests))
	for i, tc := range tests {
		ids[i] = tc.ID
	}
	stubs, err := gatherInChunks(ids, func(chunk []string, part *[]*models.TestCase) error {
		q := s.db.Select("id").Preload("Categories")
		if full {
			q = q.Preload("Steps").Preload("CustomValues")
		}
		return q.Where("id IN ?", chunk).Find(part).Error
	})
	if err != nil {
		return err
	}
	byID := make(map[string]*models.TestCase, len(stubs))
	for _, stub := range stubs {
		byID[stub.ID] = stub
	}
	for _, tc := range tests {
		stub := byID[tc.ID]
		if stub == nil {
			continue
		}
		tc.Categories = stub.Categories
		if full {
			tc.Steps, tc.CustomValues = stub.Steps, stub.CustomValues
		}
	}
	return nil
}

func (s *Store) GetTestCase(id string) (*models.TestCase, error) {
	var test models.TestCase
	if err := s.db.Preload("Categories").Preload("Steps").Preload("CustomValues").First(&test, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &test, nil
}

// GetTestCasesByIDs fetches multiple test cases by their IDs with all associations.
// Non-existent IDs are silently skipped.
func (s *Store) GetTestCasesByIDs(ids []string) ([]*models.TestCase, error) {
	if len(ids) == 0 {
		return []*models.TestCase{}, nil
	}

	var tests []*models.TestCase
	err := s.db.
		Preload("Categories").
		Preload("Steps").
		Preload("CustomValues.CustomFieldDef").
		Where("id IN ?", ids).
		Find(&tests).Error
	if err != nil {
		return nil, err
	}

	if len(tests) == 0 {
		return tests, nil
	}

	// Populate linked requirements in bulk (same pattern as ListTestCases).
	tcIDs := make([]string, len(tests))
	tcMap := make(map[string]*models.TestCase, len(tests))
	for i, tc := range tests {
		tcIDs[i] = tc.ID
		tcMap[tc.ID] = tc
	}

	type reqLink struct {
		TestCaseID string
		models.Requirement
	}
	var links []reqLink
	_ = s.db.Model(&models.Requirement{}).
		Select("requirement_test_case_links.test_case_id, requirements.*").
		Joins("JOIN requirement_test_case_links ON requirement_test_case_links.requirement_id = requirements.id").
		Where("requirement_test_case_links.test_case_id IN ?", tcIDs).
		Order("requirements.identifier").
		Scan(&links).Error
	for _, l := range links {
		tc := tcMap[l.TestCaseID]
		if tc == nil {
			continue
		}
		req := l.Requirement
		tc.LinkedRequirements = append(tc.LinkedRequirements, &req)
	}

	return tests, nil
}

// DeleteTestCase deletes a single test case by ID.
func (s *Store) DeleteTestCase(id string) error {
	return s.DeleteTestCases([]string{id})
}

// DeleteTestCases deletes multiple test cases in a single transaction.
// Run results are preserved with a NULL test_case_id (name snapshot is kept).
// Category associations, steps, custom values, and version history are removed.
func (s *Store) DeleteTestCases(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		return deleteTestCasesTx(tx, ids)
	})
}

// deleteTestCasesTx performs the test-case deletion cascade within an existing
// transaction, so callers (e.g. folder deletion) can compose it atomically (F-015).
// The ids are chunked: a folder subtree can hold more test cases than SQLite binds
// in one statement.
func deleteTestCasesTx(tx *gorm.DB, ids []string) error {
	for _, chunk := range idChunks(ids) {
		if err := deleteTestCaseChunkTx(tx, chunk); err != nil {
			return err
		}
	}
	return nil
}

// deleteTestCaseChunkTx is deleteTestCasesTx for ids that fit in one statement.
func deleteTestCaseChunkTx(tx *gorm.DB, ids []string) error {
	// Preserve run history: NULL out the FK so RunResults are kept with their name snapshot.
	if err := tx.Model(&models.RunResult{}).Where("test_case_id IN ?", ids).Update("test_case_id", nil).Error; err != nil {
		return err
	}
	// Remove category junction rows.
	if err := tx.Delete(&models.CategoryTestCase{}, "test_case_id IN ?", ids).Error; err != nil {
		return err
	}
	// Remove version history for each test case (FR-012).
	for _, id := range ids {
		if err := DeleteVersionsByTestCase(tx, id); err != nil {
			return err
		}
	}
	// Remove traceability links (007-req-traceability).
	if err := tx.Delete(&models.RequirementTestCaseLink{}, "test_case_id IN ?", ids).Error; err != nil {
		return err
	}
	// Remove defect links (008-jira-integration, FR-014 cascade delete).
	// Test-case-scoped links are deleted; result-scoped links have test_case_id nulled to preserve run history.
	if err := tx.Where("test_case_id IN ? AND run_result_id IS NULL", ids).Delete(&models.DefectLink{}).Error; err != nil {
		return err
	}
	if err := tx.Model(&models.DefectLink{}).Where("test_case_id IN ? AND run_result_id IS NOT NULL", ids).
		Update("test_case_id", nil).Error; err != nil {
		return err
	}
	// Delete test cases — steps and custom_values cascade via GORM constraints.
	return tx.Delete(&models.TestCase{}, "id IN ?", ids).Error
}
