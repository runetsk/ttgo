package aigen

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCriticSchemaJSONIsStrict(t *testing.T) {
	var schema map[string]interface{}
	require.NoError(t, json.Unmarshal(CriticSchemaJSON, &schema))
	assert.Equal(t, "object", schema["type"])
	assert.Equal(t, false, schema["additionalProperties"])
}

func TestParseCriticResponse(t *testing.T) {
	fs, err := ParseCriticResponse("```json\n{\"findings\":[{\"draft_index\":1,\"dimension\":\"relevance\",\"message\":\"drifts off the requirement\"}]}\n```")
	require.NoError(t, err)
	require.Len(t, fs, 1)
	assert.Equal(t, 1, fs[0].DraftIndex)

	_, err = ParseCriticResponse("not json at all")
	assert.Error(t, err)

	fs, err = ParseCriticResponse(`{"findings":[]}`)
	require.NoError(t, err)
	assert.Empty(t, fs)
}
