package store

import (
	"fmt"
	"ttgo/pkg/tracker/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CreateFolder creates a new folder.
func (s *Store) CreateFolder(name string, parentID *string) (*models.Folder, error) {
	return s.createFolderTx(s.db, name, parentID)
}

func (s *Store) createFolderTx(tx *gorm.DB, name string, parentID *string) (*models.Folder, error) {
	folder := &models.Folder{
		ID:       uuid.New().String(),
		Name:     name,
		ParentID: parentID,
	}
	if err := tx.Create(folder).Error; err != nil {
		return nil, err
	}
	return folder, nil
}

// FindOrCreateSubfolder returns an existing child folder with the given name
// under parentID, or creates a new one if it doesn't exist.
func (s *Store) FindOrCreateSubfolder(parentID, name string) (*models.Folder, error) {
	return s.findOrCreateSubfolderTx(s.db, parentID, name)
}

func (s *Store) findOrCreateSubfolderTx(tx *gorm.DB, parentID, name string) (*models.Folder, error) {
	var existing models.Folder
	err := tx.Where("parent_id = ? AND name = ?", parentID, name).First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return s.createFolderTx(tx, name, &parentID)
}

// GetFolder returns a folder by ID.
func (s *Store) GetFolder(id string) (*models.Folder, error) {
	var folder models.Folder
	if err := s.db.First(&folder, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &folder, nil
}

// GetFolderTree returns the full hierarchy of folders.
func (s *Store) GetFolderTree() ([]*models.Folder, error) {
	var allFolders []*models.Folder
	// Fetch all folders
	if err := s.db.Order("name asc").Find(&allFolders).Error; err != nil {
		return nil, err
	}

	// Fetch all test cases
	var allTestCases []*models.TestCase
	if err := s.db.Select("id, name, folder_id, reverification_flagged").Order("created_at asc, name asc").Find(&allTestCases).Error; err != nil {
		return nil, err
	}

	// Populate defect counts per test case (008-jira-integration, FR-015).
	type defectCount struct {
		TestCaseID     string
		StatusCategory string
		Count          int
	}
	var counts []defectCount
	_ = s.db.Model(&models.DefectLink{}).
		Where("run_result_id IS NULL").
		Select("test_case_id, status_category, count(*) as count").
		Group("test_case_id, status_category").
		Scan(&counts).Error
	openMap := make(map[string]int)
	closedMap := make(map[string]int)
	for _, c := range counts {
		if c.StatusCategory == "done" {
			closedMap[c.TestCaseID] += c.Count
		} else {
			openMap[c.TestCaseID] += c.Count
		}
	}
	for _, tc := range allTestCases {
		tc.OpenDefectCount = openMap[tc.ID]
		tc.ClosedDefectCount = closedMap[tc.ID]
	}

	// Build tree
	folderMap := make(map[string]*models.Folder)
	var roots []*models.Folder

	for _, f := range allFolders {
		f.SubFolders = []*models.Folder{}  // Init slice
		f.TestCases = []*models.TestCase{} // Init slice
		folderMap[f.ID] = f
	}

	// Attach test cases to folders
	for _, tc := range allTestCases {
		if folder, exists := folderMap[tc.FolderID]; exists {
			folder.TestCases = append(folder.TestCases, tc)
		}
	}

	for _, f := range allFolders {
		if f.ParentID == nil || *f.ParentID == "" {
			roots = append(roots, f)
		} else {
			if parent, exists := folderMap[*f.ParentID]; exists {
				parent.SubFolders = append(parent.SubFolders, f)
			} else {
				// Parent not found, treat as root or orphan? Treat as root for safety
				roots = append(roots, f)
			}
		}
	}

	return roots, nil
}

// DeleteFolder deletes a folder and its contents (subfolders, test cases) recursively.
func (s *Store) DeleteFolder(id string) error {
	descendants, err := s.GetFolderDescendants(id)
	if err != nil {
		return err
	}
	return s.deleteFoldersAndContents(descendants)
}

// BulkDeleteFolders deletes multiple folders and their descendants.
func (s *Store) BulkDeleteFolders(ids []string) error {
	allIDs := make(map[string]bool)
	for _, id := range ids {
		descendants, err := s.GetFolderDescendants(id)
		if err != nil {
			return err
		}
		for _, d := range descendants {
			allIDs[d] = true
		}
	}
	toDelete := make([]string, 0, len(allIDs))
	for id := range allIDs {
		toDelete = append(toDelete, id)
	}
	return s.deleteFoldersAndContents(toDelete)
}

// deleteFoldersAndContents deletes the given folders and all test cases inside
// them, atomically: the test-case cascade and the folder delete share one
// transaction so a mid-cascade failure cannot leave tests destroyed but folders
// behind (or vice versa) (F-015).
func (s *Store) deleteFoldersAndContents(folderIDs []string) error {
	if len(folderIDs) == 0 {
		return nil
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		var testCaseIDs []string
		if err := tx.Model(&models.TestCase{}).Where("folder_id IN ?", folderIDs).Pluck("id", &testCaseIDs).Error; err != nil {
			return err
		}
		if len(testCaseIDs) > 0 {
			if err := deleteTestCasesTx(tx, testCaseIDs); err != nil {
				return err
			}
		}
		return tx.Delete(&models.Folder{}, "id IN ?", folderIDs).Error
	})
}

// GetFolderDescendants returns ID of the folder and all its subfolders recursively.
func (s *Store) GetFolderDescendants(rootID string) ([]string, error) {
	var allFolders []models.Folder
	if err := s.db.Select("id", "parent_id").Find(&allFolders).Error; err != nil {
		return nil, err
	}

	descendants := []string{rootID}
	visited := map[string]bool{rootID: true}
	queue := []string{rootID}

	// Simple BFS with visited set to prevent infinite loops from data cycles
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, f := range allFolders {
			if f.ParentID != nil && *f.ParentID == current && !visited[f.ID] {
				visited[f.ID] = true
				descendants = append(descendants, f.ID)
				queue = append(queue, f.ID)
			}
		}
	}
	return descendants, nil
}

// RenameFolder updates the name of a folder.
func (s *Store) RenameFolder(id string, name string) error {
	// Reject a rename that collides with an existing sibling name, which would make
	// import's find-or-create-by-(parent,name) ambiguous (F-061).
	var folder models.Folder
	if err := s.db.Select("parent_id").First(&folder, "id = ?", id).Error; err != nil {
		return err
	}
	q := s.db.Model(&models.Folder{}).Where("name = ? AND id <> ?", name, id)
	if folder.ParentID == nil {
		q = q.Where("parent_id IS NULL")
	} else {
		q = q.Where("parent_id = ?", *folder.ParentID)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("a sibling folder named %q already exists", name)
	}
	return s.db.Model(&models.Folder{}).Where("id = ?", id).Update("name", name).Error
}

func (s *Store) MoveFolder(id string, newParentID *string) error {
	// Moving to root is always safe
	if newParentID == nil || *newParentID == "" {
		return s.db.Model(&models.Folder{}).Where("id = ?", id).Update("parent_id", newParentID).Error
	}
	// Cannot move a folder into itself
	if *newParentID == id {
		return models.ErrCircularReference
	}
	// Check that newParentID is not a descendant of id
	descendants, err := s.GetFolderDescendants(id)
	if err != nil {
		return err
	}
	for _, d := range descendants {
		if d == *newParentID {
			return models.ErrCircularReference
		}
	}
	return s.db.Model(&models.Folder{}).Where("id = ?", id).Update("parent_id", newParentID).Error
}

func (s *Store) BulkMoveFolders(ids []string, newParentID *string) error {
	// Moving to root is always safe
	if newParentID == nil || *newParentID == "" {
		return s.db.Model(&models.Folder{}).Where("id IN ?", ids).Update("parent_id", newParentID).Error
	}
	// Check each folder for circular reference
	for _, id := range ids {
		if *newParentID == id {
			return models.ErrCircularReference
		}
		descendants, err := s.GetFolderDescendants(id)
		if err != nil {
			return err
		}
		for _, d := range descendants {
			if d == *newParentID {
				return models.ErrCircularReference
			}
		}
	}
	return s.db.Model(&models.Folder{}).Where("id IN ?", ids).Update("parent_id", newParentID).Error
}
