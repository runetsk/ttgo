package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestRecoverUserAccess pins the break-glass semantics: new hash applied,
// account re-enabled and undeleted, and every session revoked, atomically.
func TestRecoverUserAccess(t *testing.T) {
	s, err := New(":memory:")
	require.NoError(t, err)

	u, err := s.CreateUser("locked@example.com", "Locked Out", "oldhash", "admin")
	require.NoError(t, err)

	// Simulate the worst case: deactivated AND soft-deleted, with a live session.
	_, err = s.UpdateUser(u.ID, map[string]interface{}{"active": false, "deleted": true})
	require.NoError(t, err)
	sess, err := s.CreateSession(u.ID)
	require.NoError(t, err)

	recovered, err := s.RecoverUserAccess(u.ID, "newhash")
	require.NoError(t, err)
	assert.Equal(t, "newhash", recovered.HashedPassword)
	assert.True(t, recovered.Active)
	assert.False(t, recovered.Deleted)

	// The pre-existing session must be gone.
	got, err := s.ValidateSession(sess.ID)
	require.NoError(t, err)
	assert.Nil(t, got, "sessions must be revoked on recovery")
}

// TestRecoverUserAccess_UnknownID pins the not-found contract.
func TestRecoverUserAccess_UnknownID(t *testing.T) {
	s, err := New(":memory:")
	require.NoError(t, err)

	_, err = s.RecoverUserAccess("no-such-id", "hash")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}
