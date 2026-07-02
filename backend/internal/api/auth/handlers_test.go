package auth_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	api "ttgo/internal/api"
	"ttgo/pkg/tracker/store"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func newTestStore(t *testing.T) (*store.Store, error) {
	t.Helper()
	return store.New(":memory:")
}

// addTestAuth seeds the admin user (test@test.com / testpassword1234) if needed
// and attaches a valid session cookie for that admin to req.
func addTestAuth(t *testing.T, s *store.Store, req *http.Request) {
	t.Helper()
	require.NoError(t, s.SeedAdminIfNeeded("test@test.com", "testpassword1234"))
	user, err := s.FindUserByEmail("test@test.com")
	require.NoError(t, err)
	sess, err := s.CreateSession(user.ID)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: sess.ID})
}

// findSessionCookie returns the session_token cookie set on the response, if any.
func findSessionCookie(w *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range w.Result().Cookies() {
		if c.Name == "session_token" {
			return c
		}
	}
	return nil
}

func loginBody(email, password string) *bytes.Reader {
	b, _ := json.Marshal(map[string]string{"email": email, "password": password})
	return bytes.NewReader(b)
}

// TestLogin_Success asserts the pinned 200 + session_token cookie (HttpOnly) behavior.
func TestLogin_Success(t *testing.T) {
	st, err := newTestStore(t)
	require.NoError(t, err)
	require.NoError(t, st.SeedAdminIfNeeded("test@test.com", "testpassword1234"))
	srv := api.NewServer(st)

	req := httptest.NewRequest("POST", "/api/auth/login", loginBody("test@test.com", "testpassword1234"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		User struct {
			Email string `json:"email"`
		} `json:"user"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "test@test.com", resp.User.Email)

	cookie := findSessionCookie(w)
	require.NotNil(t, cookie, "expected session_token cookie to be set")
	assert.NotEmpty(t, cookie.Value)
	assert.True(t, cookie.HttpOnly, "session_token cookie must be HttpOnly")
}

// TestLogin_WrongPassword asserts the pinned 401, no session cookie behavior
// for both a wrong password and an unknown email.
func TestLogin_WrongPassword(t *testing.T) {
	st, err := newTestStore(t)
	require.NoError(t, err)
	require.NoError(t, st.SeedAdminIfNeeded("test@test.com", "testpassword1234"))
	srv := api.NewServer(st)

	req := httptest.NewRequest("POST", "/api/auth/login", loginBody("test@test.com", "wrongpassword"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "invalid email or password")
	assert.Nil(t, findSessionCookie(w), "no session cookie should be set on failed login")

	// Unknown email should behave identically.
	req2 := httptest.NewRequest("POST", "/api/auth/login", loginBody("nobody@test.com", "whatever123"))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusUnauthorized, w2.Code)
	assert.Contains(t, w2.Body.String(), "invalid email or password")
	assert.Nil(t, findSessionCookie(w2))
}

// TestLogin_DisabledUser asserts the pinned 401 "account is disabled" behavior
// (not 403) for a deactivated user who supplies the CORRECT password.
func TestLogin_DisabledUser(t *testing.T) {
	st, err := newTestStore(t)
	require.NoError(t, err)
	srv := api.NewServer(st)

	hash, err := bcrypt.GenerateFromPassword([]byte("correcthorse123"), 12)
	require.NoError(t, err)
	user, err := st.CreateUser("disabled@test.com", "Disabled User", string(hash), "member")
	require.NoError(t, err)

	_, err = st.UpdateUser(user.ID, map[string]interface{}{"active": false})
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/auth/login", loginBody("disabled@test.com", "correcthorse123"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "account is disabled")
	assert.Nil(t, findSessionCookie(w), "no session cookie should be set for a disabled account")
}

// TestLogin_MalformedBody asserts the pinned 400 for an unparsable request body.
func TestLogin_MalformedBody(t *testing.T) {
	st, err := newTestStore(t)
	require.NoError(t, err)
	srv := api.NewServer(st)

	req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader("{not valid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestLogout_InvalidatesSession logs in, captures the session cookie, logs out
// (pinned 204), and then verifies the SAME cookie no longer authenticates (401)
// against /api/auth/me — proving logout actually deletes the session server-side.
func TestLogout_InvalidatesSession(t *testing.T) {
	st, err := newTestStore(t)
	require.NoError(t, err)
	require.NoError(t, st.SeedAdminIfNeeded("test@test.com", "testpassword1234"))
	srv := api.NewServer(st)

	loginReq := httptest.NewRequest("POST", "/api/auth/login", loginBody("test@test.com", "testpassword1234"))
	loginReq.Header.Set("Content-Type", "application/json")
	loginW := httptest.NewRecorder()
	srv.ServeHTTP(loginW, loginReq)
	require.Equal(t, http.StatusOK, loginW.Code)

	cookie := findSessionCookie(loginW)
	require.NotNil(t, cookie)

	// Sanity check: the session works before logout.
	meReq := httptest.NewRequest("GET", "/api/auth/me", nil)
	meReq.AddCookie(cookie)
	meW := httptest.NewRecorder()
	srv.ServeHTTP(meW, meReq)
	require.Equal(t, http.StatusOK, meW.Code)

	logoutReq := httptest.NewRequest("POST", "/api/auth/logout", nil)
	logoutReq.AddCookie(cookie)
	logoutW := httptest.NewRecorder()
	srv.ServeHTTP(logoutW, logoutReq)
	assert.Equal(t, http.StatusNoContent, logoutW.Code)

	// The same (now-invalidated) cookie must be rejected.
	meReq2 := httptest.NewRequest("GET", "/api/auth/me", nil)
	meReq2.AddCookie(cookie)
	meW2 := httptest.NewRecorder()
	srv.ServeHTTP(meW2, meReq2)
	assert.Equal(t, http.StatusUnauthorized, meW2.Code, "session must be invalidated server-side after logout")
}

// TestMe_ReturnsCurrentUser asserts the pinned 200 body for an authenticated
// request and 401 for an unauthenticated one.
func TestMe_ReturnsCurrentUser(t *testing.T) {
	st, err := newTestStore(t)
	require.NoError(t, err)
	srv := api.NewServer(st)

	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	addTestAuth(t, st, req)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		User struct {
			Email string `json:"email"`
		} `json:"user"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "test@test.com", resp.User.Email)

	// Unauthenticated request (no cookie at all).
	req2 := httptest.NewRequest("GET", "/api/auth/me", nil)
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusUnauthorized, w2.Code)
}

// TestChangePassword_WrongCurrent asserts the pinned 401 "current password is
// incorrect" (not 400), and that the OLD password still works afterward
// (i.e. nothing was mutated on a failed attempt).
func TestChangePassword_WrongCurrent(t *testing.T) {
	st, err := newTestStore(t)
	require.NoError(t, err)
	srv := api.NewServer(st)

	req := httptest.NewRequest("POST", "/api/auth/change-password", bytes.NewReader(mustJSON(map[string]string{
		"current_password": "totallywrongpassword",
		"new_password":     "brandnewpassword123",
	})))
	req.Header.Set("Content-Type", "application/json")
	addTestAuth(t, st, req)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "current password is incorrect")

	// The original admin password must still log in.
	loginReq := httptest.NewRequest("POST", "/api/auth/login", loginBody("test@test.com", "testpassword1234"))
	loginReq.Header.Set("Content-Type", "application/json")
	loginW := httptest.NewRecorder()
	srv.ServeHTTP(loginW, loginReq)
	assert.Equal(t, http.StatusOK, loginW.Code, "old password must still work after a failed change-password attempt")
}

// TestChangePassword_Success asserts the pinned 204 (not 200) on success, and
// verifies real post-mutation state: the new password logs in, the old one
// no longer does, and bad new-password inputs are rejected with 400.
func TestChangePassword_Success(t *testing.T) {
	st, err := newTestStore(t)
	require.NoError(t, err)
	srv := api.NewServer(st)

	req := httptest.NewRequest("POST", "/api/auth/change-password", bytes.NewReader(mustJSON(map[string]string{
		"current_password": "testpassword1234",
		"new_password":     "shinynewpassword456",
	})))
	req.Header.Set("Content-Type", "application/json")
	addTestAuth(t, st, req)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	require.Equal(t, http.StatusNoContent, w.Code)

	// New password logs in.
	newLoginReq := httptest.NewRequest("POST", "/api/auth/login", loginBody("test@test.com", "shinynewpassword456"))
	newLoginReq.Header.Set("Content-Type", "application/json")
	newLoginW := httptest.NewRecorder()
	srv.ServeHTTP(newLoginW, newLoginReq)
	assert.Equal(t, http.StatusOK, newLoginW.Code, "new password must log in after a successful change")

	// Old password no longer logs in.
	oldLoginReq := httptest.NewRequest("POST", "/api/auth/login", loginBody("test@test.com", "testpassword1234"))
	oldLoginReq.Header.Set("Content-Type", "application/json")
	oldLoginW := httptest.NewRecorder()
	srv.ServeHTTP(oldLoginW, oldLoginReq)
	assert.Equal(t, http.StatusUnauthorized, oldLoginW.Code, "old password must be rejected after a successful change")

	// Bad new password (too short) is rejected with 400 — use a fresh session
	// since change-password rotates/invalidates prior sessions.
	newCookie := findSessionCookie(w)
	require.NotNil(t, newCookie, "change-password should issue a fresh session cookie")

	badReq := httptest.NewRequest("POST", "/api/auth/change-password", bytes.NewReader(mustJSON(map[string]string{
		"current_password": "shinynewpassword456",
		"new_password":     "short",
	})))
	badReq.Header.Set("Content-Type", "application/json")
	badReq.AddCookie(newCookie)
	badW := httptest.NewRecorder()
	srv.ServeHTTP(badW, badReq)
	assert.Equal(t, http.StatusBadRequest, badW.Code, "new password under 8 chars must be rejected")
}

func mustJSON(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
