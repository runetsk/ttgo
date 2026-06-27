package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListUsers_ExcludesDeleted(t *testing.T) {
	s := newTestStore(t)

	_, err := s.CreateUser("active@example.com", "Active", "hashed", "member")
	require.NoError(t, err)

	deleted, err := s.CreateUser("deleted@example.com", "Deleted", "hashed", "member")
	require.NoError(t, err)
	_, err = s.UpdateUser(deleted.ID, map[string]interface{}{"deleted": true, "active": false})
	require.NoError(t, err)

	users, err := s.ListUsers(false)
	require.NoError(t, err)
	assert.Len(t, users, 1)
	assert.Equal(t, "active@example.com", users[0].Email)
}

func TestListUsers_IncludesDeleted(t *testing.T) {
	s := newTestStore(t)

	_, err := s.CreateUser("active@example.com", "Active", "hashed", "member")
	require.NoError(t, err)

	deleted, err := s.CreateUser("deleted@example.com", "Deleted", "hashed", "member")
	require.NoError(t, err)
	_, err = s.UpdateUser(deleted.ID, map[string]interface{}{"deleted": true, "active": false})
	require.NoError(t, err)

	users, err := s.ListUsers(true)
	require.NoError(t, err)
	assert.Len(t, users, 2)
}

// TestFindUserByEmail_CaseInsensitive verifies that FindUserByEmail is case-insensitive.
func TestFindUserByEmail_CaseInsensitive(t *testing.T) {
	s := newTestStore(t)

	_, err := s.CreateUser("UPPER@EXAMPLE.COM", "Upper User", "hashed", "member")
	require.NoError(t, err)

	// Look up with lowercase version
	found, err := s.FindUserByEmail("upper@example.com")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "upper@example.com", found.Email)
}
