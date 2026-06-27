// Package secretbox provides authenticated encryption (AES-256-GCM) for secrets
// stored at rest, plus HMAC-SHA256 signing for backup integrity. Encrypted values
// carry an "enc:v1:" sentinel so plaintext (pre-encryption) values pass through
// transparently during the one-time backfill migration.
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const sentinel = "enc:v1:"

// Box holds the AEAD cipher and the raw key (used for HMAC).
type Box struct {
	gcm cipher.AEAD
	key []byte
}

// New builds a Box from a 32-byte key.
func New(key []byte) (*Box, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("secretbox: key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{gcm: gcm, key: append([]byte(nil), key...)}, nil
}

// LoadOrCreate resolves the encryption key from the environment variable envVar
// (hex, base64, or a raw 32-char string) or from keyPath (a generated 0600 file
// created beside the database if absent). The key intentionally lives outside the
// DB so it survives a restore.
func LoadOrCreate(envVar, keyPath string) (*Box, error) {
	if v := strings.TrimSpace(os.Getenv(envVar)); v != "" {
		key, err := decodeKey(v)
		if err != nil {
			return nil, fmt.Errorf("secretbox: %s: %w", envVar, err)
		}
		return New(key)
	}
	if data, err := os.ReadFile(keyPath); err == nil {
		key, err := decodeKey(strings.TrimSpace(string(data)))
		if err != nil {
			return nil, fmt.Errorf("secretbox: key file %s: %w", keyPath, err)
		}
		return New(key)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(keyPath, []byte(hex.EncodeToString(key)), 0o600); err != nil {
		return nil, err
	}
	return New(key)
}

func decodeKey(s string) ([]byte, error) {
	if b, err := hex.DecodeString(s); err == nil && len(b) == 32 {
		return b, nil
	}
	if b, err := base64.StdEncoding.DecodeString(s); err == nil && len(b) == 32 {
		return b, nil
	}
	if len(s) == 32 {
		return []byte(s), nil
	}
	return nil, errors.New("key must decode to 32 bytes (hex, base64, or a 32-char string)")
}

// IsEncrypted reports whether s carries the encryption sentinel.
func IsEncrypted(s string) bool { return strings.HasPrefix(s, sentinel) }

// Encrypt returns "enc:v1:<base64(nonce|ciphertext)>". Empty strings and
// already-encrypted values are returned unchanged (idempotent).
func (b *Box) Encrypt(plaintext string) (string, error) {
	if plaintext == "" || IsEncrypted(plaintext) {
		return plaintext, nil
	}
	nonce := make([]byte, b.gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ct := b.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return sentinel + base64.StdEncoding.EncodeToString(ct), nil
}

// Decrypt reverses Encrypt. Values without the sentinel are returned unchanged
// (backward-compatible with pre-encryption plaintext).
func (b *Box) Decrypt(stored string) (string, error) {
	if !IsEncrypted(stored) {
		return stored, nil
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, sentinel))
	if err != nil {
		return "", err
	}
	ns := b.gcm.NonceSize()
	if len(raw) < ns {
		return "", errors.New("secretbox: ciphertext too short")
	}
	pt, err := b.gcm.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", fmt.Errorf("secretbox: decrypt failed (wrong key?): %w", err)
	}
	return string(pt), nil
}

// Sign returns a hex HMAC-SHA256 over data, domain-separated for backups.
func (b *Box) Sign(data []byte) string {
	mac := hmac.New(sha256.New, b.key)
	_, _ = mac.Write([]byte("ttgo-backup-v1"))
	_, _ = mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify reports whether sig is a valid signature for data (constant-time).
func (b *Box) Verify(data []byte, sig string) bool {
	want, err := hex.DecodeString(sig)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, b.key)
	_, _ = mac.Write([]byte("ttgo-backup-v1"))
	_, _ = mac.Write(data)
	return hmac.Equal(mac.Sum(nil), want)
}
