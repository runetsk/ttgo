package customfields_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestListCustomFields_Empty(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "GET", "/api/custom-fields", nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCreateCustomField_Success(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "POST", "/api/custom-fields", map[string]interface{}{
		"name":         "Priority",
		"type":         "TEXT",
		"is_mandatory": false,
	})
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCreateCustomField_MissingFields(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "POST", "/api/custom-fields", map[string]string{"name": "only"})
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = do(t, st, "POST", "/api/custom-fields", map[string]string{"type": "only"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateCustomField_BadJSON(t *testing.T) {
	st := newStore(t)
	r := httptest.NewRequest("POST", "/api/custom-fields", strings.NewReader("{bad"))
	r.Header.Set("Content-Type", "application/json")
	auth(t, st, r)
	w := httptest.NewRecorder()
	api.NewServer(st).ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteCustomField(t *testing.T) {
	st := newStore(t)
	def := &models.CustomFieldDefinition{Name: "X", Type: "string"}
	require.NoError(t, st.CreateCustomFieldDefinition(def))
	w := do(t, st, "DELETE", "/api/custom-fields/"+def.ID, nil)
	assert.Equal(t, http.StatusNoContent, w.Code)
}
