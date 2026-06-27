package confluence_test

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

func newStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(":memory:")
	require.NoError(t, err)
	return s
}

func auth(t *testing.T, s *store.Store, r *http.Request) {
	t.Helper()
	require.NoError(t, s.SeedAdminIfNeeded("admin@test.com", "testpassword1234"))
	user, err := s.FindUserByEmail("admin@test.com")
	require.NoError(t, err)
	sess, err := s.CreateSession(user.ID)
	require.NoError(t, err)
	r.AddCookie(&http.Cookie{Name: "session_token", Value: sess.ID})
}

func do(t *testing.T, st *store.Store, method, path string, body interface{}) *httptest.ResponseRecorder {
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
	auth(t, st, r)
	w := httptest.NewRecorder()
	api.NewServer(st).ServeHTTP(w, r)
	return w
}

func TestGetConfig_NotConfigured(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "GET", "/api/settings/confluence", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetConfig_Configured(t *testing.T) {
	st := newStore(t)
	_, err := st.UpsertConfluenceConfig("https://example.atlassian.net", "user@example.com", "token", true)
	require.NoError(t, err)
	w := do(t, st, "GET", "/api/settings/confluence", nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpsertConfig_Success(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "PUT", "/api/settings/confluence", map[string]interface{}{
		"base_url":  "https://example.atlassian.net",
		"email":     "user@example.com",
		"api_token": "secret",
		"enabled":   true,
	})
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpsertConfig_BadJSON(t *testing.T) {
	st := newStore(t)
	r := httptest.NewRequest("PUT", "/api/settings/confluence", strings.NewReader("{bad"))
	r.Header.Set("Content-Type", "application/json")
	auth(t, st, r)
	w := httptest.NewRecorder()
	api.NewServer(st).ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpsertConfig_MissingBaseURL(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "PUT", "/api/settings/confluence", map[string]interface{}{
		"email": "u@e.com",
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpsertConfig_MissingEmail(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "PUT", "/api/settings/confluence", map[string]interface{}{
		"base_url": "https://x",
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListSpaces_NotConfigured(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "GET", "/api/confluence/spaces", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListPages_NotConfigured(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "GET", "/api/confluence/pages?space_id=x", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListPages_MissingSpaceID(t *testing.T) {
	st := newStore(t)
	_, err := st.UpsertConfluenceConfig("https://example.atlassian.net", "u@e.com", "token", true)
	require.NoError(t, err)
	w := do(t, st, "GET", "/api/confluence/pages", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetPage_NotConfigured(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "GET", "/api/confluence/pages/abc", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListChildPages_NotConfigured(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "GET", "/api/confluence/pages/abc/children", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
