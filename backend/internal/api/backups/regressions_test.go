package backups_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"ttgo/internal/api/backups"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"

	"github.com/stretchr/testify/require"
)

func createReviewBackup(t *testing.T, m *backups.Manager) models.Backup {
	t.Helper()
	w := httptest.NewRecorder()
	m.Create(w, httptest.NewRequest("POST", "/api/backups", nil))
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var backup models.Backup
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &backup))
	return backup
}

func TestBackupConfiguredDatabasePath(t *testing.T) {
	chdirTemp(t)
	s, err := store.New("custom.db")
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	folder, err := s.CreateFolder("before backup", nil)
	require.NoError(t, err)
	m := backups.NewManager(s, nil)
	backup := createReviewBackup(t, m)
	require.NoError(t, s.DB().Model(folder).Update("name", "after backup").Error)
	r := httptest.NewRequest("POST", "/api/backups/"+backup.ID+"/restore", bytes.NewBufferString(`{"confirmation":"CONFIRM RESTORE"}`))
	r.SetPathValue("id", backup.ID)
	w := httptest.NewRecorder()
	m.Restore(w, r)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	fresh, err := s.GetFolder(folder.ID)
	require.NoError(t, err)
	require.Equal(t, "before backup", fresh.Name)
	_, err = os.Stat("tracker.db")
	require.True(t, os.IsNotExist(err), "restore must not create a different database")
}

func uploadReviewBackup(t *testing.T, m *backups.Manager, data []byte, signature string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	f, err := mw.CreateFormFile("file", "backup.zip")
	require.NoError(t, err)
	_, err = f.Write(data)
	require.NoError(t, err)
	require.NoError(t, mw.WriteField("confirmation", "CONFIRM RESTORE"))
	if signature != "" {
		require.NoError(t, mw.WriteField("signature", signature))
	}
	require.NoError(t, mw.Close())
	r := httptest.NewRequest("POST", "/api/backups/upload-restore", &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	m.UploadRestore(w, r)
	return w
}

func TestSignedBackupArchiveRoundTrip(t *testing.T) {
	st, _ := newFileBackedServer(t)
	m := backups.NewManager(st, nil)
	folder, err := st.CreateFolder("saved folder", nil)
	require.NoError(t, err)
	backup := createReviewBackup(t, m)
	r := httptest.NewRequest("GET", "/api/backups/"+backup.ID+"/download?format=archive", nil)
	r.SetPathValue("id", backup.ID)
	w := httptest.NewRecorder()
	m.Download(w, r)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	zr, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	require.NoError(t, err, "download must include the signature in a ZIP")
	entries := map[string][]byte{}
	for _, f := range zr.File {
		in, err := f.Open()
		require.NoError(t, err)
		entries[f.Name], err = io.ReadAll(in)
		require.NoError(t, err)
		require.NoError(t, in.Close())
	}
	require.True(t, st.VerifyFileBytes(entries["backup.db"], string(entries["signature.txt"])))
	require.NoError(t, st.DB().Model(folder).Update("name", "changed").Error)
	upload := uploadReviewBackup(t, m, w.Body.Bytes(), "")
	require.Equal(t, http.StatusOK, upload.Code, upload.Body.String())
	fresh, err := st.GetFolder(folder.ID)
	require.NoError(t, err)
	require.Equal(t, "saved folder", fresh.Name)
	legacy := uploadReviewBackup(t, m, entries["backup.db"], string(entries["signature.txt"]))
	require.Equal(t, http.StatusOK, legacy.Code, "raw database uploads with an explicit signature remain supported: %s", legacy.Body.String())

	// Repack the real DB with a bad signature: authentication must still fail closed.
	var tampered bytes.Buffer
	zw := zip.NewWriter(&tampered)
	for _, name := range []string{"backup.db", "signature.txt"} {
		out, err := zw.Create(name)
		require.NoError(t, err)
		value := entries[name]
		if name == "signature.txt" {
			value = []byte("invalid-signature")
		}
		_, err = out.Write(value)
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	bad := uploadReviewBackup(t, m, tampered.Bytes(), "")
	require.Equal(t, http.StatusBadRequest, bad.Code)
	require.NoError(t, st.DB().Model(folder).Update("name", "still writable").Error)
}
