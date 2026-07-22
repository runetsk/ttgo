package main

import (
	"bytes"
	"path/filepath"
	"regexp"
	"testing"
	"ttgo/pkg/tracker/store"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

var tempPasswordRe = regexp.MustCompile(`Temporary password: (\S+)`)

// TestRunResetPassword_HappyPath drives the command end to end against a real
// database file: the printed temporary password must actually verify against
// the stored hash, and existing sessions must be revoked.
func TestRunResetPassword_HappyPath(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := store.New(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() }) // Windows: TempDir removal needs the file closed
	require.NoError(t, s.SeedAdminIfNeeded("boss@example.com", "originalpass1"))

	u, err := s.FindUserByEmail("boss@example.com")
	require.NoError(t, err)
	require.NotNil(t, u)
	sess, err := s.CreateSession(u.ID)
	require.NoError(t, err)

	t.Setenv("DB_PATH", dbPath)
	var out, errOut bytes.Buffer
	code := runResetPassword([]string{"Boss@Example.com"}, &out, &errOut)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())

	m := tempPasswordRe.FindStringSubmatch(out.String())
	require.Len(t, m, 2, "output must contain the temporary password: %s", out.String())
	temp := m[1]

	fresh, err := s.FindUserByEmail("boss@example.com")
	require.NoError(t, err)
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(fresh.HashedPassword), []byte(temp)),
		"printed temporary password must match the stored hash")
	assert.Error(t, bcrypt.CompareHashAndPassword([]byte(fresh.HashedPassword), []byte("originalpass1")),
		"old password must no longer work")

	got, err := s.ValidateSession(sess.ID)
	require.NoError(t, err)
	assert.Nil(t, got, "existing sessions must be revoked")
}

// TestRunResetPassword_RecoversDisabledAccount pins that a deactivated account
// is re-enabled (with a notice) — resetting the password alone would still
// leave the owner locked out.
func TestRunResetPassword_RecoversDisabledAccount(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := store.New(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() }) // Windows: TempDir removal needs the file closed
	require.NoError(t, s.SeedAdminIfNeeded("boss@example.com", "originalpass1"))
	u, err := s.FindUserByEmail("boss@example.com")
	require.NoError(t, err)
	_, err = s.UpdateUser(u.ID, map[string]interface{}{"active": false})
	require.NoError(t, err)

	t.Setenv("DB_PATH", dbPath)
	var out, errOut bytes.Buffer
	code := runResetPassword([]string{"boss@example.com"}, &out, &errOut)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Contains(t, out.String(), "re-enabled")

	fresh, err := s.FindUserByEmail("boss@example.com")
	require.NoError(t, err)
	assert.True(t, fresh.Active)
}

// TestRunResetPassword_UnknownEmailListsAdmins pins the friendly failure: the
// error names the admin accounts so a "which email did I register?" operator
// isn't stuck guessing.
func TestRunResetPassword_UnknownEmailListsAdmins(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := store.New(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() }) // Windows: TempDir removal needs the file closed
	require.NoError(t, s.SeedAdminIfNeeded("boss@example.com", "originalpass1"))

	t.Setenv("DB_PATH", dbPath)
	var out, errOut bytes.Buffer
	code := runResetPassword([]string{"typo@example.com"}, &out, &errOut)
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut.String(), "No user with email")
	assert.Contains(t, errOut.String(), "boss@example.com")
}

// TestRunResetPassword_MissingDB pins that a wrong working directory fails
// with guidance instead of silently creating an empty database.
func TestRunResetPassword_MissingDB(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.db")
	t.Setenv("DB_PATH", missing)

	var out, errOut bytes.Buffer
	code := runResetPassword([]string{"boss@example.com"}, &out, &errOut)
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut.String(), "Database not found")
	assert.NoFileExists(t, missing, "the command must never create a database")
}

// TestRunResetPassword_Usage pins arg handling: help exits 0, bad arity exits 2.
func TestRunResetPassword_Usage(t *testing.T) {
	var out, errOut bytes.Buffer
	assert.Equal(t, 0, runResetPassword([]string{"--help"}, &out, &errOut))
	assert.Contains(t, out.String(), "Usage:")

	out.Reset()
	errOut.Reset()
	assert.Equal(t, 2, runResetPassword(nil, &out, &errOut))
	assert.Contains(t, errOut.String(), "Usage:")
}
