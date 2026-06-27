package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateSession_Valid verifies that ValidateSession returns the User for a valid session.
func TestValidateSession_Valid(t *testing.T) {
	s := newTestStore(t)

	user, err := s.CreateUser("session@example.com", "Session User", "hashed", "member")
	require.NoError(t, err)

	session, err := s.CreateSession(user.ID)
	require.NoError(t, err)

	got, err := s.ValidateSession(session.ID)
	require.NoError(t, err)
	require.NotNil(t, got, "should return the user for a valid session")
	assert.Equal(t, user.ID, got.ID)
}

// TestValidateSession_Expired verifies that ValidateSession returns nil for expired sessions.
func TestValidateSession_Expired(t *testing.T) {
	s := newTestStore(t)

	user, err := s.CreateUser("expired@example.com", "Expired User", "hashed", "member")
	require.NoError(t, err)

	session, err := s.CreateSession(user.ID)
	require.NoError(t, err)

	// Manually expire the session
	s.db.Model(session).Update("expires_at", time.Now().Add(-time.Hour))

	got, err := s.ValidateSession(session.ID)
	require.NoError(t, err)
	assert.Nil(t, got, "expired session should return nil")
}

// TestValidateSession_InactiveUser verifies that ValidateSession returns nil for deactivated users.
func TestValidateSession_InactiveUser(t *testing.T) {
	s := newTestStore(t)

	user, err := s.CreateUser("inactive@example.com", "Inactive User", "hashed", "member")
	require.NoError(t, err)

	session, err := s.CreateSession(user.ID)
	require.NoError(t, err)

	// Deactivate the user
	_, err = s.UpdateUser(user.ID, map[string]interface{}{"active": false})
	require.NoError(t, err)

	got, err := s.ValidateSession(session.ID)
	require.NoError(t, err)
	assert.Nil(t, got, "session for inactive user should return nil")
}

// TestValidateSession_UnknownToken verifies that ValidateSession returns nil for unknown tokens.
func TestValidateSession_UnknownToken(t *testing.T) {
	s := newTestStore(t)

	got, err := s.ValidateSession("nonexistent-token-xxxxx")
	require.NoError(t, err)
	assert.Nil(t, got, "unknown token should return nil")
}

// TestDeleteSession verifies that DeleteSession removes the session record.
func TestDeleteSession(t *testing.T) {
	s := newTestStore(t)

	user, err := s.CreateUser("delsession@example.com", "Del Session", "hashed", "member")
	require.NoError(t, err)

	session, err := s.CreateSession(user.ID)
	require.NoError(t, err)

	require.NoError(t, s.DeleteSession(session.ID))

	got, err := s.ValidateSession(session.ID)
	require.NoError(t, err)
	assert.Nil(t, got, "deleted session should not be valid")
}

// TestDeleteUserSessions verifies that DeleteUserSessions removes all sessions for a user.
func TestDeleteUserSessions(t *testing.T) {
	s := newTestStore(t)

	user, err := s.CreateUser("multisess@example.com", "Multi Session", "hashed", "member")
	require.NoError(t, err)

	sess1, err := s.CreateSession(user.ID)
	require.NoError(t, err)
	sess2, err := s.CreateSession(user.ID)
	require.NoError(t, err)

	require.NoError(t, s.DeleteUserSessions(user.ID))

	got1, _ := s.ValidateSession(sess1.ID)
	got2, _ := s.ValidateSession(sess2.ID)
	assert.Nil(t, got1, "all user sessions should be deleted")
	assert.Nil(t, got2, "all user sessions should be deleted")
}
