package api

import (
	"testing"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"

	"github.com/stretchr/testify/require"
)

func resolverStore(t *testing.T) *store.Store {
	t.Helper()
	t.Chdir(t.TempDir())
	s, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	cfg := &models.LLMProviderConfig{Label: "gen", ProviderType: "openai", APIKey: "k", ModelName: "gpt-x", Enabled: true}
	require.NoError(t, s.CreateProviderConfig(cfg)) // store/ai_generation.go:18 returns only an error
	require.NoError(t, s.SetDefaultProviderConfig(cfg.ID))
	return s
}

func enableTypeSafe(t *testing.T, s *store.Store, auto bool) {
	t.Helper()
	key, en := "ts-key-1234", true
	_, err := s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{APIKey: &key, Enabled: &en, AllowAutoFailureAnalysis: &auto})
	require.NoError(t, err)
}

func TestResolver_ManualUsesTypeSafeAutoNeedsConsent(t *testing.T) {
	s := resolverStore(t)
	enableTypeSafe(t, s, false)
	r := newAnalyzeDepsResolver(s)

	manual, err := r(models.RunAnalysisJobTriggerManual)
	require.NoError(t, err)
	require.NotNil(t, manual.Narrative)
	require.NotNil(t, manual.Decider)
	require.NotNil(t, manual.Semantic)

	auto, err := r(models.RunAnalysisJobTriggerAutoOnDone)
	require.NoError(t, err)
	require.NotNil(t, auto.Narrative)
	require.Nil(t, auto.Decider, "auto jobs need the per-vendor consent flag")
	require.Nil(t, auto.Semantic)

	enableTypeSafe(t, s, true)
	auto2, err := r(models.RunAnalysisJobTriggerAutoOnDone)
	require.NoError(t, err)
	require.NotNil(t, auto2.Decider)
}

func TestResolver_FeatureSwitchesAndMissingKey(t *testing.T) {
	s := resolverStore(t)
	enableTypeSafe(t, s, false)
	off := false
	_, err := s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{VerdictEngineEnabled: &off})
	require.NoError(t, err)
	d, err := newAnalyzeDepsResolver(s)(models.RunAnalysisJobTriggerManual)
	require.NoError(t, err)
	require.Nil(t, d.Decider)
	require.NotNil(t, d.Semantic, "semantic-only mode still gets a client")

	_, err = s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{ClearAPIKey: true})
	require.NoError(t, err)
	d2, err := newAnalyzeDepsResolver(s)(models.RunAnalysisJobTriggerManual)
	require.NoError(t, err, "a missing key is a fallback, never a job failure")
	require.Nil(t, d2.Decider)
	require.Nil(t, d2.Semantic)
	require.NotNil(t, d2.Narrative)
}
