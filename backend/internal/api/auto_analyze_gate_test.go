package api

import (
	"testing"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

// The run-completion gate asks the worker's own resolver whether anything may analyze a
// completed run automatically (spec §7). resolverStore creates a default LLM that is NOT
// approved for automatic analysis.

func TestAutoAnalyzeGate_TypeSafeAloneIsEnough(t *testing.T) {
	s := resolverStore(t)
	enableTypeSafe(t, s, true)
	ok, reason := newAutoAnalyzeGate(newAnalyzeDepsResolver(s))()
	require.True(t, ok, reason)
}

func TestAutoAnalyzeGate_NothingMayAnalyze(t *testing.T) {
	s := resolverStore(t)
	enableTypeSafe(t, s, false) // TypeSafe on, but not for automatic analysis
	ok, reason := newAutoAnalyzeGate(newAnalyzeDepsResolver(s))()
	require.False(t, ok)
	require.Contains(t, reason, "not approved for automatic analysis")
}

func TestAutoAnalyzeGate_SemanticGroupingAloneCannotAnalyze(t *testing.T) {
	s := resolverStore(t)
	enableTypeSafe(t, s, true)
	off := false
	_, err := s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{VerdictEngineEnabled: &off})
	require.NoError(t, err)
	ok, _ := newAutoAnalyzeGate(newAnalyzeDepsResolver(s))()
	require.False(t, ok, "grouping with nobody to decide queues nothing")
}

func TestAutoAnalyzeGate_ApprovedLLMWithoutTypeSafe(t *testing.T) {
	s := resolverStore(t)
	approveProviderForAuto(t, s)
	ok, reason := newAutoAnalyzeGate(newAnalyzeDepsResolver(s))()
	require.True(t, ok, reason)
}

func TestAutoAnalyzeGate_MasterSwitchOff(t *testing.T) {
	s := resolverStore(t)
	approveProviderForAuto(t, s)
	_, err := s.UpdateAIFeatureSettings(false)
	require.NoError(t, err)
	ok, reason := newAutoAnalyzeGate(newAnalyzeDepsResolver(s))()
	require.False(t, ok)
	require.NotEmpty(t, reason)
}

// With verdicts on and no usable key the resolver returns an "unavailable" decider, so a job is
// queued and each group then takes the unavailable path (LLM fallback or a visible failed attempt).
func TestAutoAnalyzeGate_MissingKeyStillQueuesSoTheFailureIsVisible(t *testing.T) {
	s := resolverStore(t)
	enableTypeSafe(t, s, true)
	_, err := s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{ClearAPIKey: true})
	require.NoError(t, err)
	ok, reason := newAutoAnalyzeGate(newAnalyzeDepsResolver(s))()
	require.True(t, ok, reason)
}
