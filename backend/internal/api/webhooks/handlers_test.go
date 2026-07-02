package webhooks_test

import (
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
