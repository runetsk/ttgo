package aigen

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ttgo/pkg/tracker/models"
)

func validDraft() models.GeneratedTestCase {
	return models.GeneratedTestCase{
		Name:        "[Functional] Sign in with valid credentials",
		Category:    "Functional",
		Description: "Verifies successful authentication.",
		SourceRefs:  []string{"AC-1"},
		Steps: []models.GeneratedStep{
			{Action: "Enter user@example.com", ExpectedResult: "Field contains the email"},
		},
	}
}

func findingCodes(fs []Finding) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.Field + ":" + f.Code
	}
	return out
}

func TestValidateDraft_CleanDraftHasNoFindings(t *testing.T) {
	assert.Empty(t, ValidateDraft(validDraft()))
}

func TestValidateDraft_RequiredFields(t *testing.T) {
	d := validDraft()
	d.Name = "  "
	d.Steps = []models.GeneratedStep{{Action: "", ExpectedResult: ""}}
	fs := ValidateDraft(d)
	codes := findingCodes(fs)
	assert.Contains(t, codes, "name:required")
	assert.Contains(t, codes, "steps[0].action:required")
	assert.Contains(t, codes, "steps[0].expected_result:required")
	assert.True(t, HasErrors(fs))
}

func TestValidateDraft_NoSteps(t *testing.T) {
	d := validDraft()
	d.Steps = nil
	fs := ValidateDraft(d)
	assert.Contains(t, findingCodes(fs), "steps:no_steps")
	assert.True(t, HasErrors(fs))
}

func TestValidateDraft_MaxLengths(t *testing.T) {
	d := validDraft()
	d.Name = strings.Repeat("x", MaxNameLen+1)
	d.Description = strings.Repeat("x", MaxDescriptionLen+1)
	d.Steps[0].Action = strings.Repeat("x", MaxStepFieldLen+1)
	fs := ValidateDraft(d)
	codes := findingCodes(fs)
	assert.Contains(t, codes, "name:too_long")
	assert.Contains(t, codes, "description:too_long")
	assert.Contains(t, codes, "steps[0].action:too_long")
	assert.True(t, HasErrors(fs))
}

func TestValidateDraft_CategoryWarnings(t *testing.T) {
	d := validDraft()
	d.Category = "Exploratory Vibes"
	fs := ValidateDraft(d)
	require.Len(t, fs, 1)
	assert.Equal(t, "unknown_category", fs[0].Code)
	assert.Equal(t, SeverityWarning, fs[0].Severity)
	assert.False(t, HasErrors(fs), "warnings alone must not block acceptance")

	d.Category = ""
	fs = ValidateDraft(d)
	assert.Contains(t, findingCodes(fs), "category:missing_category")
	assert.False(t, HasErrors(fs))
}

func TestValidateDraft_EveryFindingHasHumanMessage(t *testing.T) {
	d := models.GeneratedTestCase{} // everything wrong
	for _, f := range ValidateDraft(d) {
		assert.NotEmpty(t, f.Message, "finding %s:%s must explain itself", f.Field, f.Code)
		assert.NotEmpty(t, f.Field)
	}
}

func TestEnvelopeSchemaJSON_IsValidJSONWithStrictShape(t *testing.T) {
	var schema map[string]interface{}
	require.NoError(t, json.Unmarshal(EnvelopeSchemaJSON, &schema))
	assert.Equal(t, "object", schema["type"])
	props := schema["properties"].(map[string]interface{})
	_, ok := props["test_cases"]
	assert.True(t, ok)
	assert.Equal(t, false, schema["additionalProperties"])
}
