package jira_test

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
	w := do(t, st, "GET", "/api/settings/jira", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "false")
}

func TestGetConfig_Configured(t *testing.T) {
	st := newStore(t)
	_, err := st.UpsertJiraConfig("https://example.atlassian.net", "u@e.com", "token", true, "PROJ", "Bug")
	require.NoError(t, err)
	w := do(t, st, "GET", "/api/settings/jira", nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpsertConfig_Success(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "PUT", "/api/settings/jira", map[string]interface{}{
		"base_url":  "https://example.atlassian.net",
		"email":     "u@e.com",
		"api_token": "secret",
		"enabled":   true,
	})
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpsertConfig_BadJSON(t *testing.T) {
	st := newStore(t)
	r := httptest.NewRequest("PUT", "/api/settings/jira", strings.NewReader("{bad"))
	r.Header.Set("Content-Type", "application/json")
	auth(t, st, r)
	w := httptest.NewRecorder()
	api.NewServer(st).ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpsertConfig_MissingBaseURL(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "PUT", "/api/settings/jira", map[string]interface{}{"email": "u@e.com"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpsertConfig_MissingEmail(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "PUT", "/api/settings/jira", map[string]interface{}{"base_url": "https://x"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetTicket_NotConfigured(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "GET", "/api/jira/ticket/PROJ-1", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetTicket_ConfiguredNotEnabled(t *testing.T) {
	st := newStore(t)
	_, err := st.UpsertJiraConfig("https://example.atlassian.net", "u@e.com", "token", false, "", "")
	require.NoError(t, err)
	w := do(t, st, "GET", "/api/jira/ticket/PROJ-1", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSearch_BadJSON(t *testing.T) {
	st := newStore(t)
	r := httptest.NewRequest("POST", "/api/jira/search", strings.NewReader("{bad"))
	r.Header.Set("Content-Type", "application/json")
	auth(t, st, r)
	w := httptest.NewRecorder()
	api.NewServer(st).ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSearch_MissingJQL(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "POST", "/api/jira/search", map[string]interface{}{})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSearch_NotConfigured(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "POST", "/api/jira/search", map[string]interface{}{"jql": "project = X"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
