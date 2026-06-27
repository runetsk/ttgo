package runs_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
	api "ttgo/internal/api"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

func TestUploadScreenshotsReplacesExistingGalleryAndTouchesRun(t *testing.T) {
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

	secondURLs := upload("a.png", "b.png")
	require.Len(t, secondURLs, 2)

	var updated models.RunResult
	require.NoError(t, s.DB().First(&updated, "id = ?", result.ID).Error)
	require.JSONEq(t, `["/api/uploads/screenshots/`+result.ID+`/step_001.png","/api/uploads/screenshots/`+result.ID+`/step_002.png"]`, updated.Screenshots)
	require.True(t, updated.UpdatedAt.After(result.UpdatedAt))

	refreshedRun, err := s.GetTestRun(run.ID)
	require.NoError(t, err)
	require.NotNil(t, refreshedRun)
	require.True(t, refreshedRun.UpdatedAt.After(initialUpdatedAt))

	require.FileExists(t, filepath.Join(tmpDir, "uploads", "screenshots", result.ID, "step_001.png"))
	require.FileExists(t, filepath.Join(tmpDir, "uploads", "screenshots", result.ID, "step_002.png"))
	require.NoFileExists(t, filepath.Join(tmpDir, "uploads", "screenshots", result.ID, "step_003.png"))
}
