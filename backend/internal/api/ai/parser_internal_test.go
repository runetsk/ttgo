package ai

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const canonicalEnvelope = `{
  "test_cases": [
    {
      "name": "[Functional] Sign in with valid credentials",
      "category": "Functional",
      "description": "Verifies successful authentication.",
      "source_refs": ["AC-1"],
      "steps": [
        {"action": "Enter user@example.com in the Email field.",
         "expected_result": "The Email field contains user@example.com."}
      ]
    }
  ]
}`

func TestParseLLMResponse_CanonicalEnvelope(t *testing.T) {
	drafts, err := parseLLMResponse(canonicalEnvelope)
	require.NoError(t, err)
	require.Len(t, drafts, 1)
	assert.Equal(t, "[Functional] Sign in with valid credentials", drafts[0].Name)
	assert.Equal(t, "Functional", drafts[0].Category)
	assert.Equal(t, []string{"AC-1"}, drafts[0].SourceRefs)
	require.Len(t, drafts[0].Steps, 1)
	assert.NotEmpty(t, drafts[0].TempID)
}

func TestParseLLMResponse_LegacyArrayStillWorks(t *testing.T) {
	legacy := `[{"name":"N","category":"Functional","description":"d",
		"steps":[{"action":"a","expected_result":"e"}]}]`
	drafts, err := parseLLMResponse(legacy)
	require.NoError(t, err)
	require.Len(t, drafts, 1)
	assert.Empty(t, drafts[0].SourceRefs)
}

func TestParseLLMResponse_EnvelopeInsideFences(t *testing.T) {
	fenced := "```json\n" + canonicalEnvelope + "\n```"
	drafts, err := parseLLMResponse(fenced)
	require.NoError(t, err)
	require.Len(t, drafts, 1)
}
