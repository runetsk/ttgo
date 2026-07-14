package ai

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"ttgo/pkg/tracker/aigen"
	"ttgo/pkg/tracker/models"
)

func TestBuildRegenerationPrompt(t *testing.T) {
	req := &models.Requirement{Identifier: "REQ-1", Title: "Login", Description: "<ul><li>User can sign in</li></ul>"}
	original := models.DraftContent{
		Name: "Vague test", Category: "Functional", Description: "d",
		Steps: []models.GeneratedStep{{Action: "Do it", ExpectedResult: "It works"}},
	}

	p := buildRegenerationPrompt(req, original, "focus on the lockout rule", "repair_findings", []aigen.Finding{
		{Field: "steps[0].expected_result", Code: "vague_expected", Message: "not observable", Severity: aigen.SeverityWarning},
	})

	assert.Contains(t, p, "REQ-1")
	assert.Contains(t, p, "- User can sign in", "description is HTML-stripped")
	assert.Contains(t, p, `"Vague test"`, "original draft embedded as JSON")
	assert.Contains(t, p, "steps[0].expected_result: not observable", "findings listed")
	assert.Contains(t, p, "focus on the lockout rule")
	assert.Contains(t, p, `{"test_cases"`, "canonical single-case envelope demanded")
	assert.Contains(t, p, "exactly ONE")

	for _, action := range []string{"make_more_specific", "add_negative_case"} {
		assert.NotEmpty(t, regenActionText[action], "action %s has guidance text", action)
		assert.True(t, strings.Contains(buildRegenerationPrompt(req, original, "", action, nil), regenActionText[action]))
	}
}
