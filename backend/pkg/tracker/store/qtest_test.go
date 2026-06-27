package store

import (
	"testing"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

// seedQTestMapping creates a test case in folderID and a QTest mapping for it.
func seedQTestMapping(t *testing.T, s *Store, tcID, folderID string, qtestTCID int64) {
	t.Helper()
	require.NoError(t, s.db.Create(&models.TestCase{ID: tcID, Name: tcID, FolderID: folderID}).Error)
	_, err := s.CreateQTestMapping(tcID, qtestTCID, "PID", "Module", 1, "http://qtest.example/"+tcID, "hash", 1)
	require.NoError(t, err)
}

func TestBulkDeleteQTestMappings(t *testing.T) {
	s := newTestStore(t)
	root, err := s.CreateFolder("Root", nil)
	require.NoError(t, err)

	seedQTestMapping(t, s, "tc-1", root.ID, 101)
	seedQTestMapping(t, s, "tc-2", root.ID, 102)

	n, err := s.BulkDeleteQTestMappings([]string{"tc-1", "tc-2"})
	require.NoError(t, err)
	require.Equal(t, 2, n)

	// Empty input is a no-op.
	n, err = s.BulkDeleteQTestMappings(nil)
	require.NoError(t, err)
	require.Equal(t, 0, n)

	// Already-deleted IDs delete nothing.
	n, err = s.BulkDeleteQTestMappings([]string{"tc-1"})
	require.NoError(t, err)
	require.Equal(t, 0, n)
}

func TestUnlinkQTestMappingsByFolder(t *testing.T) {
	s := newTestStore(t)
	parent, err := s.CreateFolder("Parent", nil)
	require.NoError(t, err)
	child, err := s.CreateFolder("Child", &parent.ID)
	require.NoError(t, err)

	seedQTestMapping(t, s, "tc-a", parent.ID, 201)
	seedQTestMapping(t, s, "tc-b", child.ID, 202)

	// Non-recursive removes only the parent folder's mapping.
	n, err := s.UnlinkQTestMappingsByFolder(parent.ID, false)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	// Recursive removes the remaining descendant mapping.
	n, err = s.UnlinkQTestMappingsByFolder(parent.ID, true)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	// Empty folder id is an error.
	_, err = s.UnlinkQTestMappingsByFolder("", true)
	require.Error(t, err)
}

func TestIsModuleChildOf(t *testing.T) {
	rootMod := &models.QTestModule{ID: 1, Name: "R"} // nil ParentID == root
	pid := int64(5)
	child := &models.QTestModule{ID: 2, Name: "C", ParentID: &pid}

	require.True(t, isModuleChildOf(rootMod, 0))
	require.False(t, isModuleChildOf(rootMod, 5))
	require.True(t, isModuleChildOf(child, 5))
	require.False(t, isModuleChildOf(child, 0))
}

func TestFindChildQTestModuleByName(t *testing.T) {
	p := int64(1)
	tree := []*models.QTestModule{
		{ID: 1, Name: "Root", Children: []*models.QTestModule{
			{ID: 2, Name: "Alpha", ParentID: &p},
			{ID: 3, Name: "Beta", ParentID: &p},
		}},
	}

	require.Equal(t, int64(1), findChildQTestModuleByName(tree, 0, "Root"))
	require.Equal(t, int64(2), findChildQTestModuleByName(tree, 1, "Alpha"))
	require.Equal(t, int64(3), findChildQTestModuleByName(tree, 1, "Beta"))
	require.Equal(t, int64(0), findChildQTestModuleByName(tree, 1, "Gamma"))
	require.Equal(t, int64(0), findChildQTestModuleByName(tree, 99, "Alpha"))
}

func TestMergeQTestBulkResults(t *testing.T) {
	dst := &models.QTestBulkResult{Total: 1, Succeeded: 1}
	src := &models.QTestBulkResult{
		Total: 2, Succeeded: 1, Failed: 1, RateLimited: true,
		Items: []models.QTestBulkResultItem{{TestCaseID: "x"}},
	}

	mergeQTestBulkResults(dst, src)
	require.Equal(t, 3, dst.Total)
	require.Equal(t, 2, dst.Succeeded)
	require.Equal(t, 1, dst.Failed)
	require.True(t, dst.RateLimited)
	require.Len(t, dst.Items, 1)

	// nil src is a no-op.
	mergeQTestBulkResults(dst, nil)
	require.Equal(t, 3, dst.Total)
}
