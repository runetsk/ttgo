package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T043: CreateToken stores SHA-256 hash (not plaintext)
func TestCreateTokenStoresHash(t *testing.T) {
	s, err := New(":memory:")
	require.NoError(t, err)

	token, rawToken, err := s.CreateToken("Test CI", "write", nil)
	require.NoError(t, err)
	assert.NotEmpty(t, rawToken, "raw token must be returned")
	assert.NotEmpty(t, token.ID)
	// The raw token must not be stored in DB — only the SHA-256 hash
	var hashInDB string
	s.db.Raw("SELECT token_hash FROM api_tokens WHERE id = ?", token.ID).Scan(&hashInDB)
	assert.NotEqual(t, rawToken, hashInDB, "raw token must not be stored in DB")
	assert.Len(t, hashInDB, 64, "SHA-256 hex digest is 64 chars")
}

// T043: ValidateToken returns record for correct raw value and nil for invalid
func TestValidateToken(t *testing.T) {
	s, err := New(":memory:")
	require.NoError(t, err)

	_, rawToken, err := s.CreateToken("Test", "read", nil)
	require.NoError(t, err)

	found, err := s.ValidateToken(rawToken)
	require.NoError(t, err)
	assert.NotNil(t, found, "valid token should be found")
	assert.Equal(t, "read", found.Scope)

	notFound, err := s.ValidateToken("invalid-token-value")
	require.NoError(t, err)
	assert.Nil(t, notFound, "invalid token should return nil")
}

// T044: DeleteToken hard-deletes; subsequent ValidateToken returns nil
func TestDeleteTokenHardDeletes(t *testing.T) {
	s, err := New(":memory:")
	require.NoError(t, err)

	token, rawToken, err := s.CreateToken("Test", "write", nil)
	require.NoError(t, err)

	err = s.DeleteToken(token.ID)
	require.NoError(t, err)

	notFound, err := s.ValidateToken(rawToken)
	require.NoError(t, err)
	assert.Nil(t, notFound, "deleted token should not be valid")
}

// T048: CreateWebhookConfig rejects non-HTTPS URLs
func TestCreateWebhookConfigRejectsHTTP(t *testing.T) {
	s, err := New(":memory:")
	require.NoError(t, err)

	_, err = s.CreateWebhookConfig("http://example.com/hook", "Test", "run.completed")
	assert.Error(t, err, "HTTP URL should be rejected")
	assert.Contains(t, err.Error(), "HTTPS")

	wh, err := s.CreateWebhookConfig("https://example.com/hook", "Test", "run.completed")
	assert.NoError(t, err)
	assert.NotNil(t, wh)
}
