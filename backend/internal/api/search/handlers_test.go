package search_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	api "ttgo/internal/api"
	"ttgo/pkg/tracker/models"
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

func do(t *testing.T, st *store.Store, path string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest("GET", path, nil)
	auth(t, st, r)
	w := httptest.NewRecorder()
	api.NewServer(st).ServeHTTP(w, r)
	return w
}

func TestSearch_EmptyQuery(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "/api/search")
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(0), resp["total"])
}

func TestSearch_WithResults(t *testing.T) {
	st := newStore(t)
	root, _ := st.CreateFolder("Root", nil)
	require.NoError(t, st.CreateTestCase(&models.TestCase{Name: "Login Test", FolderID: root.ID}))

	w := do(t, st, "/api/search?q=Login")
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Login", resp["query"])
}

func TestSearch_Pagination(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "/api/search?q=foo&limit=10&offset=5")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSearch_BadPagination(t *testing.T) {
	st := newStore(t)
	// Bad pagination values silently fall back to defaults.
	w := do(t, st, "/api/search?q=foo&limit=abc&offset=-1")
	assert.Equal(t, http.StatusOK, w.Code)
}
