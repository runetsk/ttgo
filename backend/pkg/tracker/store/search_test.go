package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ttgo/pkg/tracker/models"

	"github.com/google/uuid"
)

func seedTestCases(t *testing.T, s *Store, names []string) {
	t.Helper()
	// Create a parent folder first
	folderID := uuid.New().String()
	require.NoError(t, s.db.Exec("INSERT INTO folders (id, name, created_at, updated_at) VALUES (?, 'Test Folder', datetime('now'), datetime('now'))", folderID).Error)

	for _, name := range names {
		tc := models.TestCase{ID: uuid.New().String(), FolderID: folderID, Name: name}
		require.NoError(t, s.db.Create(&tc).Error)
	}
	// Populate FTS index
	require.NoError(t, s.PopulateFTSIndex())
}

// T032: SearchTestCases returns matching test IDs after FTS5 table is populated
func TestSearchTestCasesReturnsMatches(t *testing.T) {
	s, err := New(":memory:")
	require.NoError(t, err)
	seedTestCases(t, s, []string{"Login flow test", "Logout test", "Register user"})

	results, total, err := s.SearchTestCases("login", 10, 0)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, 1)
	assert.NotEmpty(t, results)
	found := false
	for _, r := range results {
		if r.Name == "Login flow test" {
			found = true
		}
	}
	assert.True(t, found, "should find 'Login flow test'")
}

// T033: Special characters in query don't cause 500 error
func TestSearchSpecialCharactersNoError(t *testing.T) {
	s, err := New(":memory:")
	require.NoError(t, err)
	seedTestCases(t, s, []string{"Normal test"})

	for _, q := range []string{`"`, `'`, `\`, `--`, `; DROP`, `%`, `*`, `(""`} {
		results, _, err := s.SearchTestCases(q, 10, 0)
		assert.NoError(t, err, "query %q should not cause error", q)
		_ = results
	}
}

// T034: Empty query returns empty results, not error
func TestSearchEmptyQueryReturnsEmpty(t *testing.T) {
	s, err := New(":memory:")
	require.NoError(t, err)

	results, total, err := s.SearchTestCases("", 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, results)
}

// T034: Nonexistent query returns empty results
func TestSearchNonexistentQueryReturnsEmpty(t *testing.T) {
	s, err := New(":memory:")
	require.NoError(t, err)
	seedTestCases(t, s, []string{"Actual test case"})

	results, total, err := s.SearchTestCases("zzz_nonexistent_xyz_abc", 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, results)
}
