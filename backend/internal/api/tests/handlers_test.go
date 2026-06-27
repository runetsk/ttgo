package tests_test

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

func seedFolderAndTest(t *testing.T, st *store.Store) (*models.Folder, *models.TestCase) {
	t.Helper()
	root, err := st.CreateFolder("Root", nil)
	require.NoError(t, err)
	tc := &models.TestCase{Name: "TC1", FolderID: root.ID, Description: "desc"}
	require.NoError(t, st.CreateTestCase(tc))
	return root, tc
}

func TestGetTests_Empty(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "GET", "/api/tests", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var arr []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &arr))
	assert.Empty(t, arr)
}

func TestGetTests_FolderIDsCSV(t *testing.T) {
	st := newStore(t)
	root, tc := seedFolderAndTest(t, st)
	_ = tc
	w := do(t, st, "GET", "/api/tests?folder_ids="+root.ID, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var arr []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &arr))
	assert.Len(t, arr, 1)
}

func TestGetTests_FolderIDsRepeated(t *testing.T) {
	st := newStore(t)
	root, _ := seedFolderAndTest(t, st)
	w := do(t, st, "GET", "/api/tests?folder_id="+root.ID, nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetTests_CategoryID(t *testing.T) {
	st := newStore(t)
	_, tc := seedFolderAndTest(t, st)
	cat, err := st.CreateCategory("Smoke", "")
	require.NoError(t, err)
	require.NoError(t, st.AssignCategoryToTest(cat.ID, tc.ID))

	w := do(t, st, "GET", "/api/tests?category_id="+cat.ID, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var arr []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &arr))
	assert.Len(t, arr, 1)
}

func TestGetTests_ListView(t *testing.T) {
	st := newStore(t)
	seedFolderAndTest(t, st)
	w := do(t, st, "GET", "/api/tests?view=list", nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetTestByCustomField_MissingParams(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "GET", "/api/tests/by-custom-field", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetTestByCustomField_NotFound(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "GET", "/api/tests/by-custom-field?field=QTestId&value=x", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetTest_Success(t *testing.T) {
	st := newStore(t)
	_, tc := seedFolderAndTest(t, st)
	w := do(t, st, "GET", "/api/tests/"+tc.ID, nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetTest_NotFound(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "GET", "/api/tests/nope", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCreateTest_Success(t *testing.T) {
	st := newStore(t)
	root, err := st.CreateFolder("R", nil)
	require.NoError(t, err)
	w := do(t, st, "POST", "/api/tests", map[string]interface{}{
		"name":        "My Test",
		"folder_id":   root.ID,
		"description": "<p>hello</p>",
		"steps": []map[string]interface{}{
			{"action": "<p>click</p>", "expected_result": "<p>pass</p>"},
		},
	})
	require.Equal(t, http.StatusCreated, w.Code)
}

func TestCreateTest_BadJSON(t *testing.T) {
	st := newStore(t)
	r := httptest.NewRequest("POST", "/api/tests", strings.NewReader("{bad"))
	r.Header.Set("Content-Type", "application/json")
	auth(t, st, r)
	w := httptest.NewRecorder()
	api.NewServer(st).ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateTest_MissingFields(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "POST", "/api/tests", map[string]string{"name": "only"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateTest_NonexistentFolder(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "POST", "/api/tests", map[string]string{
		"name":      "x",
		"folder_id": "doesnotexist",
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateTest_Success(t *testing.T) {
	st := newStore(t)
	root, tc := seedFolderAndTest(t, st)
	w := do(t, st, "PUT", "/api/tests/"+tc.ID, map[string]interface{}{
		"name":        "renamed",
		"folder_id":   root.ID,
		"description": "<p>new</p>",
		"steps":       []map[string]interface{}{{"action": "<p>go</p>", "expected_result": "<p>ok</p>"}},
	})
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpdateTest_BadJSON(t *testing.T) {
	st := newStore(t)
	_, tc := seedFolderAndTest(t, st)
	r := httptest.NewRequest("PUT", "/api/tests/"+tc.ID, strings.NewReader("{bad"))
	r.Header.Set("Content-Type", "application/json")
	auth(t, st, r)
	w := httptest.NewRecorder()
	api.NewServer(st).ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteTest(t *testing.T) {
	st := newStore(t)
	_, tc := seedFolderAndTest(t, st)
	w := do(t, st, "DELETE", "/api/tests/"+tc.ID, nil)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestBulkDeleteTests(t *testing.T) {
	st := newStore(t)
	_, tc := seedFolderAndTest(t, st)
	w := do(t, st, "POST", "/api/tests/bulk-delete", map[string]interface{}{
		"ids": []string{tc.ID},
	})
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestBulkDeleteTests_BadJSON(t *testing.T) {
	st := newStore(t)
	r := httptest.NewRequest("POST", "/api/tests/bulk-delete", strings.NewReader("{bad"))
	r.Header.Set("Content-Type", "application/json")
	auth(t, st, r)
	w := httptest.NewRecorder()
	api.NewServer(st).ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAssignCategory_Success(t *testing.T) {
	st := newStore(t)
	_, tc := seedFolderAndTest(t, st)
	cat, err := st.CreateCategory("Smoke", "")
	require.NoError(t, err)
	w := do(t, st, "POST", "/api/tests/"+tc.ID+"/categories", map[string]string{
		"category_id": cat.ID,
	})
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestAssignCategory_MissingCategoryID(t *testing.T) {
	st := newStore(t)
	_, tc := seedFolderAndTest(t, st)
	w := do(t, st, "POST", "/api/tests/"+tc.ID+"/categories", map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAssignCategory_BadJSON(t *testing.T) {
	st := newStore(t)
	_, tc := seedFolderAndTest(t, st)
	r := httptest.NewRequest("POST", "/api/tests/"+tc.ID+"/categories", strings.NewReader("{bad"))
	r.Header.Set("Content-Type", "application/json")
	auth(t, st, r)
	w := httptest.NewRecorder()
	api.NewServer(st).ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestExportTests_Success(t *testing.T) {
	st := newStore(t)
	_, tc := seedFolderAndTest(t, st)
	w := do(t, st, "POST", "/api/tests/export", map[string]interface{}{
		"ids":    []string{tc.ID},
		"fields": []string{"name", "description", "steps", "categories", "custom_values", "linked_requirements", "bogus"},
	})
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment")

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(1), resp["count"])
}

func TestExportTests_EmptyIDs(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "POST", "/api/tests/export", map[string]interface{}{
		"ids": []string{},
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestExportTests_BadJSON(t *testing.T) {
	st := newStore(t)
	r := httptest.NewRequest("POST", "/api/tests/export", strings.NewReader("{bad"))
	r.Header.Set("Content-Type", "application/json")
	auth(t, st, r)
	w := httptest.NewRecorder()
	api.NewServer(st).ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListVersions(t *testing.T) {
	st := newStore(t)
	_, tc := seedFolderAndTest(t, st)
	w := do(t, st, "GET", "/api/tests/"+tc.ID+"/versions", nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetVersion_NotFound(t *testing.T) {
	st := newStore(t)
	_, tc := seedFolderAndTest(t, st)
	w := do(t, st, "GET", "/api/tests/"+tc.ID+"/versions/nope", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRestoreVersion_NotFound(t *testing.T) {
	st := newStore(t)
	_, tc := seedFolderAndTest(t, st)
	w := do(t, st, "POST", "/api/tests/"+tc.ID+"/versions/nope/restore", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestListTestExecutions(t *testing.T) {
	st := newStore(t)
	_, tc := seedFolderAndTest(t, st)
	w := do(t, st, "GET", "/api/tests/"+tc.ID+"/executions", nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCreateJiraIssue_BadJSON(t *testing.T) {
	st := newStore(t)
	_, tc := seedFolderAndTest(t, st)
	r := httptest.NewRequest("POST", "/api/tests/"+tc.ID+"/defect-links/create-issue", strings.NewReader("{bad"))
	r.Header.Set("Content-Type", "application/json")
	auth(t, st, r)
	w := httptest.NewRecorder()
	api.NewServer(st).ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateJiraIssue_MissingSummary(t *testing.T) {
	st := newStore(t)
	_, tc := seedFolderAndTest(t, st)
	w := do(t, st, "POST", "/api/tests/"+tc.ID+"/defect-links/create-issue", map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateJiraIssue_StoreError(t *testing.T) {
	st := newStore(t)
	_, tc := seedFolderAndTest(t, st)
	// Jira not configured in store -> store returns error, handler returns 400.
	w := do(t, st, "POST", "/api/tests/"+tc.ID+"/defect-links/create-issue", map[string]string{
		"summary": "bug",
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDismissReverification(t *testing.T) {
	st := newStore(t)
	_, tc := seedFolderAndTest(t, st)
	w := do(t, st, "DELETE", "/api/tests/"+tc.ID+"/reverification-flag", nil)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestGetQTestMapping_NotLinked(t *testing.T) {
	st := newStore(t)
	_, tc := seedFolderAndTest(t, st)
	w := do(t, st, "GET", "/api/tests/"+tc.ID+"/qtest-mapping", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, false, resp["linked"])
}

func TestUnlinkQTestMapping_NoMapping(t *testing.T) {
	st := newStore(t)
	_, tc := seedFolderAndTest(t, st)
	// Store returns an error when no mapping exists -> 500.
	w := do(t, st, "DELETE", "/api/tests/"+tc.ID+"/qtest-mapping", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
