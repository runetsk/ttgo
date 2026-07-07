package runs

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"
	"ttgo/internal/api/httpx"
	apiws "ttgo/internal/api/websocket"

	"ttgo/pkg/tracker/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const screenshotUploadLimit = 50 << 20 // 50 MB total per upload

// Server-generated screenshot filenames: "step_NNN.ext" (see UploadScreenshots
// below) — allow the underscore alongside alphanumerics/hyphen so real files
// aren't rejected, while still excluding path separators, ".." and other
// traversal-relevant characters.
var screenshotFilenameRe = regexp.MustCompile(`^[A-Za-z0-9_-]+\.[A-Za-z0-9]{1,5}$`)

// copyFileInto copies srcPath to dstPath, truncating/creating the destination.
// Used to carry already-attached screenshots through the atomic staging swap
// so an upload appends to (rather than replaces) a result's gallery.
func copyFileInto(srcPath, dstPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}

// handleUploadScreenshots handles POST /runs/{id}/results/{result_id}/screenshots
//
// @Summary      Upload screenshots
// @Description  Append one or more screenshot files to a run result. Accepts multipart/form-data with files under "screenshots". Max 10 MB per file, 50 per result.
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

	// Verify result exists and belongs to this run; load its existing
	// screenshots so this upload appends rather than replaces them.
	var result models.RunResult
	if err := h.store.DB().Select("id", "screenshots").
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
	finalDir := filepath.Join(parentDir, resultID)

	stageDir := filepath.Join(parentDir, resultID+".tmp-"+uuid.NewString())
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		slog.ErrorContext(r.Context(), "failed to create staging directory", "dir", stageDir, "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	defer os.RemoveAll(stageDir)

	const maxScreenshots = 50
	const maxScreenshotBytes = 10 << 20 // 10 MB per file

	// Append semantics: copy any already-attached files into the staging dir so
	// the atomic swap below keeps them, then number new uploads after them. Their
	// URLs (plus any external ones already recorded) merge ahead of the new ones.
	var existingURLs []string
	if result.Screenshots != "" {
		_ = json.Unmarshal([]byte(result.Screenshots), &existingURLs)
	}
	existingFileCount := 0
	if entries, err := os.ReadDir(finalDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if err := copyFileInto(filepath.Join(finalDir, e.Name()), filepath.Join(stageDir, e.Name())); err != nil {
				slog.ErrorContext(r.Context(), "failed to stage existing screenshot", "name", e.Name(), "error", err)
				httpx.Error(w, http.StatusInternalServerError, err)
				return
			}
			existingFileCount++
		}
	} else if !os.IsNotExist(err) {
		slog.ErrorContext(r.Context(), "failed to read existing screenshot directory", "dir", finalDir, "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	if existingFileCount+len(files) > maxScreenshots {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "too many screenshots (max 50 per result)"})
		return
	}
	allowedExt := map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/gif": ".gif", "image/webp": ".webp"}

	var newURLs []string
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

		filename := fmt.Sprintf("step_%03d%s", existingFileCount+i+1, ext)
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
		newURLs = append(newURLs, url)
	}

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

	mergedURLs := append(existingURLs, newURLs...)
	urlsJSON, _ := json.Marshal(mergedURLs)
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
	h.broadcastResultDelta(apiws.EventResultUpdated, runID, []string{resultID}, nil, nil)

	httpx.JSON(w, http.StatusCreated, map[string]interface{}{
		"screenshots": mergedURLs,
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

	// Strict validation: result IDs are UUIDs and filenames are server-generated
	// (e.g. "step_001.png") — reject anything else outright (path traversal defense).
	if _, err := uuid.Parse(resultID); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid path"})
		return
	}
	if filename == "" || !screenshotFilenameRe.MatchString(filename) {
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
