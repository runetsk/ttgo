package jira_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	api "ttgo/internal/api"
	"ttgo/pkg/tracker/secretbox"
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

// plantUndecryptableToken saves a working config, then swaps the token for a ciphertext made
// under another key (a lost secret.key or a DB restored from another instance).
func plantUndecryptableToken(t *testing.T, st *store.Store) string {
	t.Helper()
	_, err := st.UpsertJiraConfig("https://example.atlassian.net", "u@e.com", "token-1234", true, "PROJ", "Bug")
	require.NoError(t, err)
	box, err := secretbox.New([]byte("0123456789abcdef0123456789abcdef"))
	require.NoError(t, err)
	foreign, err := box.Encrypt("token-1234")
	require.NoError(t, err)
	require.NoError(t, st.DB().Exec("UPDATE jira_configs SET api_token = ? WHERE id = ?", foreign, "singleton").Error)
	return foreign
}

func storedToken(t *testing.T, st *store.Store) string {
	t.Helper()
	var v string
	require.NoError(t, st.DB().Raw("SELECT api_token FROM jira_configs WHERE id = ?", "singleton").Scan(&v).Error)
	return v
}

func TestUndecryptableToken_SettingsStillLoadAndCallsSay422(t *testing.T) {
	st := newStore(t)
	foreign := plantUndecryptableToken(t, st)

	w := do(t, st, "GET", "/api/settings/jira", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.NotContains(t, w.Body.String(), foreign)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "undecryptable", got["api_token_status"])
	assert.Equal(t, "", got["api_token_masked"])

	for _, call := range []struct {
		method, path string
		body         interface{}
	}{
		{"GET", "/api/jira/ticket/PROJ-1", nil},
		{"POST", "/api/jira/search", map[string]interface{}{"jql": "project = PROJ"}},
	} {
		w = do(t, st, call.method, call.path, call.body)
		require.Equal(t, http.StatusUnprocessableEntity, w.Code, call.path+": "+w.Body.String())
		assert.Contains(t, w.Body.String(), "Jira API token: the stored key can't be decrypted — re-enter it")
		assert.Contains(t, w.Body.String(), `"category":"configuration"`)
	}
}

func TestUndecryptableToken_AdminCanEditClearAndReenter(t *testing.T) {
	st := newStore(t)
	foreign := plantUndecryptableToken(t, st)
	base := map[string]interface{}{"base_url": "https://example.atlassian.net", "email": "new@e.com", "enabled": false, "default_project_key": "NEW"}

	// Other fields save and the stored token is kept byte-for-byte.
	w := do(t, st, "PUT", "/api/settings/jira", base)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"api_token_status":"undecryptable"`)
	assert.Contains(t, w.Body.String(), `"email":"new@e.com"`)
	assert.Equal(t, foreign, storedToken(t, st))

	// A new token and clear_api_token together are refused.
	both := map[string]interface{}{"base_url": base["base_url"], "email": base["email"], "api_token": "t", "clear_api_token": true}
	w = do(t, st, "PUT", "/api/settings/jira", both)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	clear := map[string]interface{}{"base_url": base["base_url"], "email": base["email"], "clear_api_token": true}
	w = do(t, st, "PUT", "/api/settings/jira", clear)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"api_token_status":"missing"`)
	assert.Equal(t, "", storedToken(t, st))

	reenter := map[string]interface{}{"base_url": base["base_url"], "email": base["email"], "api_token": "fresh-9876", "enabled": true}
	w = do(t, st, "PUT", "/api/settings/jira", reenter)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"api_token_status":"ok"`)
	assert.Contains(t, w.Body.String(), `"api_token_masked":"****9876"`)
}
