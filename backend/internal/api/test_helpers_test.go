package api

import (
	"net/http"
	"testing"
	"ttgo/pkg/tracker/store"

	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) (*store.Store, error) {
	t.Helper()
	return store.New(":memory:")
}

// addTestAuth creates an admin user + session and adds the session cookie to the request.
func addTestAuth(t *testing.T, s *store.Store, req *http.Request) {
	t.Helper()
	// Seed an admin user for test auth
	err := s.SeedAdminIfNeeded("test@test.com", "testpassword1234")
	require.NoError(t, err)
	user, err := s.FindUserByEmail("test@test.com")
	require.NoError(t, err)
	sess, err := s.CreateSession(user.ID)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: sess.ID})
}
