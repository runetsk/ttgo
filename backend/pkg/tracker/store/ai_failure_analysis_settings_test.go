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
