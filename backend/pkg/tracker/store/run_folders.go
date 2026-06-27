package store

import (
	"errors"
	"time"
	"ttgo/pkg/tracker/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CreateRunFolder creates a new run folder. display_order is assigned as MAX + 10
// scoped to siblings sharing the same parent_id.
func (s *Store) CreateRunFolder(folder *models.RunFolder) error {
	if folder.ID == "" {
		folder.ID = uuid.New().String()
	}
	folder.CreatedAt = time.Now()
	folder.UpdatedAt = time.Now()

	// Assign display_order as MAX(display_order) + 10 among siblings
	var maxOrder int
	q := s.db.Model(&models.RunFolder{})
	if folder.ParentID == nil || *folder.ParentID == "" {
		q = q.Where("parent_id IS NULL OR parent_id = ''")
	} else {
		q = q.Where("parent_id = ?", *folder.ParentID)
	}
	q.Select("COALESCE(MAX(display_order), 0)").Scan(&maxOrder)
	folder.DisplayOrder = maxOrder + 10

	// Normalise empty string to nil
	if folder.ParentID != nil && *folder.ParentID == "" {
		folder.ParentID = nil
	}

	return s.db.Create(folder).Error
}

// GetRunFolders returns all run folders ordered by display_order ASC (flat list).
func (s *Store) GetRunFolders() ([]models.RunFolder, error) {
	var folders []models.RunFolder
	if err := s.db.Order("display_order ASC").Find(&folders).Error; err != nil {
		return nil, err
	}
	return folders, nil
}

// GetRunFolderTree returns the full hierarchy of run folders with nested SubFolders
// and TestRuns populated (lightweight summary: only id, name, status, run_folder_id).
func (s *Store) GetRunFolderTree() ([]*models.RunFolder, error) {
	var allFolders []*models.RunFolder
	if err := s.db.Order("display_order ASC").Find(&allFolders).Error; err != nil {
		return nil, err
	}

	// Fetch lightweight run summaries (only fields needed for sidebar)
	var allRuns []*models.TestRun
	if err := s.db.Select("id, name, status, run_folder_id, created_at").
		Order("created_at DESC").
		Find(&allRuns).Error; err != nil {
		return nil, err
	}

	// Build lookup map
	folderMap := make(map[string]*models.RunFolder)
	for _, f := range allFolders {
		f.SubFolders = []*models.RunFolder{}
		f.TestRuns = []*models.TestRun{}
		folderMap[f.ID] = f
	}

	// Attach runs to their folders
	for _, r := range allRuns {
		if r.RunFolderID != nil && *r.RunFolderID != "" {
			if folder, exists := folderMap[*r.RunFolderID]; exists {
				folder.TestRuns = append(folder.TestRuns, r)
			}
		}
	}

	// Build tree
	var roots []*models.RunFolder
	for _, f := range allFolders {
		if f.ParentID == nil || *f.ParentID == "" {
			roots = append(roots, f)
		} else {
			if parent, exists := folderMap[*f.ParentID]; exists {
				parent.SubFolders = append(parent.SubFolders, f)
			} else {
				// Orphan — treat as root for safety
				roots = append(roots, f)
			}
		}
	}

	return roots, nil
}

// UpdateRunFolder renames a run folder.
func (s *Store) UpdateRunFolder(id string, name string) error {
	result := s.db.Model(&models.RunFolder{}).Where("id = ?", id).Updates(map[string]interface{}{
		"name":       name,
		"updated_at": time.Now(),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ReorderRunFolder sets the display_order on a folder. If a collision occurs after
// save (two folders with the same order), it renumbers all folders in multiples of 10.
func (s *Store) ReorderRunFolder(id string, displayOrder int) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&models.RunFolder{}).Where("id = ?", id).Updates(map[string]interface{}{
			"display_order": displayOrder,
			"updated_at":    time.Now(),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		// Check for collisions — if any two folders share the same display_order, renumber
		var collisionCount int64
		tx.Raw(`SELECT COUNT(*) FROM (SELECT display_order FROM run_folders GROUP BY display_order HAVING COUNT(*) > 1)`).Scan(&collisionCount)
		if collisionCount > 0 {
			var folders []models.RunFolder
			if err := tx.Order("display_order ASC, created_at ASC").Find(&folders).Error; err != nil {
				return err
			}
			for i, f := range folders {
				newOrder := (i + 1) * 10
				if err := tx.Model(&models.RunFolder{}).Where("id = ?", f.ID).Update("display_order", newOrder).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// DeleteRunFolder cascades: collects all descendant folder IDs (BFS), nulls out
// run_folder_id on all runs in those folders, then deletes all folders.
func (s *Store) DeleteRunFolder(id string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Verify the folder exists first
		var folder models.RunFolder
		if err := tx.First(&folder, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return gorm.ErrRecordNotFound
			}
			return err
		}

		// Collect all descendant folder IDs via BFS
		descendantIDs := s.getRunFolderDescendantIDs(tx, id)

		// Null out run_folder_id on all runs belonging to any of these folders
		if err := tx.Model(&models.TestRun{}).Where("run_folder_id IN ?", descendantIDs).Update("run_folder_id", nil).Error; err != nil {
			return err
		}

		// Delete all descendant folders + the target
		return tx.Delete(&models.RunFolder{}, "id IN ?", descendantIDs).Error
	})
}

// getRunFolderDescendantIDs returns the id and all descendant folder IDs via BFS.
func (s *Store) getRunFolderDescendantIDs(tx *gorm.DB, rootID string) []string {
	var allFolders []models.RunFolder
	tx.Select("id, parent_id").Find(&allFolders)

	descendants := []string{rootID}
	queue := []string{rootID}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, f := range allFolders {
			if f.ParentID != nil && *f.ParentID == current {
				descendants = append(descendants, f.ID)
				queue = append(queue, f.ID)
			}
		}
	}
	return descendants
}

// MoveRunFolder updates the parent_id of a run folder with cycle detection.
func (s *Store) MoveRunFolder(id string, newParentID *string) error {
	// Moving to root is always safe
	if newParentID == nil || *newParentID == "" {
		return s.db.Model(&models.RunFolder{}).Where("id = ?", id).Updates(map[string]interface{}{
			"parent_id":  nil,
			"updated_at": time.Now(),
		}).Error
	}

	// Cycle detection: newParentID must not be id itself or a descendant of id
	if *newParentID == id {
		return models.ErrCircularReference
	}

	// Use a fresh read to collect descendants
	descendants := s.getRunFolderDescendantIDs(s.db, id)
	for _, did := range descendants {
		if did == *newParentID {
			return models.ErrCircularReference
		}
	}

	return s.db.Model(&models.RunFolder{}).Where("id = ?", id).Updates(map[string]interface{}{
		"parent_id":  newParentID,
		"updated_at": time.Now(),
	}).Error
}

// AssignRunToFolder sets the run_folder_id on a TestRun.
// Pass nil folderID to unassign (move to Uncategorised).
func (s *Store) AssignRunToFolder(runID string, folderID *string) error {
	result := s.db.Model(&models.TestRun{}).Where("id = ?", runID).Updates(map[string]interface{}{
		"run_folder_id": folderID,
		"updated_at":    time.Now(),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// GetRunFolder returns a single RunFolder by ID (used for validation in handlers).
func (s *Store) GetRunFolder(id string) (*models.RunFolder, error) {
	var folder models.RunFolder
	if err := s.db.First(&folder, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &folder, nil
}
