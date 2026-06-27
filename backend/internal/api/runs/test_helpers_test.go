package runs_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	api "ttgo/internal/api"
	"ttgo/pkg/tracker/store"

	"github.com/stretchr/testify/require"
)

type testEnv struct {
	srv          *api.Server
	sessionToken string
}

func testServer(t *testing.T) (*testEnv, func()) {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "ttgo-test-*.db")
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())

	s, err := store.New(tmpFile.Name())
	require.NoError(t, err)
	require.NoError(t, s.SeedAdminIfNeeded("test@test.com", "testpass123"))

	srv := api.NewServer(s)

	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"email":"test@test.com","password":"testpass123"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRR := httptest.NewRecorder()
	srv.ServeHTTP(loginRR, loginReq)
	require.Equal(t, http.StatusOK, loginRR.Code, loginRR.Body.String())

	var sessionToken string
	for _, c := range loginRR.Result().Cookies() {
		if c.Name == "session_token" {
			sessionToken = c.Value
			break
		}
	}
	require.NotEmpty(t, sessionToken)

	env := &testEnv{srv: srv, sessionToken: sessionToken}
	cleanup := func() { _ = os.Remove(tmpFile.Name()) }
	return env, cleanup
}

func doRequest(env *testEnv, method, path string, body interface{}) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "session_token", Value: env.sessionToken})
	rr := httptest.NewRecorder()
	env.srv.ServeHTTP(rr, req)
	return rr
}

func newTestStore(t *testing.T) (*store.Store, error) {
	t.Helper()
	return store.New(":memory:")
}

func addTestAuth(t *testing.T, s *store.Store, req *http.Request) {
	t.Helper()
	err := s.SeedAdminIfNeeded("test@test.com", "testpassword1234")
	require.NoError(t, err)
	user, err := s.FindUserByEmail("test@test.com")
	require.NoError(t, err)
	sess, err := s.CreateSession(user.ID)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: sess.ID})
}
