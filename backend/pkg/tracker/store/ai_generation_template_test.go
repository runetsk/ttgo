package store

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ttgo/pkg/tracker/models"
)

// The AI Generate UI sends exactly these values as {{DETAIL_LEVEL}}
// (frontend DETAIL_LEVELS in AIGenerationContext.jsx); the default templates
// must define each one verbatim, otherwise the model is told to generate at a
// level that matches none of the definitions in the prompt.
func TestDefaultTemplates_DefineFrontendDetailLevels(t *testing.T) {
	for _, level := range []string{"Simplified", "Standard", "Detailed"} {
		marker := `- "` + level + `":`
		assert.Contains(t, defaultPromptTemplate, marker, "standard template must define %q", level)
		assert.Contains(t, defaultParentPromptTemplate, marker, "parent template must define %q", level)
	}
	for _, stale := range []string{"More Detailed", "More Simplified"} {
		assert.NotContains(t, defaultPromptTemplate, stale, "standard template still defines stale level %q", stale)
		assert.NotContains(t, defaultParentPromptTemplate, stale, "parent template still defines stale level %q", stale)
	}
}

// The coverage-guidelines bullet must use the category enum's "Functional"
// name so the distribution guidance and the category field vocabulary agree.
func TestDefaultTemplate_CategoryNamesMatchEnum(t *testing.T) {
	assert.Contains(t, defaultPromptTemplate, "Functional/Happy path")
	assert.NotContains(t, defaultPromptTemplate, "Positive/Happy path")
}

// When the built-in default template changes between releases, an install
// whose content was never customized (content == stored default) must be
// carried along to the new default on startup — same behaviour the parent
// template already has.
func TestTemplateSync_UpgradesUncustomizedContent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sync.db")
	s1, err := New(dbPath)
	require.NoError(t, err)
	// Simulate an install seeded by an older release: content and
	// default_content both hold an outdated built-in template.
	old := "OLD DEFAULT {{TITLE}} {{DESCRIPTION}} {{COVERAGE}}"
	require.NoError(t, s1.db.Model(&models.AIGenTemplate{ID: aiGenTemplateSingletonID}).
		Updates(map[string]interface{}{"content": old, "default_content": old}).Error)
	require.NoError(t, s1.Close())

	s2, err := New(dbPath) // startup sync runs here
	require.NoError(t, err)
	defer s2.Close()
	tmpl, err := s2.GetOrCreateDefaultTemplate()
	require.NoError(t, err)
	assert.Equal(t, defaultPromptTemplate, tmpl.DefaultContent, "default_content must track the latest built-in")
	assert.Equal(t, defaultPromptTemplate, tmpl.Content, "never-customized content must ride along to the new built-in")
}

// A customized template must never be clobbered by the startup sync — only
// default_content moves.
func TestTemplateSync_PreservesCustomizedContent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sync-custom.db")
	s1, err := New(dbPath)
	require.NoError(t, err)
	old := "OLD DEFAULT {{TITLE}}"
	custom := "MY CUSTOM TEMPLATE {{TITLE}} {{COVERAGE}}"
	require.NoError(t, s1.db.Model(&models.AIGenTemplate{ID: aiGenTemplateSingletonID}).
		Updates(map[string]interface{}{"content": custom, "default_content": old}).Error)
	require.NoError(t, s1.Close())

	s2, err := New(dbPath)
	require.NoError(t, err)
	defer s2.Close()
	tmpl, err := s2.GetOrCreateDefaultTemplate()
	require.NoError(t, err)
	assert.Equal(t, defaultPromptTemplate, tmpl.DefaultContent)
	assert.Equal(t, custom, tmpl.Content, "customized content must be preserved")
}
