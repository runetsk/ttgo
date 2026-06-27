package store

import (
	"testing"
	"ttgo/pkg/tracker/secretbox"

	"github.com/stretchr/testify/require"
)

// TestJiraConfigTokenEncryptedAtRest verifies F-016: the API token is stored as
// ciphertext but returned decrypted from the store API.
func TestJiraConfigTokenEncryptedAtRest(t *testing.T) {
	s := newTestStore(t)

	const secret = "super-secret-jira-token-123"
	if _, err := s.UpsertJiraConfig("https://x.atlassian.net", "a@b.c", secret, true, "PROJ", "Bug"); err != nil {
		t.Fatal(err)
	}

	// The store API returns the decrypted token.
	cfg, err := s.GetJiraConfig()
	require.NoError(t, err)
	require.Equal(t, secret, cfg.APIToken)

	// The raw DB column must NOT contain the plaintext.
	var raw string
	require.NoError(t, s.DB().Raw("SELECT api_token FROM jira_configs WHERE id = ?", jiraConfigSingletonID).Scan(&raw).Error)
	require.True(t, secretbox.IsEncrypted(raw), "stored token must be encrypted, got %q", raw)
	require.NotContains(t, raw, secret)
}

// TestBackupSignVerify verifies F-017: a signature validates only for the exact
// bytes and only with this store's key.
func TestBackupSignVerify(t *testing.T) {
	s := newTestStore(t)
	data := []byte("pretend database bytes")
	sig, err := s.box.Encrypt("noop") // ensure box is usable
	require.NoError(t, err)
	_ = sig

	signature := s.box.Sign(data)
	require.True(t, s.VerifyFileBytes(data, signature))
	require.False(t, s.VerifyFileBytes([]byte("tampered"), signature))
	require.False(t, s.VerifyFileBytes(data, "deadbeef"))
}
