package store

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
	"ttgo/pkg/tracker/models"

	"github.com/google/uuid"
)

// GenerateToken creates a new API token, hashes it, stores the hash, and returns the raw token.
// The raw token is returned only once and is never stored.
func (s *Store) CreateToken(description, scope string, expiresAt *time.Time) (*models.ApiToken, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, "", fmt.Errorf("failed to generate token: %w", err)
	}
	rawToken := base64.StdEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(hash[:])

	token := &models.ApiToken{
		ID:          uuid.New().String(),
		TokenHash:   tokenHash,
		Description: description,
		Scope:       scope,
		ExpiresAt:   expiresAt,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := s.db.Create(token).Error; err != nil {
		return nil, "", err
	}
	return token, rawToken, nil
}

// ListTokens returns all API tokens (without hashes).
func (s *Store) ListTokens() ([]models.ApiToken, error) {
	var tokens []models.ApiToken
	if err := s.db.Order("created_at DESC").Find(&tokens).Error; err != nil {
		return nil, err
	}
	return tokens, nil
}

// DeleteToken hard-deletes a token by ID.
func (s *Store) DeleteToken(id string) error {
	return s.db.Delete(&models.ApiToken{}, "id = ?", id).Error
}

// ValidateToken checks a raw Bearer token value and returns the matching ApiToken or nil.
// It also updates last_used_at on success.
func (s *Store) ValidateToken(rawToken string) (*models.ApiToken, error) {
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(hash[:])

	var token models.ApiToken
	err := s.db.Where("token_hash = ?", tokenHash).First(&token).Error
	if err != nil {
		return nil, nil //nolint:nilerr
	}

	// Check expiry
	if token.ExpiresAt != nil && token.ExpiresAt.Before(time.Now()) {
		return nil, nil
	}

	now := time.Now()
	s.db.Model(&token).Update("last_used_at", now) //nolint:errcheck
	return &token, nil
}
