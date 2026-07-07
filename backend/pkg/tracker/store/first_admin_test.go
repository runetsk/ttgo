package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCreateFirstAdmin_EmptyDB pins the bootstrap shape: normalised email,
// role admin, active, DisplayName "Admin".
func TestCreateFirstAdmin_EmptyDB(t *testing.T) {
	s, err := New(":memory:")
	require.NoError(t, err)

	u, err := s.CreateFirstAdmin("Boss@Example.com ", "fakehash")
	require.NoError(t, err)
	assert.Equal(t, "boss@example.com", u.Email)
	assert.Equal(t, "admin", u.Role)
	assert.True(t, u.Active)
	assert.Equal(t, "Admin", u.DisplayName)

	count, err := s.CountUsers()
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)
}

// TestCreateFirstAdmin_RefusesWhenUsersExist pins the zero-users gate:
// once ANY user exists, first-run setup is permanently closed.
func TestCreateFirstAdmin_RefusesWhenUsersExist(t *testing.T) {
	s, err := New(":memory:")
	require.NoError(t, err)
	require.NoError(t, s.SeedAdminIfNeeded("a@b.com", "somepassword1"))

	_, err = s.CreateFirstAdmin("x@y.com", "fakehash")
	require.ErrorIs(t, err, ErrSetupComplete)

	count, err := s.CountUsers()
	require.NoError(t, err)
	assert.EqualValues(t, 1, count, "the refused attempt must not create a row")
}

// TestSeedAdminIfNeeded_NoEnvNoUsers_IsNotFatal pins the startup change:
// missing ADMIN_* env vars on an empty DB is a notice, not an error,
// so the browser first-run setup flow can bootstrap the instance.
func TestSeedAdminIfNeeded_NoEnvNoUsers_IsNotFatal(t *testing.T) {
	s, err := New(":memory:")
	require.NoError(t, err)

	require.NoError(t, s.SeedAdminIfNeeded("", ""))

	count, err := s.CountUsers()
	require.NoError(t, err)
	assert.EqualValues(t, 0, count, "nothing should be seeded without env credentials")
}
