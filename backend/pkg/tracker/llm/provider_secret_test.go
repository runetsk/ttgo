package llm

import (
	"context"
	"testing"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

func TestNewProvider_RefusesAnUndecryptableKey(t *testing.T) {
	cfg := &models.LLMProviderConfig{Label: "OpenAI", ProviderType: "openai", ModelName: "m", APIKeyStatus: models.SecretStatusUndecryptable}
	p, err := NewProvider(cfg)
	require.Nil(t, p)
	require.Equal(t, ErrCatConfiguration, Classify(err))
	require.ErrorIs(t, err, models.ErrSecretUndecryptable)
	require.EqualError(t, err, `LLM provider "OpenAI" API key: the stored key can't be decrypted — re-enter it`)
	var pe *ProviderError
	require.ErrorAs(t, err, &pe)
	require.False(t, pe.Retryable(), "retrying cannot fix a stored key")

	// A missing key stays the vendor's to reject, as before.
	cfg.APIKeyStatus = models.SecretStatusMissing
	p, err = NewProvider(cfg)
	require.NoError(t, err)
	require.NotNil(t, p)
}

func TestUnavailableProvider_FailsEveryCallWithoutRetry(t *testing.T) {
	want := &ProviderError{Category: ErrCatConfiguration, Message: "key unreadable"}
	p := NewUnavailableProvider(want)
	_, err := p.Chat(context.Background(), ChatRequest{})
	require.Same(t, want, err)
	_, retries, err := ChatWithRetry(context.Background(), p, ChatRequest{}, RetryOptions{})
	require.Zero(t, retries)
	require.Equal(t, ErrCatConfiguration, Classify(err))
}
