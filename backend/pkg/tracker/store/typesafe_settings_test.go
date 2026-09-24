package store

import (
	"testing"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/secretbox"

	"github.com/stretchr/testify/require"
)

// Prefixed: the store test package already defines strPtr in run_folders_test.go:26.
func tsStr(s string) *string { return &s }
func tsBool(b bool) *bool    { return &b }

func TestTypeSafeSettings_SeededDefaults(t *testing.T) {
	s := newTestStore(t)
	got, err := s.GetTypeSafeSettings()
	require.NoError(t, err)
	require.False(t, got.Enabled)
	require.Equal(t, models.TypeSafeDefaultModel, got.Model)
	require.Equal(t, 30, got.TimeoutSeconds)
	require.True(t, got.VerdictEngineEnabled)
	require.True(t, got.NarrativeEnabled)
	require.True(t, got.SemanticDedupEnabled)
	require.False(t, got.AllowAutoFailureAnalysis)
	resp, err := s.TypeSafeSettingsResponse()
	require.NoError(t, err)
	require.Equal(t, models.TypeSafeKeyStatusMissing, resp.APIKeyStatus)
	require.Equal(t, "", resp.APIKeyMasked)
	require.True(t, resp.NarrativeEnabled)
}

func TestTypeSafeSettings_NarrativeSwitchRoundTrips(t *testing.T) {
	s := newTestStore(t)
	got, err := s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{NarrativeEnabled: tsBool(false)})
	require.NoError(t, err)
	require.False(t, got.NarrativeEnabled)
	resp, err := s.TypeSafeSettingsResponse()
	require.NoError(t, err)
	require.False(t, resp.NarrativeEnabled)
	got, err = s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{Enabled: tsBool(true)})
	require.NoError(t, err)
	require.False(t, got.NarrativeEnabled, "a patch that omits the field leaves it alone")
}

func TestTypeSafeSettings_EscalationThresholdDefaultsOffAndRoundTrips(t *testing.T) {
	s := newTestStore(t)
	got, err := s.GetTypeSafeSettings()
	require.NoError(t, err)
	require.Equal(t, 0, got.EscalateBelowPct, "never escalate unless an admin opts in")
	pct := 90
	got, err = s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{EscalateBelowPct: &pct})
	require.NoError(t, err)
	require.Equal(t, 90, got.EscalateBelowPct)
	resp, err := s.TypeSafeSettingsResponse()
	require.NoError(t, err)
	require.Equal(t, 90, resp.EscalateBelowPct)
}

func TestTypeSafeSettings_KeyEncryptedAndMasked(t *testing.T) {
	s := newTestStore(t)
	_, err := s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{APIKey: tsStr("ts-secret-key-9876"), Enabled: tsBool(true)})
	require.NoError(t, err)

	row, err := s.GetTypeSafeSettings()
	require.NoError(t, err)
	require.True(t, secretbox.IsEncrypted(row.APIKey), "key must be stored encrypted")

	key, err := s.TypeSafeAPIKey()
	require.NoError(t, err)
	require.Equal(t, "ts-secret-key-9876", key)

	resp, err := s.TypeSafeSettingsResponse()
	require.NoError(t, err)
	require.Equal(t, models.TypeSafeKeyStatusOK, resp.APIKeyStatus)
	require.Equal(t, "…9876", resp.APIKeyMasked)
	require.True(t, resp.Enabled)
}

func TestTypeSafeSettings_BlankKeyPreservesAndClearFlagClears(t *testing.T) {
	s := newTestStore(t)
	_, err := s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{APIKey: tsStr("ts-secret-key-9876")})
	require.NoError(t, err)

	_, err = s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{APIKey: tsStr(""), Model: tsStr("jev-latest")})
	require.NoError(t, err)
	key, err := s.TypeSafeAPIKey()
	require.NoError(t, err)
	require.Equal(t, "ts-secret-key-9876", key, "blank key must preserve the stored key")
	row, _ := s.GetTypeSafeSettings()
	require.Equal(t, "jev-latest", row.Model)

	_, err = s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{ClearAPIKey: true})
	require.NoError(t, err)
	resp, _ := s.TypeSafeSettingsResponse()
	require.Equal(t, models.TypeSafeKeyStatusMissing, resp.APIKeyStatus)

	_, err = s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{APIKey: tsStr("x"), ClearAPIKey: true})
	require.ErrorIs(t, err, ErrKeyAndClearExclusive)
}

func TestTypeSafeSettings_UndecryptableKeyIsReported(t *testing.T) {
	s := newTestStore(t)
	// A ciphertext produced under a different key: strict decryption must fail, not pass through.
	other, err := secretbox.New([]byte("0123456789abcdef0123456789abcdef"))
	require.NoError(t, err)
	foreign, err := other.Encrypt("ts-secret")
	require.NoError(t, err)
	require.NoError(t, s.db.Model(&models.TypeSafeSettings{}).Where("id = ?", models.TypeSafeSettingsID).
		Update("api_key", foreign).Error)

	_, err = s.TypeSafeAPIKey()
	require.Error(t, err)
	resp, err := s.TypeSafeSettingsResponse()
	require.NoError(t, err)
	require.Equal(t, models.TypeSafeKeyStatusUndecryptable, resp.APIKeyStatus)
}

func TestTypeSafeSettings_EncryptionFailureRejectsWrite(t *testing.T) {
	s := newTestStore(t)
	saved := s.box
	s.box = nil // simulate an unavailable box: strict helper must refuse, not store plaintext
	defer func() { s.box = saved }()
	_, err := s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{APIKey: tsStr("ts-secret")})
	require.Error(t, err)
	row, getErr := s.GetTypeSafeSettings()
	require.NoError(t, getErr)
	require.Equal(t, "", row.APIKey)
}

func TestTypeSafeSettings_PlaintextKeyIsRejectedUntilBackfilled(t *testing.T) {
	s := newTestStore(t)
	// A plaintext value can only get here from a manual DB edit; strict read must not accept it.
	require.NoError(t, s.db.Model(&models.TypeSafeSettings{}).Where("id = ?", models.TypeSafeSettingsID).
		Update("api_key", "plain-text-key-1234").Error)
	_, err := s.TypeSafeAPIKey()
	require.ErrorIs(t, err, ErrSecretNotEncrypted)
	resp, err := s.TypeSafeSettingsResponse()
	require.NoError(t, err)
	require.Equal(t, models.TypeSafeKeyStatusUndecryptable, resp.APIKeyStatus)

	// The boot-time backfill encrypts it, after which the strict read succeeds.
	require.NoError(t, s.backfillEncryptSecrets())
	key, err := s.TypeSafeAPIKey()
	require.NoError(t, err)
	require.Equal(t, "plain-text-key-1234", key)
}
