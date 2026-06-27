package users_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	api "ttgo/internal/api"
	"ttgo/pkg/tracker/store"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) (*store.Store, error) {
	t.Helper()
	return store.New(":memory:")
}

func addTestAuth(t *testing.T, s *store.Store, req *http.Request) {
	t.Helper()
	require.NoError(t, s.SeedAdminIfNeeded("test@test.com", "testpassword1234"))
	user, err := s.FindUserByEmail("test@test.com")
	require.NoError(t, err)
	sess, err := s.CreateSession(user.ID)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: sess.ID})
}

func TestDeleteUser_Success(t *testing.T) {
	st, err := newTestStore(t)
	require.NoError(t, err)
	srv := api.NewServer(st)

	target, err := st.CreateUser("target@test.com", "Target", "$2a$12$dummyhashvalue1234567890abcdefghijklmnopqrs", "member")
	require.NoError(t, err)

	req := httptest.NewRequest("DELETE", "/api/users/"+target.ID, nil)
	addTestAuth(t, st, req)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	deleted, err := st.GetUser(target.ID)
	require.NoError(t, err)
	assert.True(t, deleted.Deleted)
	assert.False(t, deleted.Active)
}

func TestDeleteUser_CannotDeleteSelf(t *testing.T) {
	st, err := newTestStore(t)
	require.NoError(t, err)
	srv := api.NewServer(st)

	// addTestAuth seeds test@test.com as admin
	req := httptest.NewRequest("DELETE", "/api/users/placeholder", nil)
	addTestAuth(t, st, req)

	admin, err := st.FindUserByEmail("test@test.com")
	require.NoError(t, err)

	req = httptest.NewRequest("DELETE", "/api/users/"+admin.ID, nil)
	addTestAuth(t, st, req)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "cannot delete yourself")
}

func TestDeleteUser_CannotDeleteLastActiveAdmin(t *testing.T) {
	st, err := newTestStore(t)
	require.NoError(t, err)
	srv := api.NewServer(st)

	// Seed admin and get session cookie
	err = st.SeedAdminIfNeeded("test@test.com", "testpassword1234")
	require.NoError(t, err)
	admin, err := st.FindUserByEmail("test@test.com")
	require.NoError(t, err)
	sess, err := st.CreateSession(admin.ID)
	require.NoError(t, err)
	cookie := &http.Cookie{Name: "session_token", Value: sess.ID}

	// Create another admin
	otherAdmin, err := st.CreateUser("other-admin@test.com", "Other Admin", "$2a$12$dummyhashvalue1234567890abcdefghijklmnopqrs", "admin")
	require.NoError(t, err)

	// Delete other admin — should succeed (2 admins → 1 left)
	req := httptest.NewRequest("DELETE", "/api/users/"+otherAdmin.ID, nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Create a member and try to delete — should succeed (not an admin)
	member, err := st.CreateUser("member@test.com", "Member", "$2a$12$dummyhashvalue1234567890abcdefghijklmnopqrs", "member")
	require.NoError(t, err)

	req2 := httptest.NewRequest("DELETE", "/api/users/"+member.ID, nil)
	req2.AddCookie(cookie)
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestDeleteUser_AlreadyDeleted(t *testing.T) {
	st, err := newTestStore(t)
	require.NoError(t, err)
	srv := api.NewServer(st)

	target, err := st.CreateUser("target@test.com", "Target", "$2a$12$dummyhashvalue1234567890abcdefghijklmnopqrs", "member")
	require.NoError(t, err)
	_, err = st.UpdateUser(target.ID, map[string]interface{}{"deleted": true, "active": false})
	require.NoError(t, err)

	req := httptest.NewRequest("DELETE", "/api/users/"+target.ID, nil)
	addTestAuth(t, st, req)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "already deleted")
}

func TestRestoreUser_Success(t *testing.T) {
	st, err := newTestStore(t)
	require.NoError(t, err)
	srv := api.NewServer(st)

	target, err := st.CreateUser("target@test.com", "Target", "$2a$12$dummyhashvalue1234567890abcdefghijklmnopqrs", "member")
	require.NoError(t, err)
	_, err = st.UpdateUser(target.ID, map[string]interface{}{"deleted": true, "active": false})
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/users/"+target.ID+"/restore", nil)
	addTestAuth(t, st, req)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	restored, err := st.GetUser(target.ID)
	require.NoError(t, err)
	assert.False(t, restored.Deleted)
	assert.False(t, restored.Active) // must be reactivated separately
}

func TestRestoreUser_NotDeleted(t *testing.T) {
	st, err := newTestStore(t)
	require.NoError(t, err)
	srv := api.NewServer(st)

	target, err := st.CreateUser("target@test.com", "Target", "$2a$12$dummyhashvalue1234567890abcdefghijklmnopqrs", "member")
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/users/"+target.ID+"/restore", nil)
	addTestAuth(t, st, req)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "not deleted")
}
