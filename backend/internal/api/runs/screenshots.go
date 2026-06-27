package runs

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"ttgo/internal/api/httpx"
	apiws "ttgo/internal/api/websocket"

	"ttgo/pkg/tracker/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const screenshotUploadLimit = 50 << 20 // 50 MB total per upload

// handleUploadScreenshots handles POST /runs/{id}/results/{result_id}/screenshots
//
// @Summary      Upload screenshots
// @Description  Upload one or more screenshot files for a run result. Accepts multipart/form-data with files under "screenshots". Max 50 MB total.
// @Tags         runs
// @Accept       multipart/form-data
// @Produce      json
// @Param        id         path      string  true  "Test run ID"
// @Param        result_id  path      string  true  "Run result ID"
// @Param        screenshots  formData  file  true  "Screenshot files"
// @Success      201  {object}  object{screenshots=[]string}
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /runs/{id}/results/{result_id}/screenshots [post]
// @Security     BearerAuth
func (h *Handler) UploadScreenshots(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")
	resultID := r.PathValue("result_id")

	// Verify result exists and belongs to this run
	var result models.RunResult
	if err := h.store.DB().Select("id").
		Where("id = ? AND test_run_id = ?", resultID, runID).
		First(&result).Error; err != nil {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "result not found"})
		return
	}

	if err := r.ParseMultipartForm(screenshotUploadLimit); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "failed to parse multipart form: " + err.Error()})
		return
	}

	files := r.MultipartForm.File["screenshots"]
	if len(files) == 0 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "no screenshots provided"})
		return
	}

	parentDir := filepath.Join("uploads", "screenshots")
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		slog.ErrorContext(r.Context(), "failed to create screenshot parent directory", "dir", parentDir, "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	stageDir := filepath.Join(parentDir, resultID+".tmp-"+uuid.NewString())
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		slog.ErrorContext(r.Context(), "failed to create staging directory", "dir", stageDir, "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	defer os.RemoveAll(stageDir)

	const maxScreenshots = 50
	const maxScreenshotBytes = 10 << 20 // 10 MB per file
	if len(files) > maxScreenshots {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "too many screenshots (max 50)"})
		return
	}
	allowedExt := map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/gif": ".gif", "image/webp": ".webp"}

	var urls []string
	for i, fh := range files {
		if fh.Size > maxScreenshotBytes {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "screenshot too large (max 10 MB each)"})
			return
		}

		src, err := fh.Open()
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to open uploaded file", "filename", fh.Filename, "error", err)
			httpx.Error(w, http.StatusInternalServerError, err)
			return
		}

		// Sniff the real content type, reject non-images, and choose a safe
		// server-controlled extension rather than trusting the client filename (F-020).
		head := make([]byte, 512)
		n, _ := io.ReadFull(src, head)
		ext, ok := allowedExt[http.DetectContentType(head[:n])]
		if !ok {
			src.Close()
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported file type (PNG/JPEG/GIF/WebP only)"})
			return
		}
		if _, err := src.Seek(0, io.SeekStart); err != nil {
			src.Close()
			httpx.Error(w, http.StatusInternalServerError, err)
			return
		}

		filename := fmt.Sprintf("step_%03d%s", i+1, ext)
		destPath := filepath.Join(stageDir, filename)

		dst, err := os.Create(destPath)
		if err != nil {
			src.Close()
			slog.ErrorContext(r.Context(), "failed to create file", "path", destPath, "error", err)
			httpx.Error(w, http.StatusInternalServerError, err)
			return
		}

		if _, err := io.Copy(dst, src); err != nil {
			src.Close()
			dst.Close()
			slog.ErrorContext(r.Context(), "failed to write file", "path", destPath, "error", err)
			httpx.Error(w, http.StatusInternalServerError, err)
			return
		}
		src.Close()
		dst.Close()

		url := fmt.Sprintf("/api/uploads/screenshots/%s/%s", resultID, filename)
		urls = append(urls, url)
	}

	finalDir := filepath.Join(parentDir, resultID)
	backupDir := filepath.Join(parentDir, resultID+".bak-"+uuid.NewString())
	hadExistingDir := false
	if _, err := os.Stat(finalDir); err == nil {
		hadExistingDir = true
		if err := os.Rename(finalDir, backupDir); err != nil {
			slog.ErrorContext(r.Context(), "failed to rotate screenshot directory", "dir", finalDir, "error", err)
			httpx.Error(w, http.StatusInternalServerError, err)
			return
		}
	} else if !os.IsNotExist(err) {
		slog.ErrorContext(r.Context(), "failed to inspect screenshot directory", "dir", finalDir, "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	if err := os.Rename(stageDir, finalDir); err != nil {
		if hadExistingDir {
			_ = os.Rename(backupDir, finalDir)
		}
		slog.ErrorContext(r.Context(), "failed to activate screenshot directory", "dir", finalDir, "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	urlsJSON, _ := json.Marshal(urls)
	now := time.Now()
	if err := h.store.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.RunResult{}).
			Where("id = ? AND test_run_id = ?", resultID, runID).
			Updates(map[string]interface{}{
				"screenshots": string(urlsJSON),
				"updated_at":  now,
			}).Error; err != nil {
			return err
		}
		return tx.Model(&models.TestRun{}).
			Where("id = ?", runID).
			Update("updated_at", now).Error
	}); err != nil {
		_ = os.RemoveAll(finalDir)
		if hadExistingDir {
			_ = os.Rename(backupDir, finalDir)
		}
		slog.ErrorContext(r.Context(), "failed to update screenshots for result", "result_id", resultID, "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	_ = os.RemoveAll(backupDir)

	// Broadcast update
	if fullRun, err := h.store.GetTestRun(runID); err == nil && fullRun != nil && h.hub != nil {
		h.hub.Broadcast(apiws.NewEvent(apiws.EventResultUpdated, "run:"+runID, fullRun))
	}

	httpx.JSON(w, http.StatusCreated, map[string]interface{}{
		"screenshots": urls,
	})
}

// handleServeScreenshot handles GET /uploads/screenshots/{result_id}/{filename}
//
// @Summary      Serve screenshot
// @Description  Serve a screenshot file from disk by result ID and filename.
// @Tags         runs
// @Produce      image/png
// @Param        result_id  path  string  true  "Run result ID"
// @Param        filename   path  string  true  "Screenshot filename"
// @Success      200  {file}  binary
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /uploads/screenshots/{result_id}/{filename} [get]
// @Security     BearerAuth
func (h *Handler) ServeScreenshot(w http.ResponseWriter, r *http.Request) {
	resultID := r.PathValue("result_id")
	filename := r.PathValue("filename")

	// Sanitize to prevent path traversal
	if strings.Contains(resultID, "..") || strings.Contains(filename, "..") {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid path"})
		return
	}

	filePath := filepath.Join("uploads", "screenshots", resultID, filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "screenshot not found"})
		return
	}

	// Prevent the browser from MIME-sniffing a stored file into active content (F-020).
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, filePath)
}

// RemoveScreenshotDir removes the screenshot directory for a given result ID.
// Safe to call even if the directory does not exist.
func RemoveScreenshotDir(resultID string) {
	dir := filepath.Join("uploads", "screenshots", resultID)
	if err := os.RemoveAll(dir); err != nil {
		slog.Warn("failed to remove screenshot dir", "dir", dir, "error", err)
	}
}
