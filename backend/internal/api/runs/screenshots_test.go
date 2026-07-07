package runs_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
	api "ttgo/internal/api"
	"ttgo/pkg/tracker/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUploadScreenshotsAppendsToExistingGalleryAndTouchesRun(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := api.NewServer(s)

	origWD, err := os.Getwd()
	require.NoError(t, err)
	tmpDir := t.TempDir()
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() {
		_ = os.Chdir(origWD)
	})

	run := &models.TestRun{Name: "Upload Run"}
	require.NoError(t, s.CreateTestRun(run))
	result := &models.RunResult{
		TestRunID:        run.ID,
		TestNameSnapshot: "Checkout flow",
		Status:           models.StatusFail,
	}
	require.NoError(t, s.AddRunResult(result))

	initialRun, err := s.GetTestRun(run.ID)
	require.NoError(t, err)
	require.NotNil(t, initialRun)
	initialUpdatedAt := initialRun.UpdatedAt

	upload := func(names ...string) []string {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		for _, name := range names {
			part, err := writer.CreateFormFile("screenshots", name)
			require.NoError(t, err)
			// Valid PNG signature so the content-type sniff (F-020) accepts it.
			_, err = part.Write(append([]byte("\x89PNG\r\n\x1a\n"), []byte("body for "+name)...))
			require.NoError(t, err)
		}
		require.NoError(t, writer.Close())

		req := httptest.NewRequest(http.MethodPost, "/api/runs/"+run.ID+"/results/"+result.ID+"/screenshots", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		addTestAuth(t, s, req)

		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

		var resp struct {
			Screenshots []string `json:"screenshots"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		return resp.Screenshots
	}

	require.Len(t, upload("a.png", "b.png", "c.png"), 3)

	time.Sleep(10 * time.Millisecond)

	// A second upload appends to the existing gallery rather than replacing it:
	// the response is the full merged set and the earlier files survive.
	secondURLs := upload("d.png", "e.png")
	require.Len(t, secondURLs, 5)

	var updated models.RunResult
	require.NoError(t, s.DB().First(&updated, "id = ?", result.ID).Error)
	require.JSONEq(t, `[`+
		`"/api/uploads/screenshots/`+result.ID+`/step_001.png",`+
		`"/api/uploads/screenshots/`+result.ID+`/step_002.png",`+
		`"/api/uploads/screenshots/`+result.ID+`/step_003.png",`+
		`"/api/uploads/screenshots/`+result.ID+`/step_004.png",`+
		`"/api/uploads/screenshots/`+result.ID+`/step_005.png"]`, updated.Screenshots)
	require.True(t, updated.UpdatedAt.After(result.UpdatedAt))

	refreshedRun, err := s.GetTestRun(run.ID)
	require.NoError(t, err)
	require.NotNil(t, refreshedRun)
	require.True(t, refreshedRun.UpdatedAt.After(initialUpdatedAt))

	for i := 1; i <= 5; i++ {
		require.FileExists(t, filepath.Join(tmpDir, "uploads", "screenshots", result.ID, fmt.Sprintf("step_%03d.png", i)))
	}
	require.NoFileExists(t, filepath.Join(tmpDir, "uploads", "screenshots", result.ID, "step_006.png"))
}

// TestServeScreenshot_RejectsNonUUIDResultID: result IDs are always UUIDs, so a
// non-UUID result_id segment must be rejected outright with 400 (path traversal
// defense), not fall through to a 404 "not found" on the filesystem lookup.
func TestServeScreenshot_RejectsNonUUIDResultID(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := api.NewServer(s)

	req := httptest.NewRequest(http.MethodGet, "/api/uploads/screenshots/notauuid/foo.png", nil)
	addTestAuth(t, s, req)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// TestServeScreenshot_RejectsBadFilename: filenames are always server-generated
// (e.g. "step_001.png"), so anything containing a path separator or otherwise
// not matching the expected shape must be rejected with 400.
func TestServeScreenshot_RejectsBadFilename(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := api.NewServer(s)

	id := uuid.New().String()
	req := httptest.NewRequest(http.MethodGet, "/api/uploads/screenshots/"+id+"/bad%2Ffile.png", nil)
	addTestAuth(t, s, req)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// TestServeScreenshot_ValidUUIDAndFilenamePassesValidation: a well-formed UUID
// result_id and a real generated filename shape should pass validation and reach
// the filesystem lookup — 404 (no such file) rather than 400 (invalid path).
func TestServeScreenshot_ValidUUIDAndFilenamePassesValidation(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := api.NewServer(s)

	id := uuid.New().String()
	req := httptest.NewRequest(http.MethodGet, "/api/uploads/screenshots/"+id+"/step_001.png", nil)
	addTestAuth(t, s, req)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}
