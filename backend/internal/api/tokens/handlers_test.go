package tokens_test

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
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(":memory:")
	require.NoError(t, err)
	return s
}

func addAuth(t *testing.T, s *store.Store, req *http.Request) {
	t.Helper()
	require.NoError(t, s.SeedAdminIfNeeded("admin@test.com", "testpassword1234"))
	user, err := s.FindUserByEmail("admin@test.com")
	require.NoError(t, err)
	sess, err := s.CreateSession(user.ID)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: sess.ID})
}

func doReq(t *testing.T, st *store.Store, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		r = httptest.NewRequest(method, path, bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	addAuth(t, st, r)
	w := httptest.NewRecorder()
	api.NewServer(st).ServeHTTP(w, r)
	return w
}

func TestCreateToken_Success(t *testing.T) {
	st := newTestStore(t)
	w := doReq(t, st, "POST", "/api/tokens", map[string]string{
		"description": "ci token",
		"scope":       "write",
	})
	require.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp["id"])
	assert.NotEmpty(t, resp["token"])
	assert.Equal(t, "ci token", resp["description"])
	assert.Equal(t, "write", resp["scope"])
}

func TestCreateToken_ReadScope(t *testing.T) {
	st := newTestStore(t)
	w := doReq(t, st, "POST", "/api/tokens", map[string]string{
		"description": "ro",
		"scope":       "read",
	})
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCreateToken_InvalidScope(t *testing.T) {
	st := newTestStore(t)
	w := doReq(t, st, "POST", "/api/tokens", map[string]string{
		"description": "bad",
		"scope":       "admin",
	})
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "scope")
}

func TestCreateToken_MissingDescription(t *testing.T) {
	st := newTestStore(t)
	w := doReq(t, st, "POST", "/api/tokens", map[string]string{
		"scope": "read",
	})
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "description")
}

func TestCreateToken_BadJSON(t *testing.T) {
	st := newTestStore(t)
	r := httptest.NewRequest("POST", "/api/tokens", strings.NewReader("{not json"))
	r.Header.Set("Content-Type", "application/json")
	addAuth(t, st, r)
	w := httptest.NewRecorder()
	api.NewServer(st).ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListTokens_Empty(t *testing.T) {
	st := newTestStore(t)
	w := doReq(t, st, "GET", "/api/tokens", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(0), resp["total"])
}

func TestListTokens_WithItems(t *testing.T) {
	st := newTestStore(t)
	_, _, err := st.CreateToken("t1", "read", nil)
	require.NoError(t, err)
	_, _, err = st.CreateToken("t2", "write", nil)
	require.NoError(t, err)

	w := doReq(t, st, "GET", "/api/tokens", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(2), resp["total"])
	arr, ok := resp["tokens"].([]interface{})
	require.True(t, ok)
	assert.Len(t, arr, 2)
}

func TestDeleteToken_Success(t *testing.T) {
	st := newTestStore(t)
	tok, _, err := st.CreateToken("kill me", "read", nil)
	require.NoError(t, err)

	w := doReq(t, st, "DELETE", "/api/tokens/"+tok.ID, nil)
	assert.Equal(t, http.StatusNoContent, w.Code)

	tokens, err := st.ListTokens()
	require.NoError(t, err)
	assert.Len(t, tokens, 0)
}

func TestDeleteToken_Unknown(t *testing.T) {
	st := newTestStore(t)
	// Deleting unknown token succeeds at store level (GORM soft/no-op).
	w := doReq(t, st, "DELETE", "/api/tokens/nonexistent", nil)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestTokens_RequireAuth(t *testing.T) {
	st := newTestStore(t)
	// No auth cookie.
	r := httptest.NewRequest("GET", "/api/tokens", nil)
	w := httptest.NewRecorder()
	api.NewServer(st).ServeHTTP(w, r)
	assert.NotEqual(t, http.StatusOK, w.Code)
}
