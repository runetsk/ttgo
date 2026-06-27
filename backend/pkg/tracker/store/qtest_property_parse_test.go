package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseQTestProperty_UsesAlternateKeys(t *testing.T) {
	property := parseQTestProperty([]byte(`{
		"field_name": "Priority",
		"field_type": "PriorityTestCaseSystemField",
		"value": { "activeValue": "Medium" }
	}`))

	assert.Equal(t, "Priority", property.Name)
	assert.Equal(t, "PriorityTestCaseSystemField", property.FieldType)
	assert.Equal(t, "Medium", property.ValueText)
}

func TestParseQTestProperty_UsesNestedFieldName(t *testing.T) {
	property := parseQTestProperty([]byte(`{
		"field": { "name": "Execution Type" },
		"value_text": "Manual - UI"
	}`))

	assert.Equal(t, "Execution Type", property.Name)
	assert.Equal(t, "Manual - UI", property.ValueText)
}
