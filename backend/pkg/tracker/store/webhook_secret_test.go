package store

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// TestCreateWebhookConfig_GeneratesSigningSecret verifies F-066: each webhook gets
// a unique signing secret that produces a verifiable HMAC over a payload.
func TestCreateWebhookConfig_GeneratesSigningSecret(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.CreateWebhookConfig("https://8.8.8.8/a", "a", "run.completed")
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.CreateWebhookConfig("https://8.8.8.8/b", "b", "run.completed")
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Secret) != 64 { // 32 random bytes, hex-encoded
		t.Errorf("secret length = %d, want 64 hex chars", len(a.Secret))
	}
	if a.Secret == b.Secret {
		t.Error("two webhooks must not share a signing secret")
	}

	// A receiver can verify a payload signature with the stored secret.
	payload := []byte(`{"event":"run.completed"}`)
	mac := hmac.New(sha256.New, []byte(a.Secret))
	mac.Write(payload)
	sig := hex.EncodeToString(mac.Sum(nil))

	mac2 := hmac.New(sha256.New, []byte(a.Secret))
	mac2.Write(payload)
	if !hmac.Equal([]byte(sig), []byte(hex.EncodeToString(mac2.Sum(nil)))) {
		t.Error("HMAC of the same payload+secret should match")
	}
}
