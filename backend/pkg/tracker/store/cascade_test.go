package store

import (
	"testing"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDeleteCustomFieldDefinitionCascadesValues verifies F-013: deleting a
// definition removes its values atomically rather than orphaning them.
func TestDeleteCustomFieldDefinitionCascadesValues(t *testing.T) {
	s := newTestStore(t)
	folder, err := s.CreateFolder("F", nil)
	require.NoError(t, err)
	tc := &models.TestCase{ID: "tc1", Name: "T", FolderID: folder.ID}
	require.NoError(t, s.DB().Create(tc).Error)

	def := &models.CustomFieldDefinition{ID: "f1", Name: "Severity", Type: "TEXT"}
	require.NoError(t, s.CreateCustomFieldDefinition(def))
	require.NoError(t, s.DB().Create(&models.CustomFieldValue{
		ID: "v1", TestCaseID: "tc1", CustomFieldID: "f1", Value: []byte(`"High"`),
	}).Error)

	require.NoError(t, s.DeleteCustomFieldDefinition("f1"))

	var count int64
	s.DB().Model(&models.CustomFieldValue{}).Where("custom_field_id = ?", "f1").Count(&count)
	assert.Equal(t, int64(0), count, "values must be cascaded with the definition, not orphaned")
}

// TestDeleteRequirementCascadesFullClosure verifies F-036: deleting a requirement
// removes the entire descendant closure (children AND grandchildren), not one level.
func TestDeleteRequirementCascadesFullClosure(t *testing.T) {
	s := newTestStore(t)
	parent := "" // root
	mk := func(id string, parentID *string) {
		require.NoError(t, s.DB().Create(&models.Requirement{
			ID: id, Identifier: id, Title: id, ParentID: parentID,
		}).Error)
	}
	mk("A", nil)
	a := "A"
	mk("B", &a)
	b := "B"
	mk("C", &b) // grandchild
	_ = parent

	require.NoError(t, s.DeleteRequirement("A"))

	var count int64
	s.DB().Model(&models.Requirement{}).Where("id IN ?", []string{"A", "B", "C"}).Count(&count)
	assert.Equal(t, int64(0), count, "deleting A must cascade to child B and grandchild C")
}
