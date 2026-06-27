package folders_test

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

// TestGetFolderNotFound verifies that GET /api/folders/{unknown-id} returns HTTP 404
// with {"error":"folder not found"} rather than a 500 server error.
func TestGetFolderNotFound(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := api.NewServer(s)
	req := httptest.NewRequest(http.MethodGet, "/api/folders/non-existent-uuid", nil)
	addTestAuth(t, s, req)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "folder not found")
}

// TestGetFolderOK verifies that GET /api/folders/{id} returns HTTP 200 for an existing folder.
func TestGetFolderOK(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := api.NewServer(s)

	folder, err := s.CreateFolder("My Folder", nil)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/folders/"+folder.ID, nil)
	addTestAuth(t, s, req)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "My Folder")
}
