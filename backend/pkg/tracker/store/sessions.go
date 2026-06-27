package store

import (
	"crypto/rand"
	"encoding/hex"
	"time"
	"ttgo/pkg/tracker/models"

	"gorm.io/gorm"
)

// CreateSession generates a new session for the given user.
// The session ID is 32 cryptographically-random bytes encoded as a 64-char hex string.
// ExpiresAt is set to now + 24 hours.
func (s *Store) CreateSession(userID string) (*models.UserSession, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(raw)

	now := time.Now()
	sess := &models.UserSession{
		ID:         token,
		UserID:     userID,
		CreatedAt:  now,
		LastSeenAt: now,
		ExpiresAt:  now.Add(24 * time.Hour),
	}
	if err := s.db.Create(sess).Error; err != nil {
		return nil, err
	}
	return sess, nil
}

// ValidateSession looks up a session by token and returns the owning User if
// the session is valid (not expired and user is active). Returns nil (no error)
// for any invalid/expired/unknown token.
func (s *Store) ValidateSession(token string) (*models.User, error) {
	var sess models.UserSession
	err := s.db.Preload("User").Where("id = ? AND expires_at > ?", token, time.Now()).First(&sess).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !sess.User.Active {
		return nil, nil
	}

	// Update LastSeenAt
	s.db.Model(&sess).Update("last_seen_at", time.Now()) //nolint:errcheck

	return &sess.User, nil
}

// DeleteSession removes a single session record (explicit logout).
func (s *Store) DeleteSession(token string) error {
	return s.db.Delete(&models.UserSession{}, "id = ?", token).Error
}

// DeleteUserSessions removes all sessions belonging to a user (e.g. on password change).
func (s *Store) DeleteUserSessions(userID string) error {
	return s.db.Delete(&models.UserSession{}, "user_id = ?", userID).Error
}

// DeleteExpiredSessions purges sessions whose expiry time is in the past.
func (s *Store) DeleteExpiredSessions() error {
	return s.db.Delete(&models.UserSession{}, "expires_at <= ?", time.Now()).Error
}
