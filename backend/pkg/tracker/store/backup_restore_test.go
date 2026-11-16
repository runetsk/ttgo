package store

import (
	"os"
	"path/filepath"
	"testing"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newFileStore opens a real file-backed store (VACUUM INTO and restore need a
// file, not :memory:).
func newFileStore(t *testing.T) *Store {
	t.Helper()
	t.Chdir(t.TempDir())
	s, err := New("ttgo.db")
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestBackupToFile_ProducesValidDatabase(t *testing.T) {
	s := newFileStore(t)
	root, err := s.CreateFolder("Root", nil)
	require.NoError(t, err)
	require.NoError(t, s.CreateTestCase(&models.TestCase{Name: "TC", FolderID: root.ID}))

	dest := filepath.Join(t.TempDir(), "snap.db")
	require.NoError(t, s.BackupToFile(dest))

	// The snapshot is a valid TTGO database carrying the data.
	require.NoError(t, ValidateBackupFile(dest))
	snap, err := New(dest)
	require.NoError(t, err)
	defer snap.Close()
	tests, err := snap.ListTestCases(TestCaseFilter{FolderIDs: []string{root.ID}})
	require.NoError(t, err)
	assert.Len(t, tests, 1)
}

func TestRestoreDatabase_RestoresSnapshot(t *testing.T) {
	s := newFileStore(t)
	root, err := s.CreateFolder("Root", nil)
	require.NoError(t, err)
	require.NoError(t, s.CreateTestCase(&models.TestCase{Name: "V1", FolderID: root.ID}))

	snap := filepath.Join(t.TempDir(), "v1.db")
	require.NoError(t, s.BackupToFile(snap))

	// Mutate after the snapshot, then restore it.
	require.NoError(t, s.CreateTestCase(&models.TestCase{Name: "V2", FolderID: root.ID}))
	require.NoError(t, s.RestoreDatabase(snap))

	tests, err := s.ListTestCases(TestCaseFilter{FolderIDs: []string{root.ID}})
	require.NoError(t, err)
	require.Len(t, tests, 1, "V2 (added after the snapshot) must be gone")
	assert.Equal(t, "V1", tests[0].Name)
}

func TestRestoreDatabase_RollsBackOnInvalidReplacement(t *testing.T) {
	s := newFileStore(t)
	root, err := s.CreateFolder("KeepMe", nil)
	require.NoError(t, err)
	require.NoError(t, s.CreateTestCase(&models.TestCase{Name: "Original", FolderID: root.ID}))

	// A non-database file: installing it must fail to reopen and roll back to the
	// previous database rather than leaving the store broken.
	garbage := filepath.Join(t.TempDir(), "garbage.db")
	require.NoError(t, os.WriteFile(garbage, []byte("this is definitely not a sqlite database"), 0o644))

	err = s.RestoreDatabase(garbage)
	require.Error(t, err, "installing a non-database must fail")

	// The original data survived and the store is still usable.
	tests, lerr := s.ListTestCases(TestCaseFilter{FolderIDs: []string{root.ID}})
	require.NoError(t, lerr)
	require.Len(t, tests, 1)
	assert.Equal(t, "Original", tests[0].Name)
}
