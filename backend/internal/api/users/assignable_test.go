package users_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	api "ttgo/internal/api"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const dummyHash = "$2a$12$dummyhashvalue1234567890abcdefghijklmnopqrs"

func TestListAssignableReturnsActiveUsers(t *testing.T) {
	st, err := newTestStore(t)
	require.NoError(t, err)
	srv := api.NewServer(st)

	alice, err := st.CreateUser("alice@test.com", "Alice", dummyHash, "member")
	require.NoError(t, err)
	gone, err := st.CreateUser("gone@test.com", "Gone", dummyHash, "member")
	require.NoError(t, err)

	// Soft-delete "gone" via the admin API (addTestAuth seeds an admin session).
	del := httptest.NewRequest(http.MethodDelete, "/api/users/"+gone.ID, nil)
	addTestAuth(t, st, del)
	dw := httptest.NewRecorder()
	srv.ServeHTTP(dw, del)
	require.Equal(t, http.StatusOK, dw.Code, dw.Body.String())

	// Call /users/assignable as a NON-admin member (proves it is not admin-gated).
	sess, err := st.CreateSession(alice.ID)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodGet, "/api/users/assignable", nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: sess.ID})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	body := w.Body.String()
	assert.Contains(t, body, "alice@test.com")
	assert.NotContains(t, body, "gone@test.com", "deleted users must be excluded")
	assert.NotContains(t, body, `"role"`, "minimal DTO must not leak role")
	assert.NotContains(t, body, `"deleted"`, "minimal DTO must not leak deleted")
}
