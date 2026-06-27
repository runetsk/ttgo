package categories_test

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

func TestCreateCategory_Success(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "POST", "/api/categories", map[string]string{
		"name": "Smoke", "description": "smoke tests",
	})
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCreateCategory_MissingName(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "POST", "/api/categories", map[string]string{"description": "x"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateCategory_BadJSON(t *testing.T) {
	st := newStore(t)
	r := httptest.NewRequest("POST", "/api/categories", strings.NewReader("{bad"))
	r.Header.Set("Content-Type", "application/json")
	auth(t, st, r)
	w := httptest.NewRecorder()
	api.NewServer(st).ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListCategories(t *testing.T) {
	st := newStore(t)
	_, err := st.CreateCategory("A", "")
	require.NoError(t, err)
	_, err = st.CreateCategory("B", "")
	require.NoError(t, err)

	w := do(t, st, "GET", "/api/categories", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(2), resp["total"])
}

func TestListCategories_WithQuery(t *testing.T) {
	st := newStore(t)
	_, err := st.CreateCategory("Smoke", "")
	require.NoError(t, err)
	_, err = st.CreateCategory("Regression", "")
	require.NoError(t, err)

	w := do(t, st, "GET", "/api/categories?q=Smoke&limit=5&offset=0", nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDeleteCategory(t *testing.T) {
	st := newStore(t)
	cat, err := st.CreateCategory("X", "")
	require.NoError(t, err)
	w := do(t, st, "DELETE", "/api/categories/"+cat.ID, nil)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestBulkDeleteCategories(t *testing.T) {
	st := newStore(t)
	c1, err := st.CreateCategory("1", "")
	require.NoError(t, err)
	c2, err := st.CreateCategory("2", "")
	require.NoError(t, err)
	w := do(t, st, "POST", "/api/categories/bulk-delete", map[string]interface{}{
		"ids": []string{c1.ID, c2.ID},
	})
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestBulkDeleteCategories_BadJSON(t *testing.T) {
	st := newStore(t)
	r := httptest.NewRequest("POST", "/api/categories/bulk-delete", strings.NewReader("{bad"))
	r.Header.Set("Content-Type", "application/json")
	auth(t, st, r)
	w := httptest.NewRecorder()
	api.NewServer(st).ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
