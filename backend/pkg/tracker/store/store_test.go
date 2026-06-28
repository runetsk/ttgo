package store

import (
	"testing"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *Store {
	s, err := New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestCreateFolder(t *testing.T) {
	s := newTestStore(t)
	f, err := s.CreateFolder("Root", nil)
	require.NoError(t, err)
	assert.Equal(t, "Root", f.Name)
}

func TestFolderTree(t *testing.T) {
	s := newTestStore(t)
	root, _ := s.CreateFolder("Root", nil)
	child, _ := s.CreateFolder("Child", &root.ID)

	tree, err := s.GetFolderTree()
	require.NoError(t, err)
	require.Len(t, tree, 1)
	assert.Equal(t, root.ID, tree[0].ID)
	require.Len(t, tree[0].SubFolders, 1)
	assert.Equal(t, child.ID, tree[0].SubFolders[0].ID)
}

func TestCategories(t *testing.T) {
	s := newTestStore(t)
	category, err := s.CreateCategory("Smoke", "Smoke Tests")
	require.NoError(t, err)

	categories, total, err := s.ListCategories(0, 0, "")
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, categories, 1)
	assert.Equal(t, category.ID, categories[0].ID)
	assert.Equal(t, "Smoke", categories[0].Name)
}

func TestTestCasesAndExecution(t *testing.T) {
	s := newTestStore(t)
	root, _ := s.CreateFolder("Root", nil)
	test := &models.TestCase{Name: "Login Test", FolderID: root.ID, Description: "Verify login"}
	err := s.CreateTestCase(test)
	require.NoError(t, err)

	tests, err := s.ListTestCases(TestCaseFilter{FolderIDs: []string{root.ID}})
	require.NoError(t, err)
	require.Len(t, tests, 1)

}

func TestAssignCategory(t *testing.T) {
	s := newTestStore(t)
	root, _ := s.CreateFolder("Root", nil)
	test := &models.TestCase{Name: "Login", FolderID: root.ID}
	_ = s.CreateTestCase(test)
	category, _ := s.CreateCategory("Functional", "")

	err := s.AssignCategoryToTest(category.ID, test.ID)
	require.NoError(t, err)

	tests, err := s.ListTestCases(TestCaseFilter{CategoryID: &category.ID})
	require.NoError(t, err)
	require.Len(t, tests, 1)
	assert.Equal(t, test.ID, tests[0].ID)
}
func TestListTestCasesRecursive(t *testing.T) {
	s := newTestStore(t)
	parent, _ := s.CreateFolder("Parent", nil)
	child, _ := s.CreateFolder("Child", &parent.ID)
	grandchild, _ := s.CreateFolder("Grandchild", &child.ID)

	_ = s.CreateTestCase(&models.TestCase{Name: "Test 1", FolderID: parent.ID})
	_ = s.CreateTestCase(&models.TestCase{Name: "Test 2", FolderID: child.ID})
	test3 := &models.TestCase{Name: "Test 3", FolderID: grandchild.ID}
	_ = s.CreateTestCase(test3)

	// Test 1: Get tests for parent (should include all 3)
	tests, err := s.ListTestCases(TestCaseFilter{FolderIDs: []string{parent.ID}})
	require.NoError(t, err)
	assert.Len(t, tests, 3)

	// Test 2: Get tests for child (should include 2 and 3)
	tests, err = s.ListTestCases(TestCaseFilter{FolderIDs: []string{child.ID}})
	require.NoError(t, err)
	assert.Len(t, tests, 2)

	// Test 3: Get tests for grandchild (should include only 3)
	tests, err = s.ListTestCases(TestCaseFilter{FolderIDs: []string{grandchild.ID}})
	require.NoError(t, err)
	assert.Len(t, tests, 1)
	assert.Equal(t, test3.ID, tests[0].ID)
}

func TestTestSteps(t *testing.T) {
	s := newTestStore(t)
	root, _ := s.CreateFolder("Root", nil)
	test := &models.TestCase{Name: "Steps Test", FolderID: root.ID}
	require.NoError(t, s.CreateTestCase(test))

	// Add steps
	s1 := &models.TestStep{TestCaseID: test.ID, Action: "Step 1", ExpectedResult: "Result 1", OrderIndex: 0}
	s2 := &models.TestStep{TestCaseID: test.ID, Action: "Step 2", ExpectedResult: "Result 2", OrderIndex: 1}
	require.NoError(t, s.AddTestStep(s1))
	require.NoError(t, s.AddTestStep(s2))

	// Get Steps
	steps, err := s.GetTestSteps(test.ID)
	require.NoError(t, err)
	require.Len(t, steps, 2)
	assert.Equal(t, "Step 1", steps[0].Action)

	// Reorder
	s1.OrderIndex = 1
	s2.OrderIndex = 0
	require.NoError(t, s.UpdateTestStepsOrder([]models.TestStep{*s1, *s2}))

	steps, err = s.GetTestSteps(test.ID)
	require.NoError(t, err)
	assert.Equal(t, "Step 2", steps[0].Action)
	assert.Equal(t, "Step 1", steps[1].Action)

	// Delete
	require.NoError(t, s.DeleteTestStep(s1.ID))
	steps, err = s.GetTestSteps(test.ID)
	require.NoError(t, err)
	require.Len(t, steps, 1)
	assert.Equal(t, "Step 2", steps[0].Action)
}

func TestCustomFields(t *testing.T) {
	s := newTestStore(t)
	// 1. Create Def
	def := &models.CustomFieldDefinition{
		Name:    "Priority",
		Type:    models.FieldTypeSelect,
		Options: []byte(`["Low","High"]`),
	}
	require.NoError(t, s.CreateCustomFieldDefinition(def))

	// 2. Create Test with Value
	root, _ := s.CreateFolder("Root", nil)
	val := &models.CustomFieldValue{
		CustomFieldID: def.ID,
		Value:         []byte(`"High"`),
	}
	test := &models.TestCase{
		Name:         "CF Test",
		FolderID:     root.ID,
		CustomValues: []*models.CustomFieldValue{val},
	}
	require.NoError(t, s.CreateTestCase(test))

	// 3. Verify Retrieval
	fetched, err := s.GetTestCase(test.ID)
	require.NoError(t, err)
	require.Len(t, fetched.CustomValues, 1)
	assert.Equal(t, `"High"`, string(fetched.CustomValues[0].Value))
}

func TestGetTestCaseByCustomField_FindsJSONStoredTextValue(t *testing.T) {
	s := newTestStore(t)

	def := &models.CustomFieldDefinition{
		Name: "ExternalId",
		Type: models.FieldTypeText,
	}
	require.NoError(t, s.CreateCustomFieldDefinition(def))

	root, _ := s.CreateFolder("Root", nil)
	test := &models.TestCase{
		Name:     "external id lookup",
		FolderID: root.ID,
		CustomValues: []*models.CustomFieldValue{
			{
				CustomFieldID: def.ID,
				Value:         []byte(`"46260677"`),
			},
		},
	}
	require.NoError(t, s.CreateTestCase(test))

	found, err := s.GetTestCaseByCustomField("ExternalId", "46260677")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, test.ID, found.ID)
}

func TestUpdateTestCaseCategories(t *testing.T) {
	s := newTestStore(t)
	root, _ := s.CreateFolder("Root", nil)
	test := &models.TestCase{Name: "Suite Test", FolderID: root.ID}
	_ = s.CreateTestCase(test)

	category1, _ := s.CreateCategory("Suite 1", "")
	category2, _ := s.CreateCategory("Suite 2", "")

	// 1. Assign two categories
	test.Categories = []*models.Category{category1, category2}
	err := s.UpdateTestCase(test)
	require.NoError(t, err)

	fetched, _ := s.GetTestCase(test.ID)
	assert.Len(t, fetched.Categories, 2)

	// 2. Remove one category
	test.Categories = []*models.Category{category1}
	err = s.UpdateTestCase(test)
	require.NoError(t, err)

	fetched, _ = s.GetTestCase(test.ID)
	assert.Len(t, fetched.Categories, 1)
	assert.Equal(t, category1.ID, fetched.Categories[0].ID)

	// 3. Clear all categories
	test.Categories = []*models.Category{}
	err = s.UpdateTestCase(test)
	require.NoError(t, err)

	fetched, _ = s.GetTestCase(test.ID)
	assert.Len(t, fetched.Categories, 0)
}

// TestUpdateTestCaseFolderID verifies that updating a test case's FolderID
// moves it to a new folder (drag-and-drop / orphan-recovery use case).
func TestUpdateTestCaseFolderID(t *testing.T) {
	s := newTestStore(t)

	src, _ := s.CreateFolder("Source", nil)
	dst, _ := s.CreateFolder("Destination", nil)

	tc := &models.TestCase{Name: "Movable Test", FolderID: src.ID}
	require.NoError(t, s.CreateTestCase(tc))

	// Confirm initial folder
	got, err := s.GetTestCase(tc.ID)
	require.NoError(t, err)
	assert.Equal(t, src.ID, got.FolderID)

	// Move to destination folder — only FolderID changes, name is preserved
	tc.FolderID = dst.ID
	require.NoError(t, s.UpdateTestCase(tc))

	got, err = s.GetTestCase(tc.ID)
	require.NoError(t, err)
	assert.Equal(t, dst.ID, got.FolderID, "FolderID should be updated to destination")
	assert.Equal(t, "Movable Test", got.Name, "Name should remain unchanged after folder move")
}

func TestGetTestCasesByIDs(t *testing.T) {
	s := newTestStore(t)

	folder, err := s.CreateFolder("ExportFolder", nil)
	require.NoError(t, err)

	tc1 := &models.TestCase{
		Name:        "Login Test",
		FolderID:    folder.ID,
		Description: "<p>Login flow</p>",
		Steps: []*models.TestStep{
			{Action: "Enter username", ExpectedResult: "Accepted", OrderIndex: 0},
			{Action: "Click login", ExpectedResult: "Logged in", OrderIndex: 1},
		},
	}
	require.NoError(t, s.CreateTestCase(tc1))

	tc2 := &models.TestCase{
		Name:        "Logout Test",
		FolderID:    folder.ID,
		Description: "<p>Logout flow</p>",
	}
	require.NoError(t, s.CreateTestCase(tc2))

	// Fetch both by IDs
	results, err := s.GetTestCasesByIDs([]string{tc1.ID, tc2.ID})
	require.NoError(t, err)
	assert.Len(t, results, 2)

	// Verify associations loaded for tc1
	var login *models.TestCase
	for _, tc := range results {
		if tc.Name == "Login Test" {
			login = tc
		}
	}
	require.NotNil(t, login)
	assert.Len(t, login.Steps, 2)

	// Fetch with a non-existent ID — should silently skip
	results2, err := s.GetTestCasesByIDs([]string{tc1.ID, "nonexistent-id"})
	require.NoError(t, err)
	assert.Len(t, results2, 1)

	// Empty IDs — should return empty
	results3, err := s.GetTestCasesByIDs([]string{})
	require.NoError(t, err)
	assert.Len(t, results3, 0)
}

func TestListTestCasesListView(t *testing.T) {
	s := newTestStore(t)
	root, _ := s.CreateFolder("Root", nil)

	tc := &models.TestCase{
		Name:        "With Steps",
		FolderID:    root.ID,
		Description: "body",
		Steps: []*models.TestStep{
			{Action: "a1", ExpectedResult: "r1", OrderIndex: 0},
			{Action: "a2", ExpectedResult: "r2", OrderIndex: 1},
			{Action: "a3", ExpectedResult: "r3", OrderIndex: 2},
		},
	}
	require.NoError(t, s.CreateTestCase(tc))

	// Full view: Steps preloaded, StepsCount not set.
	full, err := s.ListTestCases(TestCaseFilter{FolderIDs: []string{root.ID}})
	require.NoError(t, err)
	require.Len(t, full, 1)
	assert.Len(t, full[0].Steps, 3)
	assert.Equal(t, 0, full[0].StepsCount)

	// List view: Steps empty, StepsCount populated.
	list, err := s.ListTestCases(TestCaseFilter{FolderIDs: []string{root.ID}, ListView: true})
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Empty(t, list[0].Steps, "Steps must NOT be preloaded in list view")
	assert.Empty(t, list[0].CustomValues, "CustomValues must NOT be preloaded in list view")
	assert.Equal(t, 3, list[0].StepsCount)
}
