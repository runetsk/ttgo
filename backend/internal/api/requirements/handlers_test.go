package requirements_test

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

func createReq(t *testing.T, st *store.Store, id, title string) *models.Requirement {
	t.Helper()
	r := &models.Requirement{Identifier: id, Title: title}
	require.NoError(t, st.CreateRequirement(r))
	return r
}

func TestCreateRequirement_Success(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "POST", "/api/requirements", map[string]string{
		"identifier":  "REQ-1",
		"title":       "First",
		"description": "desc",
	})
	require.Equal(t, http.StatusCreated, w.Code)
}

func TestCreateRequirement_MissingFields(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "POST", "/api/requirements", map[string]string{"title": "t"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "identifier")

	w = do(t, st, "POST", "/api/requirements", map[string]string{"identifier": "x"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "title")
}

func TestCreateRequirement_BadJSON(t *testing.T) {
	st := newStore(t)
	r := httptest.NewRequest("POST", "/api/requirements", strings.NewReader("{not json"))
	r.Header.Set("Content-Type", "application/json")
	auth(t, st, r)
	w := httptest.NewRecorder()
	api.NewServer(st).ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateRequirement_Duplicate(t *testing.T) {
	st := newStore(t)
	createReq(t, st, "REQ-1", "First")
	w := do(t, st, "POST", "/api/requirements", map[string]string{
		"identifier": "REQ-1", "title": "dup",
	})
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestListRequirements(t *testing.T) {
	st := newStore(t)
	createReq(t, st, "REQ-1", "a")
	createReq(t, st, "REQ-2", "b")
	w := do(t, st, "GET", "/api/requirements", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var arr []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &arr))
	assert.Len(t, arr, 2)
}

func TestGetRequirement_Success(t *testing.T) {
	st := newStore(t)
	r := createReq(t, st, "REQ-X", "X")
	w := do(t, st, "GET", "/api/requirements/"+r.ID, nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetRequirement_NotFound(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "GET", "/api/requirements/nope", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUpdateRequirement_Success(t *testing.T) {
	st := newStore(t)
	r := createReq(t, st, "REQ-U", "orig")
	w := do(t, st, "PUT", "/api/requirements/"+r.ID, map[string]string{
		"identifier": "REQ-U2", "title": "new", "description": "upd",
	})
	assert.Equal(t, http.StatusOK, w.Code)
	got, err := st.GetRequirement(r.ID)
	require.NoError(t, err)
	assert.Equal(t, "REQ-U2", got.Identifier)
}

func TestUpdateRequirement_NotFound(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "PUT", "/api/requirements/nope", map[string]string{
		"identifier": "x", "title": "y",
	})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUpdateRequirement_MissingFields(t *testing.T) {
	st := newStore(t)
	r := createReq(t, st, "REQ-U", "orig")
	w := do(t, st, "PUT", "/api/requirements/"+r.ID, map[string]string{"title": "only"})
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = do(t, st, "PUT", "/api/requirements/"+r.ID, map[string]string{"identifier": "only"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateRequirement_BadJSON(t *testing.T) {
	st := newStore(t)
	r := createReq(t, st, "REQ-U", "orig")
	req := httptest.NewRequest("PUT", "/api/requirements/"+r.ID, strings.NewReader("{not json"))
	req.Header.Set("Content-Type", "application/json")
	auth(t, st, req)
	w := httptest.NewRecorder()
	api.NewServer(st).ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateRequirement_DuplicateIdentifier(t *testing.T) {
	st := newStore(t)
	createReq(t, st, "REQ-A", "a")
	r := createReq(t, st, "REQ-B", "b")
	w := do(t, st, "PUT", "/api/requirements/"+r.ID, map[string]string{
		"identifier": "REQ-A", "title": "conflict",
	})
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestDeleteRequirement(t *testing.T) {
	st := newStore(t)
	r := createReq(t, st, "REQ-D", "d")
	w := do(t, st, "DELETE", "/api/requirements/"+r.ID, nil)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestBulkDeleteRequirements(t *testing.T) {
	st := newStore(t)
	r1 := createReq(t, st, "REQ-1", "a")
	r2 := createReq(t, st, "REQ-2", "b")
	w := do(t, st, "POST", "/api/requirements/bulk-delete", map[string]interface{}{
		"ids": []string{r1.ID, r2.ID},
	})
	assert.Equal(t, http.StatusNoContent, w.Code)

	list, err := st.ListRequirements()
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestBulkDeleteRequirements_EmptyIDs(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "POST", "/api/requirements/bulk-delete", map[string]interface{}{
		"ids": []string{},
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBulkDeleteRequirements_BadJSON(t *testing.T) {
	st := newStore(t)
	r := httptest.NewRequest("POST", "/api/requirements/bulk-delete", strings.NewReader("{broken"))
	r.Header.Set("Content-Type", "application/json")
	auth(t, st, r)
	w := httptest.NewRecorder()
	api.NewServer(st).ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListChildren_Empty(t *testing.T) {
	st := newStore(t)
	r := createReq(t, st, "REQ-P", "parent")
	w := do(t, st, "GET", "/api/requirements/"+r.ID+"/children", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "[]")
}

func TestCreateLink_Success(t *testing.T) {
	st := newStore(t)
	root, err := st.CreateFolder("Root", nil)
	require.NoError(t, err)
	tc := &models.TestCase{Name: "TC1", FolderID: root.ID}
	require.NoError(t, st.CreateTestCase(tc))
	r := createReq(t, st, "REQ-L", "l")

	w := do(t, st, "POST", "/api/requirements/"+r.ID+"/links",
		map[string]string{"test_case_id": tc.ID})
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCreateLink_MissingTestCaseID(t *testing.T) {
	st := newStore(t)
	r := createReq(t, st, "REQ-L", "l")
	w := do(t, st, "POST", "/api/requirements/"+r.ID+"/links", map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateLink_BadJSON(t *testing.T) {
	st := newStore(t)
	r := createReq(t, st, "REQ-L", "l")
	req := httptest.NewRequest("POST", "/api/requirements/"+r.ID+"/links", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	auth(t, st, req)
	w := httptest.NewRecorder()
	api.NewServer(st).ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateLink_Duplicate(t *testing.T) {
	st := newStore(t)
	root, _ := st.CreateFolder("Root", nil)
	tc := &models.TestCase{Name: "TC1", FolderID: root.ID}
	require.NoError(t, st.CreateTestCase(tc))
	r := createReq(t, st, "REQ-L", "l")
	_, err := st.CreateLink(r.ID, tc.ID)
	require.NoError(t, err)

	w := do(t, st, "POST", "/api/requirements/"+r.ID+"/links",
		map[string]string{"test_case_id": tc.ID})
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestCreateLink_NotFound(t *testing.T) {
	st := newStore(t)
	r := createReq(t, st, "REQ-L", "l")
	w := do(t, st, "POST", "/api/requirements/"+r.ID+"/links",
		map[string]string{"test_case_id": "nope"})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteLink_Success(t *testing.T) {
	st := newStore(t)
	root, _ := st.CreateFolder("Root", nil)
	tc := &models.TestCase{Name: "TC1", FolderID: root.ID}
	require.NoError(t, st.CreateTestCase(tc))
	r := createReq(t, st, "REQ-L", "l")
	_, err := st.CreateLink(r.ID, tc.ID)
	require.NoError(t, err)

	w := do(t, st, "DELETE", "/api/requirements/"+r.ID+"/links/"+tc.ID, nil)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestDeleteLink_NotFound(t *testing.T) {
	st := newStore(t)
	r := createReq(t, st, "REQ-L", "l")
	w := do(t, st, "DELETE", "/api/requirements/"+r.ID+"/links/unknown", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestListTestCaseRequirements(t *testing.T) {
	st := newStore(t)
	root, _ := st.CreateFolder("Root", nil)
	tc := &models.TestCase{Name: "TC1", FolderID: root.ID}
	require.NoError(t, st.CreateTestCase(tc))
	r := createReq(t, st, "REQ-L", "l")
	_, err := st.CreateLink(r.ID, tc.ID)
	require.NoError(t, err)

	w := do(t, st, "GET", "/api/tests/"+tc.ID+"/requirements", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	var arr []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &arr))
	assert.Len(t, arr, 1)
}

func TestTraceabilityMatrix(t *testing.T) {
	st := newStore(t)
	createReq(t, st, "REQ-A", "alpha")
	createReq(t, st, "REQ-B", "beta")
	w := do(t, st, "GET", "/api/traceability", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotNil(t, resp["rows"])
}

func TestTraceabilityMatrix_QueryFilter(t *testing.T) {
	st := newStore(t)
	createReq(t, st, "REQ-A", "alpha")
	createReq(t, st, "REQ-B", "beta")
	w := do(t, st, "GET", "/api/traceability?q=alpha", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	rows := resp["rows"].([]interface{})
	assert.Len(t, rows, 1)
}

func TestTraceabilityMatrix_UncoveredOnly(t *testing.T) {
	st := newStore(t)
	createReq(t, st, "REQ-A", "a")
	w := do(t, st, "GET", "/api/traceability?uncovered=true", nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestImportRequirement_BadJSON(t *testing.T) {
	st := newStore(t)
	r := httptest.NewRequest("POST", "/api/requirements/import", strings.NewReader("{bad"))
	r.Header.Set("Content-Type", "application/json")
	auth(t, st, r)
	w := httptest.NewRecorder()
	api.NewServer(st).ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestImportRequirement_InvalidSourceType(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "POST", "/api/requirements/import", map[string]string{
		"source_type": "notion", "source_key": "X-1",
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestImportRequirement_MissingKey(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "POST", "/api/requirements/import", map[string]string{
		"source_type": "jira",
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestImportRequirement_Duplicate(t *testing.T) {
	st := newStore(t)
	r := &models.Requirement{
		Identifier: "PROJ-1", Title: "t",
		SourceType: "jira", SourceKey: "PROJ-1",
	}
	require.NoError(t, st.CreateImportedRequirement(r))

	w := do(t, st, "POST", "/api/requirements/import", map[string]string{
		"source_type": "jira", "source_key": "PROJ-1",
	})
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestImportRequirement_JiraNotConfigured(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "POST", "/api/requirements/import", map[string]string{
		"source_type": "jira", "source_key": "PROJ-999",
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Jira")
}

func TestImportRequirement_ConfluenceNotConfigured(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "POST", "/api/requirements/import", map[string]string{
		"source_type": "confluence", "source_key": "123",
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Confluence")
}

func TestBulkImport_BadJSON(t *testing.T) {
	st := newStore(t)
	r := httptest.NewRequest("POST", "/api/requirements/bulk-import", strings.NewReader("{bad"))
	r.Header.Set("Content-Type", "application/json")
	auth(t, st, r)
	w := httptest.NewRecorder()
	api.NewServer(st).ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBulkImport_InvalidSourceType(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "POST", "/api/requirements/bulk-import", map[string]interface{}{
		"source_type": "notion",
		"source_keys": []string{"A"},
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBulkImport_EmptyKeys(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "POST", "/api/requirements/bulk-import", map[string]interface{}{
		"source_type": "jira",
		"source_keys": []string{},
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBulkImport_SkipsAndFails(t *testing.T) {
	st := newStore(t)
	existing := &models.Requirement{
		Identifier: "EXIST-1", Title: "x",
		SourceType: "jira", SourceKey: "EXIST-1",
	}
	require.NoError(t, st.CreateImportedRequirement(existing))

	w := do(t, st, "POST", "/api/requirements/bulk-import", map[string]interface{}{
		"source_type": "jira",
		"source_keys": []string{"EXIST-1", "NEW-1"},
	})
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	skipped, _ := resp["skipped"].([]interface{})
	failed, _ := resp["failed"].([]interface{})
	assert.Len(t, skipped, 1)
	// NEW-1 fails because Jira is not configured
	assert.Len(t, failed, 1)
}

func TestResync_NotFound(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "POST", "/api/requirements/nope/resync", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestResync_NotImported(t *testing.T) {
	st := newStore(t)
	r := createReq(t, st, "REQ-1", "local")
	w := do(t, st, "POST", "/api/requirements/"+r.ID+"/resync", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestResyncResolve_BadJSON(t *testing.T) {
	st := newStore(t)
	r := createReq(t, st, "REQ-1", "local")
	req := httptest.NewRequest("POST", "/api/requirements/"+r.ID+"/resync/resolve", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	auth(t, st, req)
	w := httptest.NewRecorder()
	api.NewServer(st).ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestResyncResolve_InvalidResolution(t *testing.T) {
	st := newStore(t)
	r := createReq(t, st, "REQ-1", "local")
	w := do(t, st, "POST", "/api/requirements/"+r.ID+"/resync/resolve", map[string]string{
		"resolution": "bogus",
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestResyncResolve_AcceptRemote(t *testing.T) {
	st := newStore(t)
	r := &models.Requirement{
		Identifier: "PROJ-R", Title: "old", Description: "old",
		SourceType: "jira", SourceKey: "PROJ-R",
	}
	require.NoError(t, st.CreateImportedRequirement(r))
	w := do(t, st, "POST", "/api/requirements/"+r.ID+"/resync/resolve", map[string]string{
		"resolution":         "accept_remote",
		"remote_title":       "new",
		"remote_description": "new desc",
	})
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestResyncResolve_KeepLocal(t *testing.T) {
	st := newStore(t)
	r := &models.Requirement{
		Identifier: "PROJ-K", Title: "t",
		SourceType: "jira", SourceKey: "PROJ-K",
	}
	require.NoError(t, st.CreateImportedRequirement(r))
	w := do(t, st, "POST", "/api/requirements/"+r.ID+"/resync/resolve", map[string]string{
		"resolution": "keep_local",
	})
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUnlink_Success(t *testing.T) {
	st := newStore(t)
	r := &models.Requirement{
		Identifier: "PROJ-U", Title: "t",
		SourceType: "jira", SourceKey: "PROJ-U",
	}
	require.NoError(t, st.CreateImportedRequirement(r))
	w := do(t, st, "POST", "/api/requirements/"+r.ID+"/unlink", nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPostToJira_NotFound(t *testing.T) {
	st := newStore(t)
	w := do(t, st, "POST", "/api/requirements/nope/post-to-jira", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPostToJira_NotJiraSourced(t *testing.T) {
	st := newStore(t)
	r := createReq(t, st, "REQ-1", "local")
	w := do(t, st, "POST", "/api/requirements/"+r.ID+"/post-to-jira", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPostToJira_JiraNotConfigured(t *testing.T) {
	st := newStore(t)
	r := &models.Requirement{
		Identifier: "PROJ-P", Title: "t",
		SourceType: "jira", SourceKey: "PROJ-P",
	}
	require.NoError(t, st.CreateImportedRequirement(r))
	w := do(t, st, "POST", "/api/requirements/"+r.ID+"/post-to-jira", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
