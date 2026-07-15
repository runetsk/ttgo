package store

import (
	"os"
	"path/filepath"
	"testing"
	"ttgo/pkg/tracker/secretbox"

	"github.com/stretchr/testify/require"
)

// TestEncryptionKeyStoredBesideDatabase verifies the at-rest encryption key file
// is created in the DATABASE's directory (the persisted volume in the Docker
// deployment), not in the process CWD. If it lands in CWD it is lost on container
// recreation, a new key is generated, and every stored secret (LLM/integration
// API keys) becomes undecryptable — the "401 Missing Authentication header after
// rebuild" bug.
func TestEncryptionKeyStoredBesideDatabase(t *testing.T) {
	t.Setenv("TTGO_ENCRYPTION_KEY", "") // force the file-based key, not an env-provided one
	dir := t.TempDir()
	s, err := New(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() {
		if sqlDB, err := s.db.DB(); err == nil {
			_ = sqlDB.Close() // close before TempDir removal (Windows can't delete an open SQLite file)
		}
	})

	_, statErr := os.Stat(filepath.Join(dir, "secret.key"))
	require.NoError(t, statErr, "secret.key must be created beside the database, not in CWD")
}

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
