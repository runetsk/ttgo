package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"ttgo/pkg/tracker/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UpdateTestRun updates the provided fields of a test run.
// Pointer fields: nil means "not provided" (skip), non-nil means "set to this value" (including empty string to clear).
func (s *Store) UpdateTestRun(runID string, name *string, categoryID *string, status *string) error {
	updates := map[string]interface{}{}
	if name != nil {
		updates["name"] = *name
	}
	if categoryID != nil {
		updates["category_id"] = *categoryID
	}
	if status != nil {
		updates["status"] = *status
		updates["updated_at"] = time.Now()
	}
	if len(updates) == 0 {
		return nil
	}
	return s.db.Model(&models.TestRun{}).Where("id = ?", runID).Updates(updates).Error
}

// CreateTestRun creates a new test run, optionally based on a Test Category.
// When a category is provided it snapshots all its tests into individual RunResults.
// When no category is given an empty run is created.
func (s *Store) CreateTestRun(run *models.TestRun) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var categoryName string
		var testCases []models.TestCase

		if run.CategoryID != nil && *run.CategoryID != "" {
			// Validate Category existence and fetch associated test cases
			var category models.Category
			if err := tx.First(&category, "id = ?", *run.CategoryID).Error; err != nil {
				return err
			}
			categoryName = category.Name
			if err := tx.Model(&category).Association("TestCases").Find(&testCases); err != nil {
				return err
			}
		}

		// Prepare TestRun data
		if run.ID == "" {
			run.ID = uuid.New().String()
		}
		run.Status = models.StatusPending
		run.CreatedAt = time.Now()
		run.UpdatedAt = time.Now()
		// Default name if empty
		if run.Name == "" {
			if categoryName != "" {
				run.Name = fmt.Sprintf("%s Run - %s", categoryName, run.CreatedAt.Format("2006-01-02 15:04"))
			} else {
				run.Name = fmt.Sprintf("Empty Run - %s", run.CreatedAt.Format("2006-01-02 15:04"))
			}
		}

		// Create TestRun record
		if err := tx.Create(run).Error; err != nil {
			return err
		}

		// Create RunResults (Snapshot) — only when a category was selected
		if len(testCases) > 0 {
			now := time.Now()
			var results []models.RunResult
			for _, tc := range testCases {
				testID := tc.ID
				results = append(results, models.RunResult{
					ID:               uuid.New().String(),
					TestRunID:        run.ID,
					TestCaseID:       &testID,
					AttemptNumber:    1,
					TestNameSnapshot: tc.Name,
					Status:           models.StatusPending,
					CreatedAt:        now,
					UpdatedAt:        now,
				})
			}
			if err := tx.Create(&results).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// GetTestRuns returns a list of test runs, optionally filtered by status, category_id, or run_folder_id.
// Pass folderID == "uncategorised" to filter runs with run_folder_id IS NULL.
// Supports pagination (offset/limit) and sorting. Returns (runs, total, error).
func (s *Store) GetTestRuns(categoryID string, status string, sortBy string, sortDir string, limit int, offset int, folderID string) ([]models.TestRun, int64, error) {
	var runs []models.TestRun
	var total int64
	query := s.db.Model(&models.TestRun{})

	if categoryID != "" {
		query = query.Where("category_id = ?", categoryID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if folderID == "uncategorised" {
		query = query.Where("run_folder_id IS NULL")
	} else if folderID != "" {
		// Include runs from this folder and all descendant subfolders
		folderIDs := s.getRunFolderDescendantIDs(s.db, folderID)
		query = query.Where("run_folder_id IN ?", folderIDs)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	orderClause := "created_at DESC" // Default
	if sortBy != "" {
		allowedSortColumns := map[string]bool{
			"name":       true,
			"status":     true,
			"created_at": true,
			"updated_at": true,
		}
		if !allowedSortColumns[sortBy] {
			sortBy = "created_at"
		}
		if sortDir != "ASC" && sortDir != "DESC" {
			sortDir = "DESC"
		}
		orderClause = fmt.Sprintf("%s %s", sortBy, sortDir)
	}

	query = query.Order(orderClause)

	if limit > 0 {
		query = query.Limit(limit).Offset(offset)
	}

	if err := query.Find(&runs).Error; err != nil {
		return nil, 0, err
	}

	// Populate aggregate counts instead of full RunResults
	if len(runs) > 0 {
		var runIDs []string
		runIndex := make(map[string]*models.TestRun)
		for i := range runs {
			runIDs = append(runIDs, runs[i].ID)
			runIndex[runs[i].ID] = &runs[i]
		}

		// Status counts — latest attempt per test case only
		type runCount struct {
			TestRunID string
			Status    string
			Count     int
		}
		var counts []runCount
		s.db.Raw(`
			SELECT rr.test_run_id, rr.status, COUNT(*) as count
			FROM run_results rr
			WHERE rr.test_run_id IN ?
			  AND (rr.test_case_id IS NULL OR rr.attempt_number = (
			    SELECT MAX(rr2.attempt_number)
			    FROM run_results rr2
			    WHERE rr2.test_run_id = rr.test_run_id
			      AND rr2.test_case_id = rr.test_case_id
			  ))
			GROUP BY rr.test_run_id, rr.status
		`, runIDs).Scan(&counts)

		for _, c := range counts {
			if run, ok := runIndex[c.TestRunID]; ok {
				run.TotalResults += c.Count
				switch models.ExecutionStatus(c.Status) {
				case models.StatusPass:
					run.PassedResults += c.Count
				case models.StatusFail, models.StatusError:
					run.FailedResults += c.Count
				case models.StatusSkip:
					run.SkippedResults += c.Count
				case models.StatusPending:
					run.PendingResults += c.Count
				}
			}
		}

		// Defect type counts for failed results
		type defectCount struct {
			TestRunID  string
			DefectType string
			Count      int
		}
		var defects []defectCount
		s.db.Raw(`
			SELECT rr.test_run_id, rr.defect_type, COUNT(*) as count
			FROM run_results rr
			WHERE rr.test_run_id IN ?
			  AND rr.status IN ('FAIL','ERROR')
			  AND (rr.test_case_id IS NULL OR rr.attempt_number = (
			    SELECT MAX(rr2.attempt_number)
			    FROM run_results rr2
			    WHERE rr2.test_run_id = rr.test_run_id
			      AND rr2.test_case_id = rr.test_case_id
			  ))
			GROUP BY rr.test_run_id, rr.defect_type
		`, runIDs).Scan(&defects)

		for _, d := range defects {
			if run, ok := runIndex[d.TestRunID]; ok {
				switch d.DefectType {
				case "product_bug":
					run.ProductBug += d.Count
				case "automation_bug":
					run.AutomationBug += d.Count
				case "system_issue":
					run.SystemIssue += d.Count
				default:
					// "to_investigate" or empty defect_type
					run.ToInvestigate += d.Count
				}
			}
		}

		// Retry stats per run
		type retryCount struct {
			TestRunID     string
			RetriedCount  int
			TotalAttempts int
		}
		var retryCounts []retryCount
		s.db.Raw(`
			SELECT test_run_id,
				SUM(CASE WHEN max_attempt > 1 THEN 1 ELSE 0 END) as retried_count,
				SUM(total) as total_attempts
			FROM (
				SELECT rr.test_run_id, rr.test_case_id,
					MAX(rr.attempt_number) as max_attempt,
					COUNT(*) as total
				FROM run_results rr
				WHERE rr.test_run_id IN ? AND rr.test_case_id IS NOT NULL
				GROUP BY rr.test_run_id, rr.test_case_id
			)
			GROUP BY test_run_id
		`, runIDs).Scan(&retryCounts)

		for _, rc := range retryCounts {
			if run, ok := runIndex[rc.TestRunID]; ok {
				run.RetriedCount = rc.RetriedCount
				run.TotalAttempts = rc.TotalAttempts
			}
		}
	}

	return runs, total, nil
}

// GetTestRun returns a single test run with its results, including each result's test case categories.
// Defect counts (open/closed) are populated on TestCase for FR-015 indicator display.
func (s *Store) GetTestRun(id string) (*models.TestRun, error) {
	var run models.TestRun
	if err := s.db.
		Preload("RunResults").
		Preload("RunResults.TestCase.Categories").
		First(&run, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // Return nil if not found, let handler handle 404
		}
		return nil, err
	}

	// Populate defect counts per run result (scoped defect links).
	rrIDs := make([]string, 0, len(run.RunResults))
	rrIndex := make(map[string]*models.RunResult, len(run.RunResults))
	for i, rr := range run.RunResults {
		rrIDs = append(rrIDs, rr.ID)
		rrIndex[rr.ID] = run.RunResults[i]
	}
	if len(rrIDs) > 0 {
		openCounts, closedCounts, err := s.CountDefectLinksByRunResults(rrIDs)
		if err == nil {
			for _, rr := range run.RunResults {
				rr.OpenDefectLinkCount = openCounts[rr.ID]
				rr.ClosedDefectLinkCount = closedCounts[rr.ID]
			}
		}
	}

	// Compute retry stats — a test case with more than one attempt counts as retried
	s.db.Raw(`
		SELECT COUNT(*) FROM (
			SELECT rr.test_case_id FROM run_results rr
			WHERE rr.test_run_id = ? AND rr.test_case_id IS NOT NULL
			GROUP BY rr.test_case_id
			HAVING MAX(rr.attempt_number) > 1
		)
	`, id).Scan(&run.RetriedCount)

	s.db.Raw(`SELECT COUNT(*) FROM run_results WHERE test_run_id = ?`, id).Scan(&run.TotalAttempts)

	return &run, nil
}

// ListRecentResultsForTestCase returns the most recent run results (up to limit)
// for a given test case, enriched with the run name and run status.
func (s *Store) ListRecentResultsForTestCase(testCaseID string, limit int) ([]models.TestCaseExecution, error) {
	if limit <= 0 {
		limit = 10
	}
	var rows []models.TestCaseExecution
	err := s.db.Raw(`
		SELECT rr.id, rr.status, rr.defect_type, rr.duration_ms, rr.error_message,
		       rr.environment, rr.browser, rr.attempt_number, rr.updated_at AS created_at,
		       tr.id AS run_id, tr.name AS run_name, tr.status AS run_status
		FROM run_results rr
		JOIN test_runs tr ON tr.id = rr.test_run_id
		WHERE rr.test_case_id = ?
		ORDER BY rr.updated_at DESC
		LIMIT ?
	`, testCaseID, limit).Scan(&rows).Error
	return rows, err
}

// TouchTestRun bumps the updated_at timestamp on a test run.
func (s *Store) TouchTestRun(runID string) {
	s.db.Model(&models.TestRun{}).Where("id = ?", runID).Update("updated_at", time.Now())
}

// UpdateRunResult updates the status and other fields of a specific run result by its primary key.
func (s *Store) UpdateRunResult(runID string, resultID string, updates interface{}) error {
	return s.db.Model(&models.RunResult{}).
		Where("id = ? AND test_run_id = ?", resultID, runID).
		Updates(updates).Error
}

// BulkUpdateRunResults updates multiple run results in a single transaction.
func (s *Store) BulkUpdateRunResults(runID string, resultIDs []string, updates map[string]interface{}) error {
	return s.db.Model(&models.RunResult{}).
		Where("id IN ? AND test_run_id = ?", resultIDs, runID).
		Updates(updates).Error
}

// removeScreenshotDir removes the screenshot directory for a result.
func removeScreenshotDir(resultID string) {
	dir := filepath.Join("uploads", "screenshots", resultID)
	_ = os.RemoveAll(dir)
}

// DeleteTestRun deletes a test run, its result-level comments, defect links, and run-level comments.
func (s *Store) DeleteTestRun(id string) error {
	var resultIDs []string
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Delete result-level comments and defect links for all results in this run
		tx.Model(&models.RunResult{}).Where("test_run_id = ?", id).Pluck("id", &resultIDs)
		if len(resultIDs) > 0 {
			if err := tx.Where("target_type = ? AND target_id IN ?", "result", resultIDs).
				Delete(&models.Comment{}).Error; err != nil {
				return err
			}
			if err := tx.Where("run_result_id IN ?", resultIDs).
				Delete(&models.DefectLink{}).Error; err != nil {
				return err
			}
			// AI failure-analysis rows have no DB FK to results; delete them here
			// so they are not orphaned when the results vanish (F-047).
			if err := tx.Where("run_result_id IN ?", resultIDs).
				Delete(&models.RunResultAnalysis{}).Error; err != nil {
				return err
			}
		}
		// Delete run-level comments
		if err := tx.Where("target_type = ? AND target_id = ?", "run", id).
			Delete(&models.Comment{}).Error; err != nil {
			return err
		}
		// Delete analysis jobs for this run (no DB FK to runs) (F-047).
		if err := tx.Where("test_run_id = ?", id).Delete(&models.RunAnalysisJob{}).Error; err != nil {
			return err
		}
		// Delete the run (cascades to run_results via FK constraint)
		return tx.Delete(&models.TestRun{}, "id = ?", id).Error
	})
	if err == nil {
		for _, rid := range resultIDs {
			removeScreenshotDir(rid)
		}
	}
	return err
}

// DeleteTestRuns deletes multiple test runs, their comments, and defect links.
func (s *Store) DeleteTestRuns(ids []string) error {
	var resultIDs []string
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Delete result-level comments and defect links for all results in these runs
		tx.Model(&models.RunResult{}).Where("test_run_id IN ?", ids).Pluck("id", &resultIDs)
		if len(resultIDs) > 0 {
			if err := tx.Where("target_type = ? AND target_id IN ?", "result", resultIDs).
				Delete(&models.Comment{}).Error; err != nil {
				return err
			}
			if err := tx.Where("run_result_id IN ?", resultIDs).
				Delete(&models.DefectLink{}).Error; err != nil {
				return err
			}
			// AI failure-analysis rows have no DB FK to results (F-047).
			if err := tx.Where("run_result_id IN ?", resultIDs).
				Delete(&models.RunResultAnalysis{}).Error; err != nil {
				return err
			}
		}
		// Delete run-level comments
		if err := tx.Where("target_type = ? AND target_id IN ?", "run", ids).
			Delete(&models.Comment{}).Error; err != nil {
			return err
		}
		// Delete analysis jobs for these runs (no DB FK to runs) (F-047).
		if err := tx.Where("test_run_id IN ?", ids).Delete(&models.RunAnalysisJob{}).Error; err != nil {
			return err
		}
		// Delete the runs (cascades to run_results via FK constraint)
		return tx.Delete(&models.TestRun{}, "id IN ?", ids).Error
	})
	if err == nil {
		for _, rid := range resultIDs {
			removeScreenshotDir(rid)
		}
	}
	return err
}

// AddRunResult adds a new test result (test case) to an existing run
// AddRunResult adds a new test result to an existing run.
// If test_case_id already exists in the run, auto-increments attempt_number.
// If attempt_number is explicitly set and > 0, validates no conflict.
func (s *Store) AddRunResult(result *models.RunResult) error {
	if result.ID == "" {
		result.ID = uuid.New().String()
	}
	if result.Status == "" {
		result.Status = models.StatusPending
	}
	// The parent run must exist, else we create an orphan result (F-046).
	var runCount int64
	if err := s.db.Model(&models.TestRun{}).Where("id = ?", result.TestRunID).Count(&runCount).Error; err != nil {
		return err
	}
	if runCount == 0 {
		return fmt.Errorf("test run not found")
	}

	now := time.Now()
	result.CreatedAt = now
	result.UpdatedAt = now

	// Snapshot name if missing and TestCaseID provided
	if result.TestNameSnapshot == "" && result.TestCaseID != nil {
		var tc models.TestCase
		if err := s.db.First(&tc, "id = ?", *result.TestCaseID).Error; err == nil {
			result.TestNameSnapshot = tc.Name
		}
	}

	// Handle attempt_number
	if result.TestCaseID != nil {
		var maxAttempt int
		s.db.Model(&models.RunResult{}).
			Select("COALESCE(MAX(attempt_number), 0)").
			Where("test_run_id = ? AND test_case_id = ?", result.TestRunID, *result.TestCaseID).
			Scan(&maxAttempt)

		if result.AttemptNumber > 0 {
			// Explicit attempt_number — validate no conflict
			if result.AttemptNumber <= maxAttempt {
				return fmt.Errorf("attempt_number %d already exists for this test case in this run", result.AttemptNumber)
			}
		} else {
			// Auto-increment
			result.AttemptNumber = maxAttempt + 1
		}
	} else if result.AttemptNumber == 0 {
		result.AttemptNumber = 1
	}

	// Bump parent run's updated_at
	s.db.Model(&models.TestRun{}).Where("id = ?", result.TestRunID).Update("updated_at", now)

	return s.db.Create(result).Error
}

// RunResultExists returns true if a run result with the given ID exists.
func (s *Store) RunResultExists(resultID string) bool {
	var count int64
	s.db.Model(&models.RunResult{}).Where("id = ?", resultID).Count(&count)
	return count > 0
}

// DeleteRunResult removes a test result by primary key, along with its comments and defect links.
func (s *Store) DeleteRunResult(runID string, resultID string) error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Verify result exists and belongs to this run
		var result models.RunResult
		if err := tx.Select("id").Where("id = ? AND test_run_id = ?", resultID, runID).
			First(&result).Error; err != nil {
			return err
		}
		// Delete result-level comments
		if err := tx.Where("target_type = ? AND target_id = ?", "result", resultID).
			Delete(&models.Comment{}).Error; err != nil {
			return err
		}
		// Delete result-level defect links
		if err := tx.Where("run_result_id = ?", resultID).
			Delete(&models.DefectLink{}).Error; err != nil {
			return err
		}
		// Delete the result
		return tx.Delete(&models.RunResult{}, "id = ?", resultID).Error
	})
	if err == nil {
		removeScreenshotDir(resultID)
	}
	return err
}

// CopyTestRun duplicates a test run (and optionally places it in a folder).
// All RunResults from the source are copied with PENDING status and no execution data.
func (s *Store) CopyTestRun(sourceID string, newName string, newFolderID *string) (*models.TestRun, error) {
	var newRun models.TestRun
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Load source run with results
		var source models.TestRun
		if err := tx.Preload("RunResults").First(&source, "id = ?", sourceID).Error; err != nil {
			return err
		}

		now := time.Now()
		newRun = models.TestRun{
			ID:          uuid.New().String(),
			Name:        newName,
			CategoryID:  source.CategoryID,
			RunFolderID: newFolderID,
			Status:      models.StatusPending,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if newRun.Name == "" {
			newRun.Name = fmt.Sprintf("Copy of %s", source.Name)
		}

		if err := tx.Create(&newRun).Error; err != nil {
			return err
		}

		// Copy results with PENDING status, preserving test case references (latest attempt only)
		latest := latestAttempts(source.RunResults)
		if len(latest) > 0 {
			var results []models.RunResult
			for _, rr := range latest {
				results = append(results, models.RunResult{
					ID:               uuid.New().String(),
					TestRunID:        newRun.ID,
					TestCaseID:       rr.TestCaseID,
					TestNameSnapshot: rr.TestNameSnapshot,
					AttemptNumber:    1,
					Status:           models.StatusPending,
					CreatedAt:        now,
					UpdatedAt:        now,
				})
			}
			if err := tx.Create(&results).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &newRun, nil
}

// CopyRunFolder deep-copies a run folder and all its descendants.
// Runs inside each folder are copied with PENDING results (no execution data).
func (s *Store) CopyRunFolder(sourceID string, newName string, newParentID *string) (*models.RunFolder, error) {
	var rootCopy *models.RunFolder
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Load all folders in one shot
		var allFolders []models.RunFolder
		if err := tx.Find(&allFolders).Error; err != nil {
			return err
		}

		// Build children map
		childrenMap := make(map[string][]models.RunFolder)
		folderMap := make(map[string]models.RunFolder)
		for _, f := range allFolders {
			folderMap[f.ID] = f
			pid := ""
			if f.ParentID != nil {
				pid = *f.ParentID
			}
			childrenMap[pid] = append(childrenMap[pid], f)
		}

		source, ok := folderMap[sourceID]
		if !ok {
			return fmt.Errorf("run folder %s not found", sourceID)
		}

		now := time.Now()

		// idMapping maps old folder ID → new folder ID
		idMapping := make(map[string]string)

		// BFS copy folders
		type qItem struct {
			oldID       string
			newParentID *string
		}
		queue := []qItem{{oldID: sourceID, newParentID: newParentID}}

		// Bound the copy so a deep/wide subtree (or repeated copy-of-a-copy) cannot
		// amplify into an unbounded number of rows (F-035).
		const maxCopyFolders = 1000
		copied := 0

		for len(queue) > 0 {
			item := queue[0]
			queue = queue[1:]
			if copied++; copied > maxCopyFolders {
				return fmt.Errorf("refusing to copy: folder subtree exceeds %d folders", maxCopyFolders)
			}

			old := folderMap[item.oldID]
			newID := uuid.New().String()
			idMapping[item.oldID] = newID

			folderName := old.Name
			if item.oldID == sourceID && newName != "" {
				folderName = newName
			} else if item.oldID == sourceID {
				folderName = fmt.Sprintf("Copy of %s", source.Name)
			}

			newFolder := models.RunFolder{
				ID:           newID,
				ParentID:     item.newParentID,
				Name:         folderName,
				DisplayOrder: old.DisplayOrder,
				CreatedAt:    now,
				UpdatedAt:    now,
			}
			if err := tx.Create(&newFolder).Error; err != nil {
				return err
			}
			if item.oldID == sourceID {
				rootCopy = &newFolder
			}

			// Enqueue children
			for _, child := range childrenMap[item.oldID] {
				cNewParent := newID
				queue = append(queue, qItem{oldID: child.ID, newParentID: &cNewParent})
			}
		}

		// Copy runs in each mapped folder
		for oldFolderID, newFolderID := range idMapping {
			var runs []models.TestRun
			if err := tx.Preload("RunResults").Where("run_folder_id = ?", oldFolderID).Find(&runs).Error; err != nil {
				return err
			}
			for _, run := range runs {
				nfid := newFolderID
				newRunID := uuid.New().String()
				newRun := models.TestRun{
					ID:          newRunID,
					Name:        fmt.Sprintf("Copy of %s", run.Name),
					CategoryID:  run.CategoryID,
					RunFolderID: &nfid,
					Status:      models.StatusPending,
					CreatedAt:   now,
					UpdatedAt:   now,
				}
				if err := tx.Create(&newRun).Error; err != nil {
					return err
				}
				latest := latestAttempts(run.RunResults)
				if len(latest) > 0 {
					var results []models.RunResult
					for _, rr := range latest {
						results = append(results, models.RunResult{
							ID:               uuid.New().String(),
							TestRunID:        newRunID,
							TestCaseID:       rr.TestCaseID,
							TestNameSnapshot: rr.TestNameSnapshot,
							AttemptNumber:    1,
							Status:           models.StatusPending,
							CreatedAt:        now,
							UpdatedAt:        now,
						})
					}
					if err := tx.Create(&results).Error; err != nil {
						return err
					}
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return rootCopy, nil
}

// CompleteRun marks a test run as completed/failed (idempotent).
// Returns (run, changed, error). changed is true only when the status was actually updated.
// Returns nil run if not found.
func (s *Store) CompleteRun(id string) (*models.TestRun, bool, error) {
	var run models.TestRun
	if err := s.db.First(&run, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}

	// Idempotent: already in a terminal state — return as-is, changed=false
	if run.Status == models.StatusPass || run.Status == models.StatusFail || run.Status == models.StatusError {
		return &run, false, nil
	}

	// Determine status from results: any FAIL/ERROR in latest attempts → FAIL, otherwise PASS
	var failCount int64
	s.db.Raw(`
		SELECT COUNT(*) FROM run_results rr
		WHERE rr.test_run_id = ?
		  AND rr.status IN ('FAIL','ERROR')
		  AND (rr.test_case_id IS NULL OR rr.attempt_number = (
		    SELECT MAX(rr2.attempt_number)
		    FROM run_results rr2
		    WHERE rr2.test_run_id = rr.test_run_id
		      AND rr2.test_case_id = rr.test_case_id
		  ))
	`, id).Scan(&failCount)

	var newStatus models.ExecutionStatus
	if failCount > 0 {
		newStatus = models.StatusFail
	} else {
		newStatus = models.StatusPass
	}

	now := time.Now()
	if err := s.db.Model(&run).Updates(map[string]interface{}{
		"status":     string(newStatus),
		"updated_at": now,
	}).Error; err != nil {
		return nil, false, err
	}
	run.Status = newStatus
	run.UpdatedAt = now
	return &run, true, nil
}

// ReopenRun sets a completed (PASS/FAIL) run back to RUNNING so the user can
// continue updating results. If the run is already non-terminal, it is returned as-is.
func (s *Store) ReopenRun(id string) (*models.TestRun, error) {
	var run models.TestRun
	if err := s.db.First(&run, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	// Only reopen if in a terminal state
	if run.Status != models.StatusPass && run.Status != models.StatusFail {
		return &run, nil
	}

	now := time.Now()
	if err := s.db.Model(&run).Updates(map[string]interface{}{
		"status":     string(models.StatusRunning),
		"updated_at": now,
	}).Error; err != nil {
		return nil, err
	}
	run.Status = models.StatusRunning
	run.UpdatedAt = now
	return &run, nil
}

// RetryRunResult creates a new PENDING attempt for the same test case as the given result.
// Returns error if the result has no test_case_id (orphaned).
// If the parent run is in a terminal state, reverts it to RUNNING.
func (s *Store) RetryRunResult(runID string, resultID string) (*models.RunResult, error) {
	var source models.RunResult
	if err := s.db.Where("id = ? AND test_run_id = ?", resultID, runID).First(&source).Error; err != nil {
		return nil, err
	}
	if source.TestCaseID == nil {
		return nil, fmt.Errorf("cannot retry orphaned result (no test_case_id)")
	}

	var newResult models.RunResult
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var maxAttempt int
		tx.Model(&models.RunResult{}).
			Select("COALESCE(MAX(attempt_number), 0)").
			Where("test_run_id = ? AND test_case_id = ?", runID, *source.TestCaseID).
			Scan(&maxAttempt)

		now := time.Now()
		newResult = models.RunResult{
			ID:               uuid.New().String(),
			TestRunID:        runID,
			TestCaseID:       source.TestCaseID,
			AttemptNumber:    maxAttempt + 1,
			TestNameSnapshot: source.TestNameSnapshot,
			Status:           models.StatusPending,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		if err := tx.Create(&newResult).Error; err != nil {
			return err
		}

		// Bump parent run's updated_at and revert to RUNNING if in terminal state
		return tx.Model(&models.TestRun{}).Where("id = ?", runID).
			Updates(map[string]interface{}{
				"updated_at": now,
				"status":     string(models.StatusRunning),
			}).Error
	})
	if err != nil {
		return nil, err
	}
	return &newResult, nil
}

// latestAttempts filters a slice of RunResults to only the highest attempt_number per test_case_id.
func latestAttempts(results []*models.RunResult) []*models.RunResult {
	best := make(map[string]*models.RunResult) // key: test_case_id
	var orphans []*models.RunResult
	for _, rr := range results {
		if rr.TestCaseID == nil {
			orphans = append(orphans, rr)
			continue
		}
		tcID := *rr.TestCaseID
		if existing, ok := best[tcID]; !ok || rr.AttemptNumber > existing.AttemptNumber {
			best[tcID] = rr
		}
	}
	out := orphans
	for _, rr := range best {
		out = append(out, rr)
	}
	return out
}

// ListLatestFailingResults returns FAIL/ERROR RunResults for a run, restricted
// to the latest attempt per test_case_id (orphan rows with NULL test_case_id
// are always included).
//
// Shared by the failure-analysis worker and the MaybeEnqueueForRun early-exit
// check so both see the same failure set — a FAIL→PASS retry is correctly
// excluded.
func (s *Store) ListLatestFailingResults(runID string) ([]*models.RunResult, error) {
	var results []*models.RunResult
	err := s.db.Raw(`
		SELECT rr.* FROM run_results rr
		WHERE rr.test_run_id = ?
		  AND rr.status IN ('FAIL','ERROR')
		  AND (rr.test_case_id IS NULL OR rr.attempt_number = (
		    SELECT MAX(rr2.attempt_number)
		    FROM run_results rr2
		    WHERE rr2.test_run_id = rr.test_run_id
		      AND rr2.test_case_id = rr.test_case_id
		  ))
		ORDER BY rr.created_at ASC
	`, runID).Scan(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

// GetRunResultByID returns a single result by id, or (nil, nil) if not found.
func (s *Store) GetRunResultByID(id string) (*models.RunResult, error) {
	var r models.RunResult
	if err := s.db.First(&r, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &r, nil
}
