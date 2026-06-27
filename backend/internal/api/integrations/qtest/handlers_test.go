package qtest_test

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

// seedMapping creates a test case in folderID and a QTest mapping for it.
func seedMapping(t *testing.T, s *store.Store, tcID, folderID string, qtestTCID int64) {
	t.Helper()
	require.NoError(t, s.CreateTestCase(&models.TestCase{ID: tcID, Name: tcID, FolderID: folderID}))
	_, err := s.CreateQTestMapping(tcID, qtestTCID, "PID", "Module", 1, "http://qtest.example/"+tcID, "hash", 1)
	require.NoError(t, err)
}

func TestUnlinkFolder_Recursive(t *testing.T) {
	st := newStore(t)
	parent, err := st.CreateFolder("Parent", nil)
	require.NoError(t, err)
	child, err := st.CreateFolder("Child", &parent.ID)
	require.NoError(t, err)
	seedMapping(t, st, "tc-a", parent.ID, 201)
	seedMapping(t, st, "tc-b", child.ID, 202)

	w := do(t, st, "POST", "/api/qtest/unlink-folder", map[string]interface{}{
		"folder_id": parent.ID,
		"recursive": true,
	})
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"deleted":2`)
}

func TestUnlinkFolder_NonRecursive(t *testing.T) {
	st := newStore(t)
	parent, err := st.CreateFolder("Parent", nil)
	require.NoError(t, err)
	child, err := st.CreateFolder("Child", &parent.ID)
	require.NoError(t, err)
	seedMapping(t, st, "tc-a", parent.ID, 201)
	seedMapping(t, st, "tc-b", child.ID, 202)

	w := do(t, st, "POST", "/api/qtest/unlink-folder", map[string]interface{}{
		"folder_id": parent.ID,
		"recursive": false,
	})
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"deleted":1`)
}

func TestUnlinkFolder_MissingFolderID(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "POST", "/api/qtest/unlink-folder", map[string]interface{}{})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBulkUnlink_Success(t *testing.T) {
	st := newStore(t)
	root, err := st.CreateFolder("Root", nil)
	require.NoError(t, err)
	seedMapping(t, st, "tc-1", root.ID, 101)
	seedMapping(t, st, "tc-2", root.ID, 102)

	w := do(t, st, "POST", "/api/qtest/bulk-unlink", map[string]interface{}{
		"test_case_ids": []string{"tc-1", "tc-2"},
	})
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"deleted":2`)
}

func TestBulkUnlink_Empty(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "POST", "/api/qtest/bulk-unlink", map[string]interface{}{
		"test_case_ids": []string{},
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBulkUnlink_ExceedsCap(t *testing.T) {
	st := newStore(t)
	ids := make([]string, 2001)
	for i := range ids {
		ids[i] = "tc"
	}
	w := do(t, st, "POST", "/api/qtest/bulk-unlink", map[string]interface{}{
		"test_case_ids": ids,
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetConfig_NotConfigured(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "GET", "/api/settings/qtest", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "false")
}

func TestGetConfig_Configured(t *testing.T) {
	st := newStore(t)
	_, err := st.UpsertQTestConfig("https://example.qtestnet.com", "u@e.com", "token", 1, "P", true)
	require.NoError(t, err)
	w := do(t, st, "GET", "/api/settings/qtest", nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpsertConfig_Success(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "PUT", "/api/settings/qtest", map[string]interface{}{
		"base_url":     "https://example.qtestnet.com",
		"email":        "u@e.com",
		"api_token":    "secret",
		"project_id":   "1",
		"project_name": "P",
		"enabled":      true,
	})
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpsertConfig_BadJSON(t *testing.T) {
	st := newStore(t)
	r := httptest.NewRequest("PUT", "/api/settings/qtest", strings.NewReader("{bad"))
	r.Header.Set("Content-Type", "application/json")
	auth(t, st, r)
	w := httptest.NewRecorder()
	api.NewServer(st).ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpsertConfig_MissingBaseURL(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "PUT", "/api/settings/qtest", map[string]interface{}{"email": "u@e.com"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpsertConfig_MissingEmail(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "PUT", "/api/settings/qtest", map[string]interface{}{"base_url": "https://x"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTestConnection_NotConfigured(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "POST", "/api/settings/qtest/test-connection", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListProjects_NotConfigured(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "GET", "/api/qtest/projects", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListEnabledProjects_Empty(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "GET", "/api/qtest/enabled-projects", nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAddEnabledProject_Success(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "POST", "/api/qtest/enabled-projects", map[string]interface{}{
		"project_id":   42,
		"project_name": "MyProj",
	})
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAddEnabledProject_MissingProjectID(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "POST", "/api/qtest/enabled-projects", map[string]interface{}{
		"project_name": "only",
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAddEnabledProject_MissingName(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "POST", "/api/qtest/enabled-projects", map[string]interface{}{
		"project_id": 7,
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAddEnabledProject_BadJSON(t *testing.T) {
	st := newStore(t)
	r := httptest.NewRequest("POST", "/api/qtest/enabled-projects", strings.NewReader("{bad"))
	r.Header.Set("Content-Type", "application/json")
	auth(t, st, r)
	w := httptest.NewRecorder()
	api.NewServer(st).ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRemoveEnabledProject_MissingID(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "POST", "/api/qtest/enabled-projects/remove", map[string]interface{}{})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRemoveEnabledProject_BadJSON(t *testing.T) {
	st := newStore(t)
	r := httptest.NewRequest("POST", "/api/qtest/enabled-projects/remove", strings.NewReader("{bad"))
	r.Header.Set("Content-Type", "application/json")
	auth(t, st, r)
	w := httptest.NewRecorder()
	api.NewServer(st).ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSetDefaultProject_BadJSON(t *testing.T) {
	st := newStore(t)
	r := httptest.NewRequest("POST", "/api/qtest/enabled-projects/set-default", strings.NewReader("{bad"))
	r.Header.Set("Content-Type", "application/json")
	auth(t, st, r)
	w := httptest.NewRecorder()
	api.NewServer(st).ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpload_MissingTestCaseIDs(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "POST", "/api/qtest/upload", map[string]interface{}{
		"module_id":  1,
		"project_id": 1,
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpload_MissingModuleID(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "POST", "/api/qtest/upload", map[string]interface{}{
		"test_case_ids": []string{"a"},
		"project_id":    1,
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpload_MissingProjectID(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "POST", "/api/qtest/upload", map[string]interface{}{
		"test_case_ids": []string{"a"},
		"module_id":     1,
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpload_BadJSON(t *testing.T) {
	st := newStore(t)
	r := httptest.NewRequest("POST", "/api/qtest/upload", strings.NewReader("{bad"))
	r.Header.Set("Content-Type", "application/json")
	auth(t, st, r)
	w := httptest.NewRecorder()
	api.NewServer(st).ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBatchGetMappings_EmptyList(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "POST", "/api/qtest/batch-mappings", map[string]interface{}{})
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "mappings")
}

func TestBatchGetMappings_BadJSON(t *testing.T) {
	st := newStore(t)
	r := httptest.NewRequest("POST", "/api/qtest/batch-mappings", strings.NewReader("{bad"))
	r.Header.Set("Content-Type", "application/json")
	auth(t, st, r)
	w := httptest.NewRecorder()
	api.NewServer(st).ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSync_BadJSON(t *testing.T) {
	st := newStore(t)
	r := httptest.NewRequest("POST", "/api/qtest/sync", strings.NewReader("{bad"))
	r.Header.Set("Content-Type", "application/json")
	auth(t, st, r)
	w := httptest.NewRecorder()
	api.NewServer(st).ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUploadFolder_MissingFolderID(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "POST", "/api/qtest/upload-folder", map[string]interface{}{
		"project_id": 1,
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUploadFolder_MissingProjectID(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "POST", "/api/qtest/upload-folder", map[string]interface{}{
		"folder_id": "f1",
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUploadFolder_BadJSON(t *testing.T) {
	st := newStore(t)
	r := httptest.NewRequest("POST", "/api/qtest/upload-folder", strings.NewReader("{bad"))
	r.Header.Set("Content-Type", "application/json")
	auth(t, st, r)
	w := httptest.NewRecorder()
	api.NewServer(st).ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListModules_NotConfigured(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "GET", "/api/qtest/modules?project_id=1", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListModules_MissingProjectID(t *testing.T) {
	st := newStore(t)
	// Config without a project_id so the handler demands one.
	_, err := st.UpsertQTestConfig("https://example.qtestnet.com", "u@e.com", "token", 0, "", true)
	require.NoError(t, err)
	w := do(t, st, "GET", "/api/qtest/modules", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListTestCases_NotConfigured(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "GET", "/api/qtest/test-cases?project_id=1&module_id=1", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListTestCases_MissingParams(t *testing.T) {
	st := newStore(t)
	_, err := st.UpsertQTestConfig("https://example.qtestnet.com", "u@e.com", "token", 1, "P", true)
	require.NoError(t, err)
	w := do(t, st, "GET", "/api/qtest/test-cases", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListTestCases_InvalidProjectID(t *testing.T) {
	st := newStore(t)
	_, err := st.UpsertQTestConfig("https://example.qtestnet.com", "u@e.com", "token", 1, "P", true)
	require.NoError(t, err)
	w := do(t, st, "GET", "/api/qtest/test-cases?project_id=abc&module_id=1", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListTestCases_InvalidModuleID(t *testing.T) {
	st := newStore(t)
	_, err := st.UpsertQTestConfig("https://example.qtestnet.com", "u@e.com", "token", 1, "P", true)
	require.NoError(t, err)
	w := do(t, st, "GET", "/api/qtest/test-cases?project_id=1&module_id=abc", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestImport_NotConfigured(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "POST", "/api/qtest/import", map[string]interface{}{})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestImport_BadJSON(t *testing.T) {
	st := newStore(t)
	_, err := st.UpsertQTestConfig("https://example.qtestnet.com", "u@e.com", "token", 1, "P", true)
	require.NoError(t, err)
	r := httptest.NewRequest("POST", "/api/qtest/import", strings.NewReader("{bad"))
	r.Header.Set("Content-Type", "application/json")
	auth(t, st, r)
	w := httptest.NewRecorder()
	api.NewServer(st).ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestImport_MissingProjectID(t *testing.T) {
	st := newStore(t)
	_, err := st.UpsertQTestConfig("https://example.qtestnet.com", "u@e.com", "token", 1, "P", true)
	require.NoError(t, err)
	w := do(t, st, "POST", "/api/qtest/import", map[string]interface{}{})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestImport_MissingFolderID(t *testing.T) {
	st := newStore(t)
	_, err := st.UpsertQTestConfig("https://example.qtestnet.com", "u@e.com", "token", 1, "P", true)
	require.NoError(t, err)
	w := do(t, st, "POST", "/api/qtest/import", map[string]interface{}{
		"project_id": 1,
		"module_id":  1,
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestImport_NoTestCases(t *testing.T) {
	st := newStore(t)
	_, err := st.UpsertQTestConfig("https://example.qtestnet.com", "u@e.com", "token", 1, "P", true)
	require.NoError(t, err)
	w := do(t, st, "POST", "/api/qtest/import", map[string]interface{}{
		"project_id": 1,
		"module_id":  1,
		"folder_id":  "f1",
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestImport_InvalidOnConflict(t *testing.T) {
	st := newStore(t)
	_, err := st.UpsertQTestConfig("https://example.qtestnet.com", "u@e.com", "token", 1, "P", true)
	require.NoError(t, err)
	w := do(t, st, "POST", "/api/qtest/import", map[string]interface{}{
		"project_id":  1,
		"module_id":   1,
		"folder_id":   "f1",
		"test_cases":  []map[string]interface{}{{"id": 1}},
		"on_conflict": "bogus",
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
