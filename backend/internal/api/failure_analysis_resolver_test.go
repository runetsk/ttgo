package api

import (
	"context"
	"testing"
	"time"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/secretbox"
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
	require.Equal(t, "configuration", failureanalysis.FailedResult(aerr, d.Analyze(), failureanalysis.AnalyzeContext{}).ErrorCategory)
}
func TestResolver_UndecryptableLLMKey(t *testing.T) {
	s := resolverStore(t)
	cfg, err := s.GetDefaultProviderConfig()
	require.NoError(t, err)
	box, err := secretbox.New([]byte("0123456789abcdef0123456789abcdef"))
	require.NoError(t, err)
	foreign, err := box.Encrypt("k")
	require.NoError(t, err)
	require.NoError(t, s.DB().Exec("UPDATE llm_provider_configs SET api_key = ? WHERE id = ?", foreign, cfg.ID).Error)
	resolve := newAnalyzeDepsResolver(s, typesafe.NewClientFactory(nil))

	// The LLM has to decide (TypeSafe off): the job runs and every attempt is recorded as failed
	// with category configuration, so the card says what to fix.
	d, err := resolve(models.RunAnalysisJobTriggerManual)
	require.NoError(t, err)
	require.NotNil(t, d.Narrative)
	_, cerr := d.Narrative.Chat(context.Background(), llm.ChatRequest{})
	require.ErrorIs(t, cerr, models.ErrSecretUndecryptable)
	require.Equal(t, "configuration", failureanalysis.FailedResult(cerr, d.Analyze(), failureanalysis.AnalyzeContext{}).ErrorCategory)

	// Explain never gets a provider that can only fail: it says why instead.
	e, err := resolve(failureanalysis.TriggerExplain)
	require.NoError(t, err)
	require.Nil(t, e.Narrative)
	require.Equal(t, "the default LLM provider's stored key can't be decrypted — re-enter it", e.LLMUnavailableReason)

	// An automatic run with nothing else to decide cannot run at all; non-secret edits still work.
	approveProviderForAuto(t, s)
	_, err = resolve(models.RunAnalysisJobTriggerAutoOnDone)
	require.ErrorIs(t, err, models.ErrSecretUndecryptable)

	// With TypeSafe deciding, the LLM is simply left out.
	enableTypeSafe(t, s, false)
	d, err = resolve(models.RunAnalysisJobTriggerManual)
	require.NoError(t, err)
	require.NotNil(t, d.Decider)
	require.Nil(t, d.Narrative)
	require.Equal(t, "the default LLM provider's stored key can't be decrypted — re-enter it", d.LLMUnavailableReason)
}
func TestResolver_CapturesThePricesInForce(t *testing.T) {
	t.Chdir(t.TempDir())
	s, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	in, out := 1.5, 6.0
	cfg := &models.LLMProviderConfig{Label: "priced", ProviderType: "openai", APIKey: "k", ModelName: "gpt-x", Enabled: true,
		PromptPricePerMTok: &in, CompletionPricePerMTok: &out}
	require.NoError(t, s.CreateProviderConfig(cfg))
	require.NoError(t, s.SetDefaultProviderConfig(cfg.ID))
	enableTypeSafe(t, s, false)
	price := 0.2
	_, err = s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{PricePerMTok: &price})
	require.NoError(t, err)

	d, err := newAnalyzeDepsResolver(s, typesafe.NewClientFactory(nil))(models.RunAnalysisJobTriggerManual)
	require.NoError(t, err)
	require.Equal(t, cfg.ID, d.Pricing.LLMProviderID)
	require.InDelta(t, 1.5, *d.Pricing.LLMPromptPerMTok, 1e-12)
	require.InDelta(t, 6.0, *d.Pricing.LLMCompletionPerMTok, 1e-12)
	require.InDelta(t, 0.2, d.Pricing.TypeSafePerMTok, 1e-12)
}

func TestResolver_BoundsEveryLLMCall(t *testing.T) {
	s := resolverStore(t)
	r := newAnalyzeDepsResolver(s, nil)

	d, err := r(models.RunAnalysisJobTriggerManual)
	require.NoError(t, err)
	require.NotNil(t, d.Narrative)
	require.Equal(t, 45*time.Second, d.LLMCallTimeout, "the default call timeout")
	require.False(t, d.HedgingOn)
	require.Zero(t, d.TypeSafeTimeout, "TypeSafe does not decide here")

	cur, err := s.GetFailureAnalysisSettings()
	require.NoError(t, err)
	cur.LLMCallTimeoutSeconds, cur.HedgeAfterSeconds = 60, 10
	_, err = s.UpdateFailureAnalysisSettings(cur)
	require.NoError(t, err)
	enableTypeSafe(t, s, false)

	for _, trigger := range []string{models.RunAnalysisJobTriggerManual, failureanalysis.TriggerExplain} {
		d, err = r(trigger)
		require.NoError(t, err)
		require.NotNil(t, d.Narrative, trigger)
		require.Equal(t, 60*time.Second, d.LLMCallTimeout, trigger)
		require.True(t, d.HedgingOn, trigger)
	}
	require.Equal(t, 30*time.Second, d.TypeSafeTimeout, "TypeSafe decides with its 30 s default")

	// A row from before the settings (or a corrupt value) falls back to the defaults.
	require.NoError(t, s.DB().Model(&models.AIFailureAnalysisSettings{}).Where("id = ?", "singleton").
		Updates(map[string]interface{}{"llm_call_timeout_seconds": 0, "hedge_after_seconds": 0}).Error)
	d, err = r(models.RunAnalysisJobTriggerManual)
	require.NoError(t, err)
	require.Equal(t, 45*time.Second, d.LLMCallTimeout)
	require.False(t, d.HedgingOn)
}

// The wrappers pass other errors through untouched: the undecryptable-key provider still fails
// with the key error, now inside the call timeout.
func TestResolver_WrappedUnavailableProviderKeepsItsError(t *testing.T) {
	s := resolverStore(t)
	cfg, err := s.GetDefaultProviderConfig()
	require.NoError(t, err)
	box, err := secretbox.New([]byte("0123456789abcdef0123456789abcdef"))
	require.NoError(t, err)
	foreign, err := box.Encrypt("k")
	require.NoError(t, err)
	require.NoError(t, s.DB().Exec("UPDATE llm_provider_configs SET api_key = ? WHERE id = ?", foreign, cfg.ID).Error)

	d, err := newAnalyzeDepsResolver(s, typesafe.NewClientFactory(nil))(models.RunAnalysisJobTriggerManual)
	require.NoError(t, err)
	require.Equal(t, 45*time.Second, d.LLMCallTimeout)
	_, cerr := d.Narrative.Chat(context.Background(), llm.ChatRequest{})
	require.ErrorIs(t, cerr, models.ErrSecretUndecryptable)
}

func TestResolver_CarriesFewShotExamples(t *testing.T) {
	s := resolverStore(t)
	r := newAnalyzeDepsResolver(s, nil)
	d, err := r(models.RunAnalysisJobTriggerManual)
	require.NoError(t, err)
	require.Equal(t, models.DefaultFewShotExamples, d.FewShotExamples, "a new install sends four examples")

	cur, err := s.GetFailureAnalysisSettings()
	require.NoError(t, err)
	cur.FewShotExamples = 0
	_, err = s.UpdateFailureAnalysisSettings(cur)
	require.NoError(t, err)
	d, err = r(models.RunAnalysisJobTriggerManual)
	require.NoError(t, err)
	require.Zero(t, d.FewShotExamples, "read live, like every other setting")
}

func TestResolver_TransferCheckFollowsSemanticGroupingAndDedup(t *testing.T) {
	s := resolverStore(t)
	enableTypeSafe(t, s, false)
	r := newAnalyzeDepsResolver(s, nil)

	d, err := r(models.RunAnalysisJobTriggerManual)
	require.NoError(t, err)
	require.NotNil(t, d.Transfer)
	require.NotNil(t, d.Semantic)
	require.Equal(t, d.Semantic.Client, d.Transfer.Client, "the same client, so the same rate limiter")
	require.Equal(t, d.Semantic.Model, d.Transfer.Model)

	explain, err := r(failureanalysis.TriggerExplain)
	require.NoError(t, err)
	require.NotNil(t, explain.Transfer, "Explain on a group runs the check too")

	auto, err := r(models.RunAnalysisJobTriggerAutoOnDone)
	require.NoError(t, err)
	require.Nil(t, auto.Transfer, "automatic jobs need the per-vendor consent flag")

	cur, err := s.GetFailureAnalysisSettings()
	require.NoError(t, err)
	cur.DedupEnabled = false
	_, err = s.UpdateFailureAnalysisSettings(cur)
	require.NoError(t, err)
	d, err = r(models.RunAnalysisJobTriggerManual)
	require.NoError(t, err)
	require.Nil(t, d.Transfer, "no dedup, no clones to check")

	cur.DedupEnabled = true
	_, err = s.UpdateFailureAnalysisSettings(cur)
	require.NoError(t, err)
	off := false
	_, err = s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{SemanticDedupEnabled: &off})
	require.NoError(t, err)
	d, err = r(models.RunAnalysisJobTriggerManual)
	require.NoError(t, err)
	require.Nil(t, d.Semantic)
	require.Nil(t, d.Transfer, "only semantic grouping creates the clones the check is about")

	_, err = s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{SemanticDedupEnabled: new(bool), ClearAPIKey: true})
	require.NoError(t, err)
	d, err = r(models.RunAnalysisJobTriggerManual)
	require.NoError(t, err)
	require.Nil(t, d.Transfer, "no key, no check")
}
