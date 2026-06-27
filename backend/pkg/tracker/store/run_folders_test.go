package store

import (
	"testing"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper: create a category + test run, return the run
func createTestRunForFolder(t *testing.T, s *Store, runFolderID *string) *models.TestRun {
	t.Helper()
	category, err := s.CreateCategory("Suite", "")
	require.NoError(t, err)
	run := &models.TestRun{
		CategoryID:  &category.ID,
		Name:        "Run",
		RunFolderID: runFolderID,
	}
	err = s.CreateTestRun(run)
	require.NoError(t, err)
	return run
}

func strPtr(s string) *string { return &s }

func TestCreateRunFolder(t *testing.T) {
	s := newTestStore(t)

	folder := &models.RunFolder{Name: "Smoke"}
	err := s.CreateRunFolder(folder)
	require.NoError(t, err)
	assert.NotEmpty(t, folder.ID)
	assert.Equal(t, "Smoke", folder.Name)
	assert.Equal(t, 10, folder.DisplayOrder) // first folder gets 10
	assert.Nil(t, folder.ParentID)
}

func TestCreateRunFolderWithParent(t *testing.T) {
	s := newTestStore(t)

	parent := &models.RunFolder{Name: "Parent"}
	require.NoError(t, s.CreateRunFolder(parent))

	child := &models.RunFolder{Name: "Child", ParentID: &parent.ID}
	require.NoError(t, s.CreateRunFolder(child))

	assert.NotEmpty(t, child.ID)
	assert.Equal(t, "Child", child.Name)
	require.NotNil(t, child.ParentID)
	assert.Equal(t, parent.ID, *child.ParentID)
	assert.Equal(t, 10, child.DisplayOrder) // first child of parent gets 10
}

func TestCreateRunFolderDisplayOrderScopedToSiblings(t *testing.T) {
	s := newTestStore(t)

	// Root siblings
	f1 := &models.RunFolder{Name: "Root-A"}
	f2 := &models.RunFolder{Name: "Root-B"}
	require.NoError(t, s.CreateRunFolder(f1))
	require.NoError(t, s.CreateRunFolder(f2))
	assert.Equal(t, 10, f1.DisplayOrder)
	assert.Equal(t, 20, f2.DisplayOrder)

	// Children of f1 — display_order starts fresh at 10
	c1 := &models.RunFolder{Name: "Child-1", ParentID: &f1.ID}
	c2 := &models.RunFolder{Name: "Child-2", ParentID: &f1.ID}
	require.NoError(t, s.CreateRunFolder(c1))
	require.NoError(t, s.CreateRunFolder(c2))
	assert.Equal(t, 10, c1.DisplayOrder)
	assert.Equal(t, 20, c2.DisplayOrder)
}

func TestCreateRunFolderDisplayOrderIncrement(t *testing.T) {
	s := newTestStore(t)

	f1 := &models.RunFolder{Name: "Alpha"}
	f2 := &models.RunFolder{Name: "Beta"}
	f3 := &models.RunFolder{Name: "Gamma"}

	require.NoError(t, s.CreateRunFolder(f1))
	require.NoError(t, s.CreateRunFolder(f2))
	require.NoError(t, s.CreateRunFolder(f3))

	assert.Equal(t, 10, f1.DisplayOrder)
	assert.Equal(t, 20, f2.DisplayOrder)
	assert.Equal(t, 30, f3.DisplayOrder)
}

func TestGetRunFolders(t *testing.T) {
	s := newTestStore(t)

	// Empty DB returns empty slice (not error)
	folders, err := s.GetRunFolders()
	require.NoError(t, err)
	assert.Empty(t, folders)

	require.NoError(t, s.CreateRunFolder(&models.RunFolder{Name: "Draft"}))
	require.NoError(t, s.CreateRunFolder(&models.RunFolder{Name: "Regression"}))

	folders, err = s.GetRunFolders()
	require.NoError(t, err)
	require.Len(t, folders, 2)
	assert.Equal(t, "Draft", folders[0].Name)
	assert.Equal(t, "Regression", folders[1].Name)
	// ordered by display_order
	assert.Less(t, folders[0].DisplayOrder, folders[1].DisplayOrder)
}

func TestGetRunFolderTree(t *testing.T) {
	s := newTestStore(t)

	// Create hierarchy: Root > Child > Grandchild
	root := &models.RunFolder{Name: "Root"}
	require.NoError(t, s.CreateRunFolder(root))

	child := &models.RunFolder{Name: "Child", ParentID: &root.ID}
	require.NoError(t, s.CreateRunFolder(child))

	grandchild := &models.RunFolder{Name: "Grandchild", ParentID: &child.ID}
	require.NoError(t, s.CreateRunFolder(grandchild))

	// Create a run in the child folder
	run := createTestRunForFolder(t, s, &child.ID)

	tree, err := s.GetRunFolderTree()
	require.NoError(t, err)
	require.Len(t, tree, 1, "should have 1 root")
	assert.Equal(t, "Root", tree[0].Name)

	require.Len(t, tree[0].SubFolders, 1, "root should have 1 child")
	assert.Equal(t, "Child", tree[0].SubFolders[0].Name)

	require.Len(t, tree[0].SubFolders[0].TestRuns, 1, "child should have 1 run")
	assert.Equal(t, run.ID, tree[0].SubFolders[0].TestRuns[0].ID)

	require.Len(t, tree[0].SubFolders[0].SubFolders, 1, "child should have 1 grandchild")
	assert.Equal(t, "Grandchild", tree[0].SubFolders[0].SubFolders[0].Name)
}

func TestUpdateRunFolder(t *testing.T) {
	s := newTestStore(t)

	folder := &models.RunFolder{Name: "OldName"}
	require.NoError(t, s.CreateRunFolder(folder))

	err := s.UpdateRunFolder(folder.ID, "NewName")
	require.NoError(t, err)

	folders, _ := s.GetRunFolders()
	require.Len(t, folders, 1)
	assert.Equal(t, "NewName", folders[0].Name)
}

func TestUpdateRunFolderNotFound(t *testing.T) {
	s := newTestStore(t)
	err := s.UpdateRunFolder("nonexistent-id", "Name")
	require.Error(t, err) // should return gorm.ErrRecordNotFound
}

func TestDeleteRunFolder(t *testing.T) {
	s := newTestStore(t)

	folder := &models.RunFolder{Name: "Temp"}
	require.NoError(t, s.CreateRunFolder(folder))

	// Create a run in this folder
	run := createTestRunForFolder(t, s, &folder.ID)

	// Delete the folder
	err := s.DeleteRunFolder(folder.ID)
	require.NoError(t, err)

	// Folder is gone
	folders, _ := s.GetRunFolders()
	assert.Empty(t, folders)

	// Run still exists but run_folder_id is now NULL
	retrievedRun, err := s.GetTestRun(run.ID)
	require.NoError(t, err)
	require.NotNil(t, retrievedRun)
	assert.Nil(t, retrievedRun.RunFolderID, "deleting a folder must null out run_folder_id on its runs")
}

func TestDeleteRunFolderCascadesSubfolders(t *testing.T) {
	s := newTestStore(t)

	root := &models.RunFolder{Name: "Root"}
	require.NoError(t, s.CreateRunFolder(root))

	child := &models.RunFolder{Name: "Child", ParentID: &root.ID}
	require.NoError(t, s.CreateRunFolder(child))

	grandchild := &models.RunFolder{Name: "Grandchild", ParentID: &child.ID}
	require.NoError(t, s.CreateRunFolder(grandchild))

	// Run in grandchild
	run := createTestRunForFolder(t, s, &grandchild.ID)

	// Delete root — should cascade to child and grandchild
	err := s.DeleteRunFolder(root.ID)
	require.NoError(t, err)

	folders, _ := s.GetRunFolders()
	assert.Empty(t, folders, "all descendant folders should be deleted")

	retrievedRun, err := s.GetTestRun(run.ID)
	require.NoError(t, err)
	assert.Nil(t, retrievedRun.RunFolderID, "grandchild's run should be uncategorised")
}

func TestDeleteRunFolderNotFound(t *testing.T) {
	s := newTestStore(t)
	err := s.DeleteRunFolder("nonexistent-id")
	require.Error(t, err)
}

func TestMoveRunFolder(t *testing.T) {
	s := newTestStore(t)

	a := &models.RunFolder{Name: "A"}
	b := &models.RunFolder{Name: "B"}
	require.NoError(t, s.CreateRunFolder(a))
	require.NoError(t, s.CreateRunFolder(b))

	// Move B under A
	err := s.MoveRunFolder(b.ID, &a.ID)
	require.NoError(t, err)

	tree, err := s.GetRunFolderTree()
	require.NoError(t, err)
	require.Len(t, tree, 1, "only A at root")
	assert.Equal(t, "A", tree[0].Name)
	require.Len(t, tree[0].SubFolders, 1)
	assert.Equal(t, "B", tree[0].SubFolders[0].Name)
}

func TestMoveRunFolderToRoot(t *testing.T) {
	s := newTestStore(t)

	parent := &models.RunFolder{Name: "Parent"}
	require.NoError(t, s.CreateRunFolder(parent))

	child := &models.RunFolder{Name: "Child", ParentID: &parent.ID}
	require.NoError(t, s.CreateRunFolder(child))

	// Move child to root
	err := s.MoveRunFolder(child.ID, nil)
	require.NoError(t, err)

	tree, err := s.GetRunFolderTree()
	require.NoError(t, err)
	require.Len(t, tree, 2, "both should be at root now")
}

func TestMoveRunFolderCircularDetection(t *testing.T) {
	s := newTestStore(t)

	a := &models.RunFolder{Name: "A"}
	require.NoError(t, s.CreateRunFolder(a))

	b := &models.RunFolder{Name: "B", ParentID: &a.ID}
	require.NoError(t, s.CreateRunFolder(b))

	c := &models.RunFolder{Name: "C", ParentID: &b.ID}
	require.NoError(t, s.CreateRunFolder(c))

	// Try to move A under C (its own grandchild) — must fail
	err := s.MoveRunFolder(a.ID, &c.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, models.ErrCircularReference)

	// Try to move A under itself — must fail
	err = s.MoveRunFolder(a.ID, &a.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, models.ErrCircularReference)
}

func TestReorderRunFolder(t *testing.T) {
	s := newTestStore(t)

	f1 := &models.RunFolder{Name: "Alpha"}
	f2 := &models.RunFolder{Name: "Beta"}
	f3 := &models.RunFolder{Name: "Gamma"}
	require.NoError(t, s.CreateRunFolder(f1)) // order=10
	require.NoError(t, s.CreateRunFolder(f2)) // order=20
	require.NoError(t, s.CreateRunFolder(f3)) // order=30

	// Move Gamma to before Alpha (order = 5)
	err := s.ReorderRunFolder(f3.ID, 5)
	require.NoError(t, err)

	folders, _ := s.GetRunFolders()
	require.Len(t, folders, 3)
	// Gamma should now be first
	assert.Equal(t, "Gamma", folders[0].Name)
}

func TestReorderRunFolderCollisionRenumber(t *testing.T) {
	s := newTestStore(t)

	f1 := &models.RunFolder{Name: "Alpha"}
	f2 := &models.RunFolder{Name: "Beta"}
	require.NoError(t, s.CreateRunFolder(f1)) // order=10
	require.NoError(t, s.CreateRunFolder(f2)) // order=20

	// Set Beta to same order as Alpha → triggers renumber
	err := s.ReorderRunFolder(f2.ID, 10)
	require.NoError(t, err)

	folders, _ := s.GetRunFolders()
	require.Len(t, folders, 2)
	// After renumber, no two should have the same display_order
	assert.NotEqual(t, folders[0].DisplayOrder, folders[1].DisplayOrder)
}

func TestAssignRunToFolder(t *testing.T) {
	s := newTestStore(t)

	folder := &models.RunFolder{Name: "Draft"}
	require.NoError(t, s.CreateRunFolder(folder))

	run := createTestRunForFolder(t, s, nil)
	assert.Nil(t, run.RunFolderID)

	// Assign to folder
	err := s.AssignRunToFolder(run.ID, &folder.ID)
	require.NoError(t, err)

	retrieved, err := s.GetTestRun(run.ID)
	require.NoError(t, err)
	require.NotNil(t, retrieved.RunFolderID)
	assert.Equal(t, folder.ID, *retrieved.RunFolderID)
}

func TestAssignRunToFolderUnassign(t *testing.T) {
	s := newTestStore(t)

	folder := &models.RunFolder{Name: "Draft"}
	require.NoError(t, s.CreateRunFolder(folder))

	run := createTestRunForFolder(t, s, &folder.ID)
	require.NotNil(t, run.RunFolderID)

	// Unassign
	err := s.AssignRunToFolder(run.ID, nil)
	require.NoError(t, err)

	retrieved, err := s.GetTestRun(run.ID)
	require.NoError(t, err)
	assert.Nil(t, retrieved.RunFolderID)
}

func TestGetRunFoldersOrdering(t *testing.T) {
	s := newTestStore(t)

	names := []string{"C", "A", "B"}
	for _, n := range names {
		require.NoError(t, s.CreateRunFolder(&models.RunFolder{Name: n}))
	}

	folders, err := s.GetRunFolders()
	require.NoError(t, err)
	require.Len(t, folders, 3)

	// Verify ascending display_order
	for i := 1; i < len(folders); i++ {
		assert.Less(t, folders[i-1].DisplayOrder, folders[i].DisplayOrder)
	}
}
