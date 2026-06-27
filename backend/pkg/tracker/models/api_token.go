package models

import "time"

// ApiToken represents a CI/CD API authentication token.
// The raw token value is never stored; only a SHA-256 hex digest is persisted.
type ApiToken struct {
	ID          string     `json:"id" gorm:"primaryKey"`
	TokenHash   string     `json:"-" gorm:"uniqueIndex:idx_api_tokens_hash;not null"`
	Description string     `json:"description"`
	Scope       string     `json:"scope"` // "read" or "write"
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
