package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAIFeatureSettings_DefaultsToEnabled(t *testing.T) {
	s := newTestStore(t)
	got, err := s.GetOrCreateAIFeatureSettings()
	require.NoError(t, err)
	require.True(t, got.Enabled, "AI features must default to enabled")
}

func TestAIFeatureSettings_GetOrCreateIsIdempotent(t *testing.T) {
	s := newTestStore(t)
	a, err := s.GetOrCreateAIFeatureSettings()
	require.NoError(t, err)
	b, err := s.GetOrCreateAIFeatureSettings()
	require.NoError(t, err)
	// Same row (not recreated) and stable state. We compare ID + Enabled rather
	// than the whole struct because timestamps lose sub-second precision on the
	// SQLite round-trip, which would make a full-struct compare spuriously fail.
	require.Equal(t, a.ID, b.ID, "GetOrCreate must return the same singleton row, not recreate it")
	require.True(t, b.Enabled, "second get must preserve the default enabled state")
}

func TestAIFeatureSettings_UpdatePersistsFalse(t *testing.T) {
	s := newTestStore(t)
	updated, err := s.UpdateAIFeatureSettings(false)
	require.NoError(t, err)
	require.False(t, updated.Enabled)

	got, err := s.GetOrCreateAIFeatureSettings()
	require.NoError(t, err)
	require.False(t, got.Enabled, "disabled state must persist")
}
