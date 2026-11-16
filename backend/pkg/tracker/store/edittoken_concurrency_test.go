package store

import (
	"errors"
	"testing"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// seedTestCase creates a test case with steps, a category and a custom value so
// the token covers every editable dimension.
func seedTestCase(t *testing.T, s *Store) *models.TestCase {
	t.Helper()
	root, err := s.CreateFolder("Root", nil)
	require.NoError(t, err)
	cat, err := s.CreateCategory("Smoke", "")
	require.NoError(t, err)
	def := &models.CustomFieldDefinition{Name: "Priority", Type: models.FieldTypeText}
	require.NoError(t, s.CreateCustomFieldDefinition(def))
	tc := &models.TestCase{
		Name:        "Login",
		FolderID:    root.ID,
		Description: "<p>verify</p>",
		Steps: []*models.TestStep{
			{Action: "open", ExpectedResult: "form", OrderIndex: 0},
			{Action: "submit", ExpectedResult: "home", OrderIndex: 1},
		},
		Categories:   []*models.Category{cat},
		CustomValues: []*models.CustomFieldValue{{CustomFieldID: def.ID, Value: []byte(`"high"`)}},
	}
	require.NoError(t, s.CreateTestCase(tc))
	return tc
}

// The read path (GetTestCase) and the write precondition must hash an unchanged
// record identically, or every checked save would be a false conflict.
func TestEditToken_ReadPathMatchesWritePrecondition(t *testing.T) {
	s := newTestStore(t)
	tc := seedTestCase(t, s)

	loaded, err := s.GetTestCase(tc.ID)
	require.NoError(t, err)
	readToken := models.TestCaseEditToken(loaded)

	// A checked save that changes nothing, carrying the read token, must NOT conflict.
	patch := &models.TestCase{ID: tc.ID, Name: loaded.Name, FolderID: loaded.FolderID}
	latest, err := s.PatchTestCaseChecked(patch, &loaded.Description, readToken)
	require.NoError(t, err, "unchanged content with the read token must not conflict")
	require.Nil(t, latest)
}

func TestPatchTestCaseChecked_StaleTokenConflicts(t *testing.T) {
	s := newTestStore(t)
	tc := seedTestCase(t, s)
	loaded, _ := s.GetTestCase(tc.ID)
	staleToken := models.TestCaseEditToken(loaded)

	// Another writer changes the record after the editor loaded it.
	require.NoError(t, s.PatchTestCase(&models.TestCase{ID: tc.ID, Name: "Changed by someone else"}, nil))

	patch := &models.TestCase{ID: tc.ID, Name: "My edit"}
	latest, err := s.PatchTestCaseChecked(patch, nil, staleToken)
	require.ErrorIs(t, err, ErrEditConflict)
	require.NotNil(t, latest, "conflict must return the current persisted record")
	assert.Equal(t, "Changed by someone else", latest.Name)

	// The conflicting save must not have applied.
	after, _ := s.GetTestCase(tc.ID)
	assert.Equal(t, "Changed by someone else", after.Name)
}

func TestPatchTestCaseChecked_CorrectTokenApplies(t *testing.T) {
	s := newTestStore(t)
	tc := seedTestCase(t, s)
	loaded, _ := s.GetTestCase(tc.ID)
	token := models.TestCaseEditToken(loaded)

	patch := &models.TestCase{ID: tc.ID, Name: "Renamed"}
	latest, err := s.PatchTestCaseChecked(patch, nil, token)
	require.NoError(t, err)
	require.Nil(t, latest)
	assert.Equal(t, "Renamed", patch.Name)
	// The saved record carries a new token distinct from the one just consumed.
	assert.NotEqual(t, token, models.TestCaseEditToken(patch))
}

func TestPatchTestCaseChecked_EmptyTokenSkipsCheck(t *testing.T) {
	s := newTestStore(t)
	tc := seedTestCase(t, s)

	// Someone else writes; an empty expected token means "no conflict protection".
	require.NoError(t, s.PatchTestCase(&models.TestCase{ID: tc.ID, Name: "Other"}, nil))
	_, err := s.PatchTestCaseChecked(&models.TestCase{ID: tc.ID, Name: "CLI write"}, nil, "")
	require.NoError(t, err)
	after, _ := s.GetTestCase(tc.ID)
	assert.Equal(t, "CLI write", after.Name)
}

func TestPatchTestCaseChecked_RevertIsNotAConflict(t *testing.T) {
	s := newTestStore(t)
	tc := seedTestCase(t, s)
	loaded, _ := s.GetTestCase(tc.ID)
	token := models.TestCaseEditToken(loaded)
	original := loaded.Name

	// Change then revert to the exact original content.
	require.NoError(t, s.PatchTestCase(&models.TestCase{ID: tc.ID, Name: "Temporary"}, nil))
	require.NoError(t, s.PatchTestCase(&models.TestCase{ID: tc.ID, Name: original}, nil))

	_, err := s.PatchTestCaseChecked(&models.TestCase{ID: tc.ID, Name: "My edit"}, nil, token)
	require.NoError(t, err, "a change-then-revert leaves the token valid")
}

func TestPatchTestCaseChecked_MissingRecord(t *testing.T) {
	s := newTestStore(t)
	_, err := s.PatchTestCaseChecked(&models.TestCase{ID: "does-not-exist", Name: "x"}, nil, "sometoken")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// ── Requirements ──────────────────────────────────────────────────────────────

func TestUpdateRequirementChecked(t *testing.T) {
	s := newTestStore(t)
	req := &models.Requirement{Identifier: "PROJ-1", Title: "T", Description: "D"}
	require.NoError(t, s.CreateRequirement(req))
	loaded, _ := s.GetRequirement(req.ID)
	token := models.RequirementEditToken(loaded)

	// Correct token applies.
	ok := &models.Requirement{ID: req.ID, Identifier: "PROJ-1", Title: "T2", Description: "D"}
	latest, err := s.UpdateRequirementChecked(ok, token)
	require.NoError(t, err)
	require.Nil(t, latest)

	// A stale token (the record moved on) conflicts and returns the latest.
	stale := &models.Requirement{ID: req.ID, Identifier: "PROJ-1", Title: "T3", Description: "D"}
	latest, err = s.UpdateRequirementChecked(stale, token)
	require.ErrorIs(t, err, ErrEditConflict)
	require.NotNil(t, latest)
	assert.Equal(t, "T2", latest.Title)

	// Empty token skips the check.
	_, err = s.UpdateRequirementChecked(&models.Requirement{ID: req.ID, Identifier: "PROJ-1", Title: "T4", Description: "D"}, "")
	require.NoError(t, err)

	// Missing record.
	_, err = s.UpdateRequirementChecked(&models.Requirement{ID: "nope", Identifier: "X", Title: "Y"}, "tok")
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record-not-found, got %v", err)
	}
}
