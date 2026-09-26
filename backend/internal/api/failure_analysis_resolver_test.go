package api

import (
	"context"
	"testing"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"
	"ttgo/pkg/tracker/typesafe"

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
	r := newAnalyzeDepsResolver(s, nil)

	manual, err := r(models.RunAnalysisJobTriggerManual)
	require.NoError(t, err)
	require.NotNil(t, manual.Narrative)
	require.NotNil(t, manual.Decider)
	require.NotNil(t, manual.Semantic)

	auto, err := r(models.RunAnalysisJobTriggerAutoOnDone)
	require.NoError(t, err)
	require.Nil(t, auto.Narrative, "the provider has not been approved for automatic analysis")
	require.Contains(t, auto.LLMUnavailableReason, "not approved for automatic analysis")
	require.Nil(t, auto.Decider, "auto jobs need the per-vendor consent flag")
	require.Nil(t, auto.Semantic)

	approveProviderForAuto(t, s)
	auto, err = r(models.RunAnalysisJobTriggerAutoOnDone)
	require.NoError(t, err)
	require.NotNil(t, auto.Narrative, "consent is read when the job runs")

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
	d, err := newAnalyzeDepsResolver(s, nil)(models.RunAnalysisJobTriggerManual)
	require.NoError(t, err)
	require.Nil(t, d.Decider)
	require.NotNil(t, d.Semantic, "semantic-only mode still gets a client")

	_, err = s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{ClearAPIKey: true})
	require.NoError(t, err)
	d2, err := newAnalyzeDepsResolver(s, nil)(models.RunAnalysisJobTriggerManual)
	require.NoError(t, err, "a missing key is a fallback, never a job failure")
	require.Nil(t, d2.Decider)
	require.Nil(t, d2.Semantic)
	require.NotNil(t, d2.Narrative)
}

func TestResolver_ExplanationsOffSkipNarrationButKeepProvider(t *testing.T) {
	s := resolverStore(t)
	enableTypeSafe(t, s, false)
	off := false
	_, err := s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{NarrativeEnabled: &off})
	require.NoError(t, err)
	d, err := newAnalyzeDepsResolver(s, nil)(models.RunAnalysisJobTriggerManual)
	require.NoError(t, err)
	require.True(t, d.NarrativeSkipped)
	require.NotNil(t, d.Narrative, "the provider stays attached for the generative fallback")
	require.NotNil(t, d.Decider)
	require.True(t, d.CanAnalyze())

	// Without a decider the switch is moot: the LLM decides and explains in one call.
	_, err = s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{VerdictEngineEnabled: &off})
	require.NoError(t, err)
	d2, err := newAnalyzeDepsResolver(s, nil)(models.RunAnalysisJobTriggerManual)
	require.NoError(t, err)
	require.False(t, d2.NarrativeSkipped)
	require.Nil(t, d2.Decider)
}

func TestResolver_EscalationThresholdNeedsBothEngines(t *testing.T) {
	s := resolverStore(t)
	enableTypeSafe(t, s, false)
	d, err := newAnalyzeDepsResolver(s, nil)(models.RunAnalysisJobTriggerManual)
	require.NoError(t, err)
	require.Zero(t, d.EscalateBelow, "off by default")

	pct := 90
	_, err = s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{EscalateBelowPct: &pct})
	require.NoError(t, err)
	d, err = newAnalyzeDepsResolver(s, nil)(models.RunAnalysisJobTriggerManual)
	require.NoError(t, err)
	require.InDelta(t, 0.90, d.EscalateBelow, 1e-9)
	require.InDelta(t, 0.90, d.Analyze().EscalateBelow, 1e-9, "reaches the analyzer")

	off := false
	_, err = s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{VerdictEngineEnabled: &off})
	require.NoError(t, err)
	d, err = newAnalyzeDepsResolver(s, nil)(models.RunAnalysisJobTriggerManual)
	require.NoError(t, err)
	require.Zero(t, d.EscalateBelow, "no TypeSafe decision, nothing to escalate")
}

func approveProviderForAuto(t *testing.T, s *store.Store) {
	t.Helper()
	cfg, err := s.GetDefaultProviderConfig()
	require.NoError(t, err)
	_, err = s.UpdateProviderConfig(cfg.ID, map[string]interface{}{"allow_auto_failure_analysis": true}, "")
	require.NoError(t, err)
}

func TestResolver_AIMasterSwitchStopsEverything(t *testing.T) {
	s := resolverStore(t)
	enableTypeSafe(t, s, false)
	_, err := s.UpdateAIFeatureSettings(false)
	require.NoError(t, err)
	_, err = newAnalyzeDepsResolver(s, nil)(models.RunAnalysisJobTriggerManual)
	require.ErrorIs(t, err, failureanalysis.ErrAIDisabled)
	_, err = newAnalyzeDepsResolver(s, nil)(failureanalysis.TriggerExplain)
	require.ErrorIs(t, err, failureanalysis.ErrAIDisabled)
}

func TestResolver_TypeSafeOnlyNeverTouchesTheLLM(t *testing.T) {
	s := resolverStore(t)
	enableTypeSafe(t, s, false)
	off, zero := false, 0
	_, err := s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{NarrativeEnabled: &off, LLMFallbackEnabled: &off, EscalateBelowPct: &zero})
	require.NoError(t, err)
	// Even a broken LLM configuration cannot block a route that does not use the LLM.
	cfg, err := s.GetDefaultProviderConfig()
	require.NoError(t, err)
	_, err = s.UpdateProviderConfig(cfg.ID, map[string]interface{}{"provider_type": "bogus"}, "")
	require.NoError(t, err)

	d, err := newAnalyzeDepsResolver(s, nil)(models.RunAnalysisJobTriggerManual)
	require.NoError(t, err)
	require.NotNil(t, d.Decider)
	require.Nil(t, d.Narrative, "TypeSafe-only: no LLM is attached at all")
	require.True(t, d.NarrativeSkipped)
	require.True(t, d.NoLLMFallback)
	require.Zero(t, d.EscalateBelow)
	require.True(t, d.CanAnalyze())
	p := d.Pipeline()
	require.Equal(t, "", p.Narrator)
	require.False(t, p.LLMFallback)
	require.Contains(t, p.Label(), "no LLM")

	// An explicit Explain still needs the LLM, and says why it is missing.
	e, err := newAnalyzeDepsResolver(s, nil)(failureanalysis.TriggerExplain)
	require.NoError(t, err)
	require.Nil(t, e.Narrative)
	require.Contains(t, e.LLMUnavailableReason, "misconfigured")
}

func TestResolver_BrokenLLMLeavesTypeSafeDeciding(t *testing.T) {
	s := resolverStore(t)
	enableTypeSafe(t, s, false)
	cfg, err := s.GetDefaultProviderConfig()
	require.NoError(t, err)
	_, err = s.UpdateProviderConfig(cfg.ID, map[string]interface{}{"provider_type": "bogus"}, "")
	require.NoError(t, err)

	d, err := newAnalyzeDepsResolver(s, nil)(models.RunAnalysisJobTriggerManual)
	require.NoError(t, err, "a misconfigured LLM must not fail a job TypeSafe can decide")
	require.NotNil(t, d.Decider)
	require.Nil(t, d.Narrative)
	require.Contains(t, d.LLMUnavailableReason, "misconfigured")

	// Without TypeSafe the LLM has to decide, so the broken configuration is an error.
	off := false
	_, err = s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{VerdictEngineEnabled: &off})
	require.NoError(t, err)
	_, err = newAnalyzeDepsResolver(s, nil)(models.RunAnalysisJobTriggerManual)
	require.Error(t, err)
}

func TestResolver_FallbackSwitch(t *testing.T) {
	s := resolverStore(t)
	enableTypeSafe(t, s, false)
	d, err := newAnalyzeDepsResolver(s, nil)(models.RunAnalysisJobTriggerManual)
	require.NoError(t, err)
	require.False(t, d.NoLLMFallback, "the fallback is on by default")
	off := false
	_, err = s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{LLMFallbackEnabled: &off})
	require.NoError(t, err)
	d, err = newAnalyzeDepsResolver(s, nil)(models.RunAnalysisJobTriggerManual)
	require.NoError(t, err)
	require.True(t, d.NoLLMFallback)
	require.NotNil(t, d.Narrative, "explanations are still on, so the LLM is attached for them")
}

func TestResolver_MissingKeyIsTypeSafeUnavailableNotAnLLMRoute(t *testing.T) {
	s := resolverStore(t)
	enableTypeSafe(t, s, false)
	_, err := s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{ClearAPIKey: true})
	require.NoError(t, err)

	// Fallback on (the default): the LLM may decide, and the analysis says TypeSafe was unavailable.
	d, err := newAnalyzeDepsResolver(s, nil)(models.RunAnalysisJobTriggerManual)
	require.NoError(t, err)
	require.NotNil(t, d.Decider, "TypeSafe stays the configured decider")
	require.NotNil(t, d.Narrative)
	require.False(t, d.NoLLMFallback)
	_, derr := d.Decider.Decide(context.Background(), failureanalysis.Evidence{})
	var te *typesafe.Error
	require.ErrorAs(t, derr, &te)
	require.Equal(t, typesafe.CategoryConfiguration, te.Category)

	// Fallback, explanations and takeover off: no LLM is attached and the attempt fails.
	off, zero := false, 0
	_, err = s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{NarrativeEnabled: &off, LLMFallbackEnabled: &off, EscalateBelowPct: &zero})
	require.NoError(t, err)
	d, err = newAnalyzeDepsResolver(s, nil)(models.RunAnalysisJobTriggerManual)
	require.NoError(t, err)
	require.Nil(t, d.Narrative, "a missing key must not turn a TypeSafe-only route into an LLM route")
	require.True(t, d.NoLLMFallback)
	_, aerr := failureanalysis.Analyze(context.Background(), d.Analyze(),
		failureanalysis.AnalyzeContext{Result: &models.RunResult{ID: "r", ErrorMessage: "boom"}})
	require.ErrorIs(t, aerr, failureanalysis.ErrTypeSafeUnavailable)
	require.Equal(t, "configuration", failureanalysis.FailedResult(aerr, d.Analyze()).ErrorCategory)
}
