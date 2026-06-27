package store

import (
	"testing"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImportQTestTestCases_CreatesTestCaseAndMapping(t *testing.T) {
	s := newTestStore(t)

	require.NoError(t, s.CreateCustomFieldDefinition(&models.CustomFieldDefinition{
		Name: "QTestId",
		Type: models.FieldTypeText,
	}))
	folder, err := s.CreateFolder("Imported", nil)
	require.NoError(t, err)

	cfg := &models.QTestConfig{BaseURL: "https://qtest.example.com"}
	result, err := s.ImportQTestTestCases(cfg, 42, 77, "Root / Login", folder.ID, []models.QTestRemoteTestCase{
		{
			ID:          46260677,
			PID:         "TC-101",
			Name:        "Login works",
			Description: "<p>Imported</p>",
			ParentID:    77,
			Steps: []models.QTestRemoteStep{
				{Description: "Open login", Expected: "Form is visible"},
				{Description: "Submit credentials", Expected: "Dashboard opens"},
			},
		},
	}, "skip", false)
	require.NoError(t, err)
	require.Equal(t, 1, result.Succeeded)
	require.Len(t, result.Items, 1)

	created, err := s.GetTestCase(result.Items[0].TestCaseID)
	require.NoError(t, err)
	require.Equal(t, folder.ID, created.FolderID)
	require.Equal(t, "Login works", created.Name)
	require.Len(t, created.Steps, 2)
	assert.Equal(t, "Open login", created.Steps[0].Action)

	found, err := s.GetTestCaseByCustomField("QTestId", "46260677")
	require.NoError(t, err)
	require.Equal(t, created.ID, found.ID)

	mapping, err := s.GetQTestMappingByTestCase(created.ID)
	require.NoError(t, err)
	require.NotNil(t, mapping)
	assert.Equal(t, int64(46260677), mapping.QTestTestCaseID)
	assert.Equal(t, int64(42), mapping.QTestProjectID)
	assert.Equal(t, "Root / Login", mapping.QTestModulePath)
}

func TestImportQTestTestCases_BackfillsMappingForExistingCaseOnSkip(t *testing.T) {
	s := newTestStore(t)

	def := &models.CustomFieldDefinition{
		Name: "QTestId",
		Type: models.FieldTypeText,
	}
	require.NoError(t, s.CreateCustomFieldDefinition(def))

	folder, err := s.CreateFolder("Existing", nil)
	require.NoError(t, err)
	testCase := &models.TestCase{
		Name:     "Already imported",
		FolderID: folder.ID,
		CustomValues: []*models.CustomFieldValue{
			{
				CustomFieldID: def.ID,
				Value:         []byte(`"999"`),
			},
		},
	}
	require.NoError(t, s.CreateTestCase(testCase))

	cfg := &models.QTestConfig{BaseURL: "https://qtest.example.com"}
	result, err := s.ImportQTestTestCases(cfg, 7, 8, "Root / Existing", folder.ID, []models.QTestRemoteTestCase{
		{
			ID:       999,
			PID:      "TC-999",
			Name:     "Already imported",
			ParentID: 8,
		},
	}, "skip", false)
	require.NoError(t, err)
	require.Equal(t, 1, result.Skipped)

	mapping, err := s.GetQTestMappingByTestCase(testCase.ID)
	require.NoError(t, err)
	require.NotNil(t, mapping)
	assert.Equal(t, int64(999), mapping.QTestTestCaseID)
	assert.Equal(t, "TC-999", mapping.QTestTestCasePID)
	assert.Equal(t, int64(7), mapping.QTestProjectID)
}

func TestImportQTestTestCases_PreservesHierarchyForSubmodules(t *testing.T) {
	s := newTestStore(t)

	require.NoError(t, s.CreateCustomFieldDefinition(&models.CustomFieldDefinition{
		Name: "QTestId",
		Type: models.FieldTypeText,
	}))
	rootFolder, err := s.CreateFolder("Root Import", nil)
	require.NoError(t, err)

	cfg := &models.QTestConfig{BaseURL: "https://qtest.example.com"}
	result, err := s.ImportQTestTestCases(cfg, 42, 10, "Platform", rootFolder.ID, []models.QTestRemoteTestCase{
		{
			ID:          1001,
			PID:         "TC-1001",
			Name:        "Nested case",
			Description: "Imported from child module",
			ParentID:    12,
			ModuleID:    12,
			ModulePath:  "Platform / Web / Auth",
		},
	}, "skip", true)
	require.NoError(t, err)
	require.Equal(t, 1, result.Succeeded)

	var webFolder models.Folder
	require.NoError(t, s.db.Where("name = ? AND parent_id = ?", "Web", rootFolder.ID).First(&webFolder).Error)
	var authFolder models.Folder
	require.NoError(t, s.db.Where("name = ? AND parent_id = ?", "Auth", webFolder.ID).First(&authFolder).Error)

	created, err := s.GetTestCase(result.Items[0].TestCaseID)
	require.NoError(t, err)
	assert.Equal(t, authFolder.ID, created.FolderID)

	mapping, err := s.GetQTestMappingByTestCase(created.ID)
	require.NoError(t, err)
	require.NotNil(t, mapping)
	assert.Equal(t, int64(12), mapping.QTestModuleID)
	assert.Equal(t, "Platform / Web / Auth", mapping.QTestModulePath)
}

func TestImportQTestTestCases_MapsMatchingQTestPropertiesToCustomFields(t *testing.T) {
	s := newTestStore(t)

	require.NoError(t, s.CreateCustomFieldDefinition(&models.CustomFieldDefinition{
		Name: "QTestId",
		Type: models.FieldTypeText,
	}))
	typeField := &models.CustomFieldDefinition{
		Name:    "Type",
		Type:    models.FieldTypeSelect,
		Options: []byte(`["Manual","Automation","Performance","Scenario"]`),
	}
	executionTypeField := &models.CustomFieldDefinition{
		Name:    "Execution Type",
		Type:    models.FieldTypeSelect,
		Options: []byte(`["UI","API"]`),
	}
	priorityField := &models.CustomFieldDefinition{
		Name:    "Priority",
		Type:    models.FieldTypeSelect,
		Options: []byte(`["Undecided","Low","Medium","High","Urgent"]`),
	}
	require.NoError(t, s.CreateCustomFieldDefinition(typeField))
	require.NoError(t, s.CreateCustomFieldDefinition(executionTypeField))
	require.NoError(t, s.CreateCustomFieldDefinition(priorityField))

	folder, err := s.CreateFolder("Mapped", nil)
	require.NoError(t, err)

	cfg := &models.QTestConfig{BaseURL: "https://qtest.example.com"}
	result, err := s.ImportQTestTestCases(cfg, 42, 77, "Root / Login", folder.ID, []models.QTestRemoteTestCase{
		{
			ID:       46260777,
			PID:      "TC-777",
			Name:     "Property mapping",
			ParentID: 77,
			Properties: []models.QTestProperty{
				{Name: "Type", ValueText: "Manual"},
				{Name: "Priority", ValueText: "Medium"},
				{Name: "Execution Type", ValueText: "Manual - UI"},
				{Name: "Automation Status", ValueText: "Automated"},
			},
		},
	}, "skip", false)
	require.NoError(t, err)
	require.Equal(t, 1, result.Succeeded)

	created, err := s.GetTestCase(result.Items[0].TestCaseID)
	require.NoError(t, err)

	valuesByFieldID := make(map[string]string, len(created.CustomValues))
	for _, value := range created.CustomValues {
		valuesByFieldID[value.CustomFieldID] = string(value.Value)
	}

	assert.Equal(t, `"Manual"`, valuesByFieldID[typeField.ID])
	assert.Equal(t, `"Medium"`, valuesByFieldID[priorityField.ID])
	assert.Equal(t, `"UI"`, valuesByFieldID[executionTypeField.ID])
}
