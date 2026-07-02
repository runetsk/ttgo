package backups_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	api "ttgo/internal/api"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

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

// newFileBackedServer sets up a store and server backed by real files on disk.
//
// The backups Manager uses hardcoded relative paths — filepath.Join("backups", ...)
// for the destination and os.Stat("tracker.db") / store.CopyFile("tracker.db", ...) for
// the source — both relative to the process CWD, not the store's configured DSN.
// NewManager takes no directory parameter, so the only way to isolate a test's backup
// files from the repo tree is to chdir into a temp dir and open the store at the exact
// relative filename "tracker.db" the Manager expects, then pre-create "backups/".
func newFileBackedServer(t *testing.T) (*store.Store, *api.Server) {
	t.Helper()
	tmp := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmp))
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	require.NoError(t, os.MkdirAll("backups", 0o755))
	st, err := store.New("tracker.db")
	require.NoError(t, err)
	// Close the DB file handle before t.TempDir()'s own cleanup tries to remove the
	// directory — t.Cleanup runs LIFO, and this is registered after t.TempDir()'s
	// internal removal hook, so it runs first and releases the Windows file lock.
	t.Cleanup(func() { _ = st.Close() })
	return st, api.NewServer(st)
}

// ── Group A: auth gating + public maintenance status (no file I/O) ─────────────

// TestBackups_NonAdminForbidden verifies a member-role (non-admin) session is
// rejected with 403 on an admin-gated backups route.
func TestBackups_NonAdminForbidden(t *testing.T) {
	st, err := store.New(":memory:")
	require.NoError(t, err)
	srv := api.NewServer(st)

	hash, err := bcrypt.GenerateFromPassword([]byte("memberpassword1"), 12)
	require.NoError(t, err)
	user, err := st.CreateUser("member@example.com", "Member User", string(hash), "member")
	require.NoError(t, err)
	sess, err := st.CreateSession(user.ID)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/backups", nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: sess.ID})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestBackups_Unauthenticated verifies a request with no session cookie is
// rejected with 401 on an admin-gated backups route.
func TestBackups_Unauthenticated(t *testing.T) {
	st, err := store.New(":memory:")
	require.NoError(t, err)
	srv := api.NewServer(st)

	req := httptest.NewRequest(http.MethodGet, "/api/backups", nil)
	// Intentionally no session cookie attached.
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestMaintenanceStatus_Public verifies GET /api/maintenance-status requires no
// authentication and returns 200, since the frontend must be able to poll it
// during a restore when the caller may not have a valid session.
func TestMaintenanceStatus_Public(t *testing.T) {
	st, err := store.New(":memory:")
	require.NoError(t, err)
	srv := api.NewServer(st)

	req := httptest.NewRequest(http.MethodGet, "/api/maintenance-status", nil)
	// Intentionally no session cookie attached.
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Maintenance bool `json:"maintenance"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.Maintenance)
}

// ── Group B: real backup files (chdir'd temp dir + file-backed store) ──────────

// TestCreateAndListBackup verifies POST /api/backups creates a completed backup
// (201, per handlers.go's Create) and that it subsequently appears in the list.
func TestCreateAndListBackup(t *testing.T) {
	st, srv := newFileBackedServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/backups", nil)
	addTestAuth(t, st, req)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())
	var created models.Backup
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	assert.NotEmpty(t, created.ID)
	assert.Equal(t, "completed", created.Status)
	assert.Equal(t, "manual", created.Type)

	listReq := httptest.NewRequest(http.MethodGet, "/api/backups", nil)
	addTestAuth(t, st, listReq)
	listW := httptest.NewRecorder()
	srv.ServeHTTP(listW, listReq)

	require.Equal(t, http.StatusOK, listW.Code)
	var list []models.Backup
	require.NoError(t, json.Unmarshal(listW.Body.Bytes(), &list))
	found := false
	for _, b := range list {
		if b.ID == created.ID {
			found = true
			break
		}
	}
	assert.True(t, found, "created backup %s should appear in list", created.ID)
}

// TestDownloadBackup verifies a completed backup can be downloaded and the
// response body is non-empty (the streamed DB file).
func TestDownloadBackup(t *testing.T) {
	st, srv := newFileBackedServer(t)

	createReq := httptest.NewRequest(http.MethodPost, "/api/backups", nil)
	addTestAuth(t, st, createReq)
	createW := httptest.NewRecorder()
	srv.ServeHTTP(createW, createReq)
	require.Equal(t, http.StatusCreated, createW.Code, "body: %s", createW.Body.String())
	var created models.Backup
	require.NoError(t, json.Unmarshal(createW.Body.Bytes(), &created))

	downloadReq := httptest.NewRequest(http.MethodGet, "/api/backups/"+created.ID+"/download", nil)
	addTestAuth(t, st, downloadReq)
	downloadW := httptest.NewRecorder()
	srv.ServeHTTP(downloadW, downloadReq)

	require.Equal(t, http.StatusOK, downloadW.Code)
	assert.NotEmpty(t, downloadW.Body.Bytes(), "downloaded backup body should not be empty")
	assert.Equal(t, "application/octet-stream", downloadW.Header().Get("Content-Type"))
}

// TestDeleteBackup verifies a backup can be deleted (200, per handlers.go's
// Delete) and no longer appears in the list afterward.
func TestDeleteBackup(t *testing.T) {
	st, srv := newFileBackedServer(t)

	createReq := httptest.NewRequest(http.MethodPost, "/api/backups", nil)
	addTestAuth(t, st, createReq)
	createW := httptest.NewRecorder()
	srv.ServeHTTP(createW, createReq)
	require.Equal(t, http.StatusCreated, createW.Code, "body: %s", createW.Body.String())
	var created models.Backup
	require.NoError(t, json.Unmarshal(createW.Body.Bytes(), &created))

	delReq := httptest.NewRequest(http.MethodDelete, "/api/backups/"+created.ID, nil)
	addTestAuth(t, st, delReq)
	delW := httptest.NewRecorder()
	srv.ServeHTTP(delW, delReq)
	require.Equal(t, http.StatusOK, delW.Code, "body: %s", delW.Body.String())

	listReq := httptest.NewRequest(http.MethodGet, "/api/backups", nil)
	addTestAuth(t, st, listReq)
	listW := httptest.NewRecorder()
	srv.ServeHTTP(listW, listReq)
	require.Equal(t, http.StatusOK, listW.Code)
	var list []models.Backup
	require.NoError(t, json.Unmarshal(listW.Body.Bytes(), &list))
	for _, b := range list {
		assert.NotEqual(t, created.ID, b.ID, "deleted backup %s should no longer appear in list", created.ID)
	}
}

// TestUploadRestore_BadSignature verifies uploading a garbage (non-SQLite)
// "backup" file is rejected with a 4xx status and, critically, that the live
// database is left intact — a bad-signature restore must never clobber it.
func TestUploadRestore_BadSignature(t *testing.T) {
	st, srv := newFileBackedServer(t)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("confirmation", "CONFIRM RESTORE"))
	require.NoError(t, writer.WriteField("signature", "not-a-real-signature"))
	part, err := writer.CreateFormFile("file", "malicious.db")
	require.NoError(t, err)
	_, err = part.Write([]byte("this is not a sqlite database, just garbage bytes"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/backups/upload-restore", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	addTestAuth(t, st, req)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.GreaterOrEqual(t, w.Code, 400, "bad-signature restore should be rejected, got %d: %s", w.Code, w.Body.String())
	assert.Less(t, w.Code, 500, "bad-signature restore should be a client error, got %d: %s", w.Code, w.Body.String())

	// Prove the live DB was not clobbered: it must still accept ordinary writes.
	_, err = st.CreateWebhookConfig("https://example.com/hook", "post-restore-check", "run.completed")
	assert.NoError(t, err, "live store should still be usable after a rejected restore upload")
}
