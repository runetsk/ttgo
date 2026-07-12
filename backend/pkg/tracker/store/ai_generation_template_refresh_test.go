package store

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ttgo/pkg/tracker/models"
)

func TestGetOrCreateDefaultTemplate_RefreshesStaleDefaults(t *testing.T) {
	s := newTestStore(t)

	// Simulate an existing install seeded with an older default (user never customized:
	// content == default_content) and a customized parent template.
	stale := "OLD DEFAULT: return a JSON array"
	custom := "MY CUSTOM PARENT TEMPLATE {{TITLE}}"
	require.NoError(t, s.db.Model(&models.AIGenTemplate{}).
		Where("id = ?", aiGenTemplateSingletonID).
		Updates(map[string]interface{}{
			"content": stale, "default_content": stale,
			"parent_content": custom, "default_parent_content": "OLD PARENT DEFAULT",
		}).Error)

	tmpl, err := s.GetOrCreateDefaultTemplate()
	require.NoError(t, err)

	// Un-customized content follows the new default; defaults always refresh.
	assert.Equal(t, defaultPromptTemplate, tmpl.Content)
	assert.Equal(t, defaultPromptTemplate, tmpl.DefaultContent)
	assert.Equal(t, defaultParentPromptTemplate, tmpl.DefaultParentContent)
	// Customized content is preserved.
	assert.Equal(t, custom, tmpl.ParentContent)
	assert.True(t, strings.Contains(tmpl.Content, `"test_cases"`))
}

// An install whose parent_content was never populated (empty) but whose stored
// default_parent_content is a stale non-empty value must have parent_content
// backfilled to the current default — the case the old bootstrapDB sync block
// handled and refreshDefaultTemplate must preserve.
func TestGetOrCreateDefaultTemplate_BackfillsEmptyParentContent(t *testing.T) {
	s := newTestStore(t)

	require.NoError(t, s.db.Model(&models.AIGenTemplate{}).
		Where("id = ?", aiGenTemplateSingletonID).
		Updates(map[string]interface{}{
			"parent_content": "", "default_parent_content": "OLD PARENT DEFAULT",
		}).Error)

	tmpl, err := s.GetOrCreateDefaultTemplate()
	require.NoError(t, err)

	assert.Equal(t, defaultParentPromptTemplate, tmpl.ParentContent)
	assert.Equal(t, defaultParentPromptTemplate, tmpl.DefaultParentContent)
}
