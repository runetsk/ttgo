package store

import (
	"testing"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/assert"
)

func TestGetFolderTree_IncludeTestCases(t *testing.T) {
	// Use the package's shared newTestStore helper (isolated private :memory: DB
	// per test with t.Cleanup(Close)). The previous local setupTestStore opened a
	// process-lifetime "file::memory:?cache=shared" DB that leaked rows across
	// repeated runs in one process, colliding on the hardcoded IDs below.
	s := newTestStore(t)

	// 1. Create Folder Structure
	root, err := s.CreateFolder("Root Folder", nil)
	assert.NoError(t, err)

	sub, err := s.CreateFolder("Sub Folder", &root.ID)
	assert.NoError(t, err)

	// 2. Create Test Case in Sub Folder
	tc := &models.TestCase{
		ID:       "tc-1",
		Name:     "Test Case A",
		FolderID: sub.ID,
	}
	err = s.db.Create(tc).Error
	assert.NoError(t, err)

	// 3. Create Orphan Test Case (in Root)
	tc2 := &models.TestCase{
		ID:       "tc-2",
		Name:     "Test Case B",
		FolderID: root.ID,
	}
	err = s.db.Create(tc2).Error
	assert.NoError(t, err)

	// 4. Fetch Tree
	tree, err := s.GetFolderTree()
	assert.NoError(t, err)

	// 5. Verify Structure
	assert.NotEmpty(t, tree)
	// Find Root
	var rootFolder *models.Folder
	for _, f := range tree {
		if f.ID == root.ID {
			rootFolder = f
			break
		}
	}
	assert.NotNil(t, rootFolder)

	// Check Root Test Cases
	assert.Len(t, rootFolder.TestCases, 1)
	assert.Equal(t, "Test Case B", rootFolder.TestCases[0].Name)

	// Check Sub Folder
	assert.Len(t, rootFolder.SubFolders, 1)
	subFolder := rootFolder.SubFolders[0]
	assert.Equal(t, sub.ID, subFolder.ID)

	// Check Sub Folder Test Cases
	assert.Len(t, subFolder.TestCases, 1)
	assert.Equal(t, "Test Case A", subFolder.TestCases[0].Name)
}
