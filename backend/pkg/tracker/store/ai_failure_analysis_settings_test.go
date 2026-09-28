package store

import (
	"testing"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

func TestFailureAnalysisSettingsSeeded(t *testing.T) {
	s := newTestStore(t)
	got, err := s.GetFailureAnalysisSettings()
	require.NoError(t, err)
	require.Equal(t, false, got.EnabledOnCompletion)
	require.Equal(t, 20, got.MaxAnalysesPerRun)
	require.Equal(t, true, got.DedupEnabled)
	require.Equal(t, true, got.RedactionEnabled)
	require.Equal(t, failureanalysis.DefaultPromptTemplate, got.PromptTemplate)
	require.Equal(t, failureanalysis.DefaultPromptTemplate, got.DefaultPromptTemplate)
}

func TestUpdateFailureAnalysisSettings(t *testing.T) {
	s := newTestStore(t)
	_, err := s.UpdateFailureAnalysisSettings(&models.AIFailureAnalysisSettings{
		EnabledOnCompletion: true, MaxAnalysesPerRun: 5, DedupEnabled: false,
		RedactionEnabled: true, PromptTemplate: "custom",
	})
	require.NoError(t, err)

	got, err := s.GetFailureAnalysisSettings()
	require.NoError(t, err)
	require.True(t, got.EnabledOnCompletion)
	require.Equal(t, 5, got.MaxAnalysesPerRun)
	require.False(t, got.DedupEnabled)
	require.Equal(t, "custom", got.PromptTemplate)
	require.Equal(t, failureanalysis.DefaultPromptTemplate, got.DefaultPromptTemplate)
}

func TestResetFailureAnalysisPrompt(t *testing.T) {
	s := newTestStore(t)
	_, err := s.UpdateFailureAnalysisSettings(&models.AIFailureAnalysisSettings{
		PromptTemplate: "custom", MaxAnalysesPerRun: 20, DedupEnabled: true, RedactionEnabled: true,
	})
	require.NoError(t, err)

	require.NoError(t, s.ResetFailureAnalysisPrompt())

	got, err := s.GetFailureAnalysisSettings()
	require.NoError(t, err)
	require.Equal(t, failureanalysis.DefaultPromptTemplate, got.PromptTemplate)
}

// TestSeedFailureAnalysisUpgradesUnmodifiedPrompt verifies that an install still
// running the previous shipped default (the admin never edited it) is auto-upgraded
// to the new default on the next boot.
func TestSeedFailureAnalysisUpgradesUnmodifiedPrompt(t *testing.T) {
	s := newTestStore(t)

	// Simulate an install that booted on a previous shipped default: both the
	// stored template and the default_prompt_template column hold that old text.
	const oldDefault = "OLD SHIPPED DEFAULT TEMPLATE"
	require.NoError(t, s.db.Model(&models.AIFailureAnalysisSettings{}).
		Where("id = ?", failureAnalysisSettingsID).
		Updates(map[string]interface{}{
			"prompt_template":         oldDefault,
			"default_prompt_template": oldDefault,
		}).Error)

	require.NoError(t, s.seedFailureAnalysisSettings())

	got, err := s.GetFailureAnalysisSettings()
	require.NoError(t, err)
	require.Equal(t, failureanalysis.DefaultPromptTemplate, got.PromptTemplate)
	require.Equal(t, failureanalysis.DefaultPromptTemplate, got.DefaultPromptTemplate)
}

// TestSeedFailureAnalysisPreservesCustomizedPrompt verifies that an admin-customized
// template (diverged from the previous default) is left untouched on upgrade; only
// the default_prompt_template column realigns to the current default.
func TestSeedFailureAnalysisPreservesCustomizedPrompt(t *testing.T) {
	s := newTestStore(t)

	// default_prompt_template holds the previous default, but prompt_template
	// diverges from it — the admin hand-edited the prompt.
	const oldDefault = "OLD SHIPPED DEFAULT TEMPLATE"
	const custom = "admin hand-edited template"
	require.NoError(t, s.db.Model(&models.AIFailureAnalysisSettings{}).
		Where("id = ?", failureAnalysisSettingsID).
		Updates(map[string]interface{}{
			"prompt_template":         custom,
			"default_prompt_template": oldDefault,
		}).Error)

	require.NoError(t, s.seedFailureAnalysisSettings())

	got, err := s.GetFailureAnalysisSettings()
	require.NoError(t, err)
	require.Equal(t, custom, got.PromptTemplate)
	require.Equal(t, failureanalysis.DefaultPromptTemplate, got.DefaultPromptTemplate)
}

func TestFailureAnalysisSettings_ParallelGroups(t *testing.T) {
	s := newTestStore(t)
	got, err := s.GetFailureAnalysisSettings()
	require.NoError(t, err)
	require.Equal(t, models.DefaultParallelGroups, got.ParallelGroups, "a new install analyzes groups four at a time")

	base := models.AIFailureAnalysisSettings{MaxAnalysesPerRun: 20, DedupEnabled: true, RedactionEnabled: true, PromptTemplate: "p"}
	withSix := base
	withSix.ParallelGroups = 6
	got, err = s.UpdateFailureAnalysisSettings(&withSix)
	require.NoError(t, err)
	require.Equal(t, 6, got.ParallelGroups)

	got, err = s.UpdateFailureAnalysisSettings(&base) // a caller that predates the setting
	require.NoError(t, err)
	require.Equal(t, 6, got.ParallelGroups, "0 keeps the stored value")
}

func TestFailureAnalysisSettings_LLMLatency(t *testing.T) {
	s := newTestStore(t)
	got, err := s.GetFailureAnalysisSettings()
	require.NoError(t, err)
	require.Equal(t, models.DefaultLLMCallTimeoutSeconds, got.LLMCallTimeoutSeconds, "a new install times calls out at 45 s")
	require.Equal(t, 0, got.HedgeAfterSeconds, "hedging is off by default")

	base := models.AIFailureAnalysisSettings{MaxAnalysesPerRun: 20, DedupEnabled: true, RedactionEnabled: true, PromptTemplate: "p"}
	tuned := base
	tuned.LLMCallTimeoutSeconds, tuned.HedgeAfterSeconds = 60, 10
	got, err = s.UpdateFailureAnalysisSettings(&tuned)
	require.NoError(t, err)
	require.Equal(t, 60, got.LLMCallTimeoutSeconds)
	require.Equal(t, 10, got.HedgeAfterSeconds)

	got, err = s.UpdateFailureAnalysisSettings(&base) // a caller that predates the settings
	require.NoError(t, err)
	require.Equal(t, 60, got.LLMCallTimeoutSeconds, "0 keeps the stored pair")
	require.Equal(t, 10, got.HedgeAfterSeconds)

	off := base
	off.LLMCallTimeoutSeconds, off.HedgeAfterSeconds = 60, 0
	got, err = s.UpdateFailureAnalysisSettings(&off)
	require.NoError(t, err)
	require.Equal(t, 0, got.HedgeAfterSeconds, "a set timeout writes the hedge too, so it can be switched off")
}
