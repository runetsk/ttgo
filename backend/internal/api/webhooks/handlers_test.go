package webhooks_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	api "ttgo/internal/api"
	"ttgo/pkg/tracker/store"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestStore returns a fresh in-memory store for a single test.
func newTestStore(t *testing.T) (*store.Store, error) {
	t.Helper()
	return store.New(":memory:")
}

// addTestAuth seeds an admin user and attaches a valid session cookie to req.
func addTestAuth(t *testing.T, s *store.Store, req *http.Request) {
	t.Helper()
	require.NoError(t, s.SeedAdminIfNeeded("test@test.com", "testpassword1234"))
	user, err := s.FindUserByEmail("test@test.com")
	require.NoError(t, err)
	sess, err := s.CreateSession(user.ID)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: sess.ID})
}

func TestRotateWebhookSecret(t *testing.T) {
	st, err := newTestStore(t)
	require.NoError(t, err)
	srv := api.NewServer(st)

	wh, err := st.CreateWebhookConfig("https://example.com/hook", "", "run.completed")
	require.NoError(t, err)
	oldSecret := wh.Secret

	req := httptest.NewRequest("POST", "/api/webhooks/"+wh.ID+"/rotate-secret", nil)
	addTestAuth(t, st, req)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Secret string `json:"secret"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Secret)
	assert.NotEqual(t, oldSecret, resp.Secret)
}

// TestCreateWebhook_ReturnsSecretOnce verifies the signing secret is returned in the
// Create response body but never leaks back out through List (the model's Secret field
// is json:"-", so it must be shown exactly once, at creation) (F-066).
func TestCreateWebhook_ReturnsSecretOnce(t *testing.T) {
	st, err := newTestStore(t)
	require.NoError(t, err)
	srv := api.NewServer(st)

	body := `{"url":"https://example.com/hook","description":"test hook","event_type":"run.completed"}`
	req := httptest.NewRequest("POST", "/api/webhooks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	addTestAuth(t, st, req)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	var created struct {
		ID     string `json:"id"`
		Secret string `json:"secret"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	assert.NotEmpty(t, created.Secret)
	assert.NotEmpty(t, created.ID)

	// Now list webhooks and confirm the secret string never appears in the response body.
	listReq := httptest.NewRequest("GET", "/api/webhooks", nil)
	addTestAuth(t, st, listReq)
	listW := httptest.NewRecorder()
	srv.ServeHTTP(listW, listReq)

	require.Equal(t, http.StatusOK, listW.Code)
	listBody := listW.Body.String()
	assert.NotContains(t, listBody, created.Secret, "webhook secret must not leak through the list endpoint")
}

// TestCreateWebhook_RequiresURL verifies an empty body (missing url) is rejected with 400.
func TestCreateWebhook_RequiresURL(t *testing.T) {
	st, err := newTestStore(t)
	require.NoError(t, err)
	srv := api.NewServer(st)

	req := httptest.NewRequest("POST", "/api/webhooks", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	addTestAuth(t, st, req)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestListWebhooks verifies the list endpoint returns the correct total after creating
// multiple webhooks.
func TestListWebhooks(t *testing.T) {
	st, err := newTestStore(t)
	require.NoError(t, err)
	srv := api.NewServer(st)

	_, err = st.CreateWebhookConfig("https://example.com/hook-1", "first", "run.completed")
	require.NoError(t, err)
	_, err = st.CreateWebhookConfig("https://example.com/hook-2", "second", "run.completed")
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/webhooks", nil)
	addTestAuth(t, st, req)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Webhooks []map[string]interface{} `json:"webhooks"`
		Total    int                      `json:"total"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 2, resp.Total)
	assert.Len(t, resp.Webhooks, 2)
}

// TestDeleteWebhook verifies a created webhook can be deleted and no longer appears in
// the list afterward.
func TestDeleteWebhook(t *testing.T) {
	st, err := newTestStore(t)
	require.NoError(t, err)
	srv := api.NewServer(st)

	wh, err := st.CreateWebhookConfig("https://example.com/hook", "", "run.completed")
	require.NoError(t, err)

	delReq := httptest.NewRequest("DELETE", "/api/webhooks/"+wh.ID, nil)
	addTestAuth(t, st, delReq)
	delW := httptest.NewRecorder()
	srv.ServeHTTP(delW, delReq)

	require.Equal(t, http.StatusNoContent, delW.Code)

	listReq := httptest.NewRequest("GET", "/api/webhooks", nil)
	addTestAuth(t, st, listReq)
	listW := httptest.NewRecorder()
	srv.ServeHTTP(listW, listReq)

	require.Equal(t, http.StatusOK, listW.Code)
	var resp struct {
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(listW.Body.Bytes(), &resp))
	assert.Equal(t, 0, resp.Total)
}

// TestWebhooks_RequireAuth verifies unauthenticated requests (no session cookie) are
// rejected with 401, proving the webhook routes are auth-gated.
func TestWebhooks_RequireAuth(t *testing.T) {
	st, err := newTestStore(t)
	require.NoError(t, err)
	srv := api.NewServer(st)

	req := httptest.NewRequest("GET", "/api/webhooks", nil)
	// Intentionally NOT calling addTestAuth(t, st, req) — no session cookie attached.
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
