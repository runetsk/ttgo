package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"ttgo/internal/logging"
)

// dummy handler to use as next in middleware tests
func okHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func TestRequireAuth_ValidBearerToken_Write(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := NewServer(s)

	// Create a write-scoped API token via store
	_, rawTok, err := s.CreateToken("test", "write", nil)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/tests", nil)
	req.Header.Set("Authorization", "Bearer "+rawTok)
	rr := httptest.NewRecorder()
	srv.requireAuth("write", okHandler)(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRequireAuth_ValidSessionCookie(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := NewServer(s)

	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), 12)
	user, err := s.CreateUser("cookie@example.com", "Cookie User", string(hash), "member")
	require.NoError(t, err)
	session, err := s.CreateSession(user.ID)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/tests", nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: session.ID})
	rr := httptest.NewRecorder()
	srv.requireAuth("write", okHandler)(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRequireAuth_InvalidBearerToken_401(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := NewServer(s)

	req := httptest.NewRequest(http.MethodPost, "/api/tests", nil)
	req.Header.Set("Authorization", "Bearer invalid-token-xyz")
	rr := httptest.NewRecorder()
	srv.requireAuth("write", okHandler)(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRequireAuth_InvalidSessionCookie_401(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := NewServer(s)

	req := httptest.NewRequest(http.MethodPost, "/api/tests", nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: "expired-or-unknown"})
	rr := httptest.NewRecorder()
	srv.requireAuth("write", okHandler)(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRequireAuth_NoCredentials_401(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := NewServer(s)

	req := httptest.NewRequest(http.MethodPost, "/api/tests", nil)
	rr := httptest.NewRecorder()
	srv.requireAuth("write", okHandler)(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRequireAuth_ReadTokenOnWriteRoute_403(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := NewServer(s)

	_, rawTok, err := s.CreateToken("readonly", "read", nil)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/tests", nil)
	req.Header.Set("Authorization", "Bearer "+rawTok)
	rr := httptest.NewRecorder()
	srv.requireAuth("write", okHandler)(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestRequireAdmin_AdminRole_OK(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := NewServer(s)

	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), 12)
	admin, err := s.CreateUser("admin@example.com", "Admin User", string(hash), "admin")
	require.NoError(t, err)
	session, err := s.CreateSession(admin.ID)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: session.ID})
	rr := httptest.NewRecorder()
	srv.requireAdmin(okHandler)(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRequireAdmin_MemberRole_403(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := NewServer(s)

	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), 12)
	member, err := s.CreateUser("member@example.com", "Member User", string(hash), "member")
	require.NoError(t, err)
	session, err := s.CreateSession(member.ID)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: session.ID})
	rr := httptest.NewRecorder()
	srv.requireAdmin(okHandler)(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestRecoveryMiddleware_CatchesPanic(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := NewServer(s)

	panickingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})
	wrapped := srv.recoveryMiddleware(panickingHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "internal server error")
}

func TestRecoveryMiddleware_NoPanic(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := NewServer(s)

	wrapped := srv.recoveryMiddleware(http.HandlerFunc(okHandler))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRequestIDMiddleware_PreservesIncomingHeader(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := NewServer(s)

	const requestID = "req-123"
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, requestID, logging.RequestID(r.Context()))
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", requestID)
	rr := httptest.NewRecorder()
	srv.requestIDMiddleware(next).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.Equal(t, requestID, rr.Header().Get("X-Request-ID"))
}

func TestRequestIDMiddleware_GeneratesRequestID(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := NewServer(s)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := logging.RequestID(r.Context())
		assert.NotEmpty(t, reqID)
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	srv.requestIDMiddleware(next).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.NotEmpty(t, rr.Header().Get("X-Request-ID"))
}

func TestRequireAdmin_BearerToken_403(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := NewServer(s)

	_, rawTok, err := s.CreateToken("api-token", "write", nil)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.Header.Set("Authorization", "Bearer "+rawTok)
	rr := httptest.NewRecorder()
	srv.requireAdmin(okHandler)(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}
