package secretbox

import (
	"bytes"
	"crypto/rand"
	"path/filepath"
	"testing"
)

func newTestBox(t *testing.T) *Box {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	b, err := New(key)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	b := newTestBox(t)
	for _, pt := range []string{"hello", "api-token-12345", "a longer secret with spaces & symbols !@#"} {
		enc, err := b.Encrypt(pt)
		if err != nil {
			t.Fatal(err)
		}
		if !IsEncrypted(enc) {
			t.Errorf("Encrypt(%q) missing sentinel: %q", pt, enc)
		}
		if enc == pt {
			t.Errorf("Encrypt(%q) returned plaintext", pt)
		}
		dec, err := b.Decrypt(enc)
		if err != nil {
			t.Fatal(err)
		}
		if dec != pt {
			t.Errorf("round-trip: got %q, want %q", dec, pt)
		}
	}
}

func TestEncryptIdempotentAndEmpty(t *testing.T) {
	b := newTestBox(t)
	if got, _ := b.Encrypt(""); got != "" {
		t.Errorf("Encrypt(empty) = %q, want empty", got)
	}
	enc, _ := b.Encrypt("x")
	again, _ := b.Encrypt(enc) // already encrypted → unchanged
	if again != enc {
		t.Errorf("Encrypt not idempotent: %q != %q", again, enc)
	}
}

func TestDecryptPlaintextPassThrough(t *testing.T) {
	b := newTestBox(t)
	// Pre-encryption plaintext (no sentinel) must pass through unchanged.
	if got, err := b.Decrypt("legacy-plaintext-token"); err != nil || got != "legacy-plaintext-token" {
		t.Errorf("Decrypt(plaintext) = %q,%v; want unchanged", got, err)
	}
}

func TestSignVerify(t *testing.T) {
	b := newTestBox(t)
	data := []byte("the database bytes")
	sig := b.Sign(data)
	if !b.Verify(data, sig) {
		t.Error("Verify rejected a valid signature")
	}
	if b.Verify([]byte("tampered"), sig) {
		t.Error("Verify accepted a signature for different data")
	}
	if b.Verify(data, "deadbeef") {
		t.Error("Verify accepted a bogus signature")
	}
	// A different key must not verify.
	other := newTestBox(t)
	if other.Verify(data, sig) {
		t.Error("Verify accepted a signature made with a different key")
	}
}

func TestLoadOrCreateGeneratesStableKeyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret.key")
	b1, err := LoadOrCreate("TTGO_TEST_KEY_UNSET", path)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := LoadOrCreate("TTGO_TEST_KEY_UNSET", path) // reload same file
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b1.key, b2.key) {
		t.Error("LoadOrCreate generated a different key on reload")
	}
	enc, _ := b1.Encrypt("secret")
	if dec, err := b2.Decrypt(enc); err != nil || dec != "secret" {
		t.Errorf("reloaded key cannot decrypt: %q,%v", dec, err)
	}
}
