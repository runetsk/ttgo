package backups

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
	"ttgo/internal/api/authctx"
	"ttgo/internal/api/httpx"
	apiws "ttgo/internal/api/websocket"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"

	"github.com/google/uuid"
)

type Manager struct {
	store           *store.Store
	hub             *apiws.Hub
	backupMu        sync.Mutex
	maintenanceMu   sync.RWMutex
	maintenanceMode bool
}

func NewManager(s *store.Store, hub *apiws.Hub) *Manager {
	return &Manager{
		store: s,
		hub:   hub,
	}
}

// ── Shared helpers ──────────────────────────────────────────────────────────

// performBackup creates a database backup of the given type.
// It runs WAL checkpoint, copies the DB file, and records metadata.
// Returns the completed Backup record or an error.
func (m *Manager) performBackup(backupType, creatorID, creatorName string) (*models.Backup, error) {
	id := uuid.New().String()
	filePath := filepath.Join("backups", fmt.Sprintf("backup-%s.db", id))

	// Create in-progress record
	backup, err := m.store.CreateBackup(id, backupType, filePath, creatorID, creatorName)
	if err != nil {
		return nil, fmt.Errorf("failed to create backup record: %w", err)
	}

	// Check available disk space before copying
	srcInfo, err := os.Stat("tracker.db")
	if err != nil {
		_ = m.store.FailBackup(id, fmt.Sprintf("cannot stat database: %v", err))
		return nil, fmt.Errorf("cannot stat database: %w", err)
	}
	if err := store.CheckDiskSpace("backups", srcInfo.Size()); err != nil {
		_ = m.store.FailBackup(id, err.Error())
		return nil, err
	}

	// WAL checkpoint to ensure DB file is self-contained
	if err := m.store.DB().Exec("PRAGMA wal_checkpoint(TRUNCATE)").Error; err != nil {
		_ = m.store.FailBackup(id, fmt.Sprintf("WAL checkpoint failed: %v", err))
		return nil, fmt.Errorf("WAL checkpoint failed: %w", err)
	}

	// Copy the database file
	srcPath := "tracker.db"
	fileSize, err := store.CopyFile(srcPath, filePath)
	if err != nil {
		_ = m.store.FailBackup(id, fmt.Sprintf("file copy failed: %v", err))
		return nil, fmt.Errorf("failed to copy database: %w", err)
	}

	// Mark completed
	if err := m.store.CompleteBackup(id, fileSize, filePath); err != nil {
		return nil, fmt.Errorf("failed to complete backup record: %w", err)
	}

	// Re-fetch to get updated record
	backup, err = m.store.GetBackup(id)
	if err != nil {
		return nil, err
	}

	// Sign the file so it can be integrity-checked, and so only backups produced
	// by this server (same key) can be installed via upload-restore (F-017).
	if sig, serr := m.store.SignFile(filePath); serr == nil && sig != "" {
		if m.store.SetBackupSignature(id, sig) == nil {
			backup.Signature = sig
		}
	}

	// Global retention: cap total backups (all types) to bound disk growth from
	// repeated manual/pre-restore backups (F-041).
	const maxTotalBackups = 50
	if prunedIDs, perr := m.store.PruneBackupsBeyond(maxTotalBackups); perr == nil {
		for _, delID := range prunedIDs {
			_ = os.Remove(backupFilePath(delID))
		}
	} else {
		slog.Warn("backup: global retention prune failed", "error", perr)
	}

	return backup, nil
}

// ── Backup CRUD handlers ────────────────────────────────────────────────────

// handleCreateBackup initiates a manual on-demand backup.
//
// @Summary      Create backup
// @Description  Initiate a manual on-demand database backup. Only one backup/restore operation may run at a time.
// @Tags         backups
// @Produce      json
// @Success      201  {object}  models.Backup
// @Failure      409  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /backups [post]
// @Security     BearerAuth
func (m *Manager) Create(w http.ResponseWriter, r *http.Request) {
	if !m.backupMu.TryLock() {
		httpx.JSON(w, http.StatusConflict, map[string]string{"error": "backup or restore operation already in progress"})
		return
	}
	defer m.backupMu.Unlock()

	user := authctx.UserFromRequest(r)
	creatorID, creatorName := "", ""
	if user != nil {
		creatorID = user.ID
		creatorName = user.DisplayName
	}

	backup, err := m.performBackup("manual", creatorID, creatorName)
	if err != nil {
		slog.ErrorContext(r.Context(), "backup: create failed", "error", err)
		// Log audit event for failure
		m.logBackupEvent(creatorID, fmt.Sprintf("backup:failed:%s", "unknown"), fmt.Sprintf("error=%v", err))
		httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("backup failed: %v", err)})
		return
	}

	m.logBackupEvent(creatorID, fmt.Sprintf("backup:created:%s", backup.ID), fmt.Sprintf("type=manual, size=%d", backup.FileSize))
	slog.InfoContext(r.Context(), "backup: manual backup created", "id", backup.ID, "size", backup.FileSize)

	// 018-websocket-realtime: broadcast backup created
	if m.hub != nil {
		m.hub.Broadcast(apiws.NewEvent(apiws.EventBackupCreated, "backups:*", backup))
	}

	httpx.JSON(w, http.StatusCreated, backup)
}

// handleGetBackup returns metadata for a single backup.
//
// @Summary      Get backup
// @Description  Return metadata for a single backup by ID.
// @Tags         backups
// @Produce      json
// @Param        id  path  string  true  "Backup ID"
// @Success      200  {object}  models.Backup
// @Failure      404  {object}  map[string]string
// @Router       /backups/{id} [get]
// @Security     BearerAuth
func (m *Manager) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	backup, err := m.store.GetBackup(id)
	if err != nil {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "backup not found"})
		return
	}
	httpx.JSON(w, http.StatusOK, backup)
}

// handleListBackups returns all backup records ordered by created_at DESC.
//
// @Summary      List backups
// @Description  Return all backup records ordered by creation date descending.
// @Tags         backups
// @Produce      json
// @Success      200  {array}  models.Backup
// @Failure      500  {object}  map[string]string
// @Router       /backups [get]
// @Security     BearerAuth
func (m *Manager) List(w http.ResponseWriter, r *http.Request) {
	backups, err := m.store.ListBackups()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, backups)
}

// handleDeleteBackup deletes a backup record and its file.
//
// @Summary      Delete backup
// @Description  Delete a backup record and remove its file from disk.
// @Tags         backups
// @Produce      json
// @Param        id  path  string  true  "Backup ID"
// @Success      200  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /backups/{id} [delete]
// @Security     BearerAuth
func (m *Manager) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	backup, err := m.store.GetBackup(id)
	if err != nil {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "backup not found"})
		return
	}

	// Recompute the path from the (DB-key) id rather than trusting the stored
	// FilePath, so a crafted backups row cannot point os.Remove at an arbitrary
	// file (F-065).
	path := backupFilePath(id)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		slog.WarnContext(r.Context(), "backup: failed to remove file", "path", path, "error", err)
	}

	if err := m.store.DeleteBackup(id); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	user := authctx.UserFromRequest(r)
	uid := ""
	if user != nil {
		uid = user.ID
	}
	m.logBackupEvent(uid, fmt.Sprintf("backup:deleted:%s", id), fmt.Sprintf("type=%s, created_at=%s", backup.Type, backup.CreatedAt.Format(time.RFC3339)))

	// 018-websocket-realtime: broadcast backup deleted
	if m.hub != nil {
		m.hub.Broadcast(apiws.NewEvent(apiws.EventBackupDeleted, "backups:*", map[string]string{"id": id}))
	}

	httpx.JSON(w, http.StatusOK, map[string]string{"message": "backup deleted"})
}

// handleDownloadBackup streams a backup file as a binary download.
//
// @Summary      Download backup
// @Description  Stream a completed backup file as a binary download.
// @Tags         backups
// @Produce      octet-stream
// @Param        id  path  string  true  "Backup ID"
// @Success      200  {file}  binary
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /backups/{id}/download [get]
// @Security     BearerAuth
func (m *Manager) Download(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	backup, err := m.store.GetBackup(id)
	if err != nil {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "backup not found"})
		return
	}
	if backup.Status != "completed" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "backup is not completed"})
		return
	}

	// Audit the download — a backup is a full DB copy (bcrypt hashes + secrets),
	// so its exfiltration must leave a trail (F-016).
	user := authctx.UserFromRequest(r)
	uid := ""
	if user != nil {
		uid = user.ID
	}
	m.logBackupEvent(uid, fmt.Sprintf("backup:downloaded:%s", id), fmt.Sprintf("type=%s", backup.Type))

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="backup-%s.db"`, id))
	// Serve from the id-derived path, not the stored FilePath (F-065).
	http.ServeFile(w, r, backupFilePath(id))
}

// backupFilePath returns the canonical on-disk path for a backup id. Deriving it
// from the DB-key id (never a stored FilePath) prevents a crafted backups row
// from redirecting file operations to an arbitrary location (F-065).
func backupFilePath(id string) string {
	return filepath.Join("backups", fmt.Sprintf("backup-%s.db", id))
}

// ── Restore handlers ────────────────────────────────────────────────────────

// handleRestoreBackup restores the database from a server-stored backup.
//
// @Summary      Restore from backup
// @Description  Restore the database from a server-stored backup. Requires confirmation string "CONFIRM RESTORE". Creates a pre-restore safety backup and enters maintenance mode during the operation.
// @Tags         backups
// @Accept       json
// @Produce      json
// @Param        id    path  string                           true  "Backup ID"
// @Param        body  body  object{confirmation=string}      true  "Confirmation payload"
// @Success      200  {object}  object{message=string,pre_restore_backup_id=string}
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      409  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /backups/{id}/restore [post]
// @Security     BearerAuth
func (m *Manager) Restore(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		Confirmation string `json:"confirmation"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Confirmation != "CONFIRM RESTORE" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "confirmation must be exactly 'CONFIRM RESTORE'"})
		return
	}

	if !m.backupMu.TryLock() {
		httpx.JSON(w, http.StatusConflict, map[string]string{"error": "backup or restore operation already in progress"})
		return
	}
	defer m.backupMu.Unlock()

	backup, err := m.store.GetBackup(id)
	if err != nil || backup.Status != "completed" {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "completed backup not found"})
		return
	}

	user := authctx.UserFromRequest(r)
	uid, uname := "", ""
	if user != nil {
		uid = user.ID
		uname = user.DisplayName
	}

	// Log restore started
	m.logBackupEvent(uid, fmt.Sprintf("restore:started:%s", id), fmt.Sprintf("source=server, backup_id=%s", id))

	// Create pre-restore safety backup
	preRestoreBackup, err := m.performBackup("pre-restore", uid, uname)
	if err != nil {
		m.logBackupEvent(uid, fmt.Sprintf("restore:failed:%s", id), fmt.Sprintf("error=pre-restore backup failed: %v", err))
		httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("pre-restore backup failed: %v", err)})
		return
	}

	// Enter maintenance mode
	m.setMaintenanceMode(true)
	defer m.setMaintenanceMode(false)

	// 018-websocket-realtime: broadcast maintenance started
	if m.hub != nil {
		m.hub.Broadcast(apiws.NewEvent(apiws.EventMaintenanceChanged, "backups:*", map[string]interface{}{"maintenance": true}))
	}

	// Integrity-check the on-disk backup against its stored signature (F-017),
	// from the id-derived path rather than the stored FilePath (F-065). Fail closed:
	// an empty signature (VerifyFile returns false) or a mismatch is rejected — all
	// real backups are signed on creation and legacy ones are signed at startup.
	srcPath := backupFilePath(id)
	if ok, _ := m.store.VerifyFile(srcPath, backup.Signature); !ok {
		m.logBackupEvent(uid, fmt.Sprintf("restore:failed:%s", id), "error=signature verification failed or missing")
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "backup integrity check failed (unsigned or tampered)"})
		return
	}

	// Copy backup file over tracker.db
	if _, err := store.CopyFile(srcPath, "tracker.db"); err != nil {
		m.logBackupEvent(uid, fmt.Sprintf("restore:failed:%s", id), fmt.Sprintf("error=file copy failed: %v", err))
		httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("restore failed: %v", err)})
		return
	}

	// Reopen database connection
	if err := m.store.ReopenDB("tracker.db"); err != nil {
		m.logBackupEvent(uid, fmt.Sprintf("restore:failed:%s", id), fmt.Sprintf("error=reopen failed: %v", err))
		httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("database reopen failed: %v", err)})
		return
	}

	m.logBackupEvent(uid, fmt.Sprintf("restore:completed:%s", id), "")
	slog.InfoContext(r.Context(), "backup: restore completed", "backup_id", id, "pre_restore_id", preRestoreBackup.ID)

	// 018-websocket-realtime: broadcast restore completed and maintenance ended
	if m.hub != nil {
		m.hub.Broadcast(apiws.NewEvent(apiws.EventBackupRestored, "backups:*", map[string]interface{}{
			"pre_restore_backup_id": preRestoreBackup.ID,
		}))
		m.hub.Broadcast(apiws.NewEvent(apiws.EventMaintenanceChanged, "backups:*", map[string]interface{}{"maintenance": false}))
	}

	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"message":               "Database restored successfully",
		"pre_restore_backup_id": preRestoreBackup.ID,
	})
}

// handleUploadRestore restores the database from an uploaded file.
//
// @Summary      Upload and restore backup
// @Description  Upload a backup file and restore the database from it. Requires confirmation "CONFIRM RESTORE" as a form field. Max upload size 200 MB.
// @Tags         backups
// @Accept       multipart/form-data
// @Produce      json
// @Param        file          formData  file    true  "Backup database file"
// @Param        confirmation  formData  string  true  "Must be 'CONFIRM RESTORE'"
// @Success      200  {object}  object{message=string,pre_restore_backup_id=string}
// @Failure      400  {object}  map[string]string
// @Failure      409  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /backups/upload-restore [post]
// @Security     BearerAuth
func (m *Manager) UploadRestore(w http.ResponseWriter, r *http.Request) {
	// Multipart bodies bypass the global 5 MB cap, so hard-limit the total upload
	// to bound memory/disk use from an oversized request (F-022).
	r.Body = http.MaxBytesReader(w, r.Body, 256<<20)
	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32 MB in-memory, rest spills to disk
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "failed to parse multipart form (too large?)"})
		return
	}

	confirmation := r.FormValue("confirmation")
	if confirmation != "CONFIRM RESTORE" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "confirmation must be exactly 'CONFIRM RESTORE'"})
		return
	}

	file, handler, err := r.FormFile("file")
	if err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "missing file in upload"})
		return
	}
	defer file.Close()

	// Save to temp file for validation
	tempPath := filepath.Join("backups", fmt.Sprintf("upload-%s.db", uuid.New().String()))
	out, err := os.Create(tempPath)
	if err != nil {
		httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save uploaded file"})
		return
	}
	if _, err := io.Copy(out, file); err != nil {
		out.Close()
		os.Remove(tempPath)
		httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save uploaded file"})
		return
	}
	out.Close()

	// Validate the uploaded file
	if err := store.ValidateBackupFile(tempPath); err != nil {
		os.Remove(tempPath)
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid backup file: %v", err)})
		return
	}

	// Require a valid HMAC signature: only a backup produced by THIS server (same
	// key) verifies, which blocks installing an attacker-crafted DB (e.g. one
	// carrying a known-password admin row) via upload-restore (F-017).
	providedSig := r.FormValue("signature")
	tempBytes, rerr := os.ReadFile(tempPath)
	if rerr != nil || providedSig == "" || !m.store.VerifyFileBytes(tempBytes, providedSig) {
		os.Remove(tempPath)
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "backup signature missing or invalid — only backups created by this server can be restored"})
		return
	}

	if !m.backupMu.TryLock() {
		os.Remove(tempPath)
		httpx.JSON(w, http.StatusConflict, map[string]string{"error": "backup or restore operation already in progress"})
		return
	}
	defer m.backupMu.Unlock()

	user := authctx.UserFromRequest(r)
	uid, uname := "", ""
	if user != nil {
		uid = user.ID
		uname = user.DisplayName
	}

	m.logBackupEvent(uid, "restore:started:upload", fmt.Sprintf("source=upload, filename=%s", handler.Filename))

	// Create pre-restore safety backup
	preRestoreBackup, err := m.performBackup("pre-restore", uid, uname)
	if err != nil {
		os.Remove(tempPath)
		m.logBackupEvent(uid, "restore:failed:upload", fmt.Sprintf("error=pre-restore backup failed: %v", err))
		httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("pre-restore backup failed: %v", err)})
		return
	}

	// Enter maintenance mode
	m.setMaintenanceMode(true)
	defer m.setMaintenanceMode(false)

	// 018-websocket-realtime: broadcast maintenance started
	if m.hub != nil {
		m.hub.Broadcast(apiws.NewEvent(apiws.EventMaintenanceChanged, "backups:*", map[string]interface{}{"maintenance": true}))
	}

	// Copy uploaded file over tracker.db
	if _, err := store.CopyFile(tempPath, "tracker.db"); err != nil {
		os.Remove(tempPath)
		m.logBackupEvent(uid, "restore:failed:upload", fmt.Sprintf("error=file copy failed: %v", err))
		httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("restore failed: %v", err)})
		return
	}

	// Clean up temp file
	os.Remove(tempPath)

	// Reopen database connection
	if err := m.store.ReopenDB("tracker.db"); err != nil {
		m.logBackupEvent(uid, "restore:failed:upload", fmt.Sprintf("error=reopen failed: %v", err))
		httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("database reopen failed: %v", err)})
		return
	}

	m.logBackupEvent(uid, "restore:completed:upload", "")
	slog.InfoContext(r.Context(), "backup: restore completed from upload", "filename", handler.Filename, "pre_restore_id", preRestoreBackup.ID)

	// 018-websocket-realtime: broadcast restore completed and maintenance ended
	if m.hub != nil {
		m.hub.Broadcast(apiws.NewEvent(apiws.EventBackupRestored, "backups:*", map[string]interface{}{
			"pre_restore_backup_id": preRestoreBackup.ID,
		}))
		m.hub.Broadcast(apiws.NewEvent(apiws.EventMaintenanceChanged, "backups:*", map[string]interface{}{"maintenance": false}))
	}

	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"message":               "Database restored successfully",
		"pre_restore_backup_id": preRestoreBackup.ID,
	})
}

// ── Schedule handlers ───────────────────────────────────────────────────────

// handleGetBackupSchedule returns the current backup schedule configuration.
//
// @Summary      Get backup schedule
// @Description  Return the current automatic backup schedule configuration.
// @Tags         backups
// @Produce      json
// @Success      200  {object}  models.BackupSchedule
// @Failure      500  {object}  map[string]string
// @Router       /backup-schedule [get]
// @Security     BearerAuth
func (m *Manager) GetSchedule(w http.ResponseWriter, r *http.Request) {
	schedule, err := m.store.GetBackupSchedule()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, schedule)
}

// handleUpdateBackupSchedule updates the backup schedule configuration.
//
// @Summary      Update backup schedule
// @Description  Update the automatic backup schedule (enabled, interval, retention count).
// @Tags         backups
// @Accept       json
// @Produce      json
// @Param        body  body  object{enabled=bool,interval_hours=int,retention_count=int}  true  "Schedule settings"
// @Success      200  {object}  models.BackupSchedule
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /backup-schedule [put]
// @Security     BearerAuth
func (m *Manager) UpdateSchedule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled        *bool `json:"enabled"`
		IntervalHours  *int  `json:"interval_hours"`
		RetentionCount *int  `json:"retention_count"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Load current schedule
	current, err := m.store.GetBackupSchedule()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	enabled := current.Enabled
	intervalHours := current.IntervalHours
	retentionCount := current.RetentionCount

	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if req.IntervalHours != nil {
		// Upper-bound to avoid time.Duration overflow / nonsensical schedules (F-064).
		if *req.IntervalHours < 1 || *req.IntervalHours > 8760 {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "interval_hours must be between 1 and 8760"})
			return
		}
		intervalHours = *req.IntervalHours
	}
	if req.RetentionCount != nil {
		if *req.RetentionCount < 1 || *req.RetentionCount > 1000 {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "retention_count must be between 1 and 1000"})
			return
		}
		retentionCount = *req.RetentionCount
	}

	schedule, err := m.store.UpdateBackupSchedule(enabled, intervalHours, retentionCount)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	user := authctx.UserFromRequest(r)
	uid := ""
	if user != nil {
		uid = user.ID
	}
	m.logBackupEvent(uid, "schedule:updated", fmt.Sprintf("enabled=%v, interval=%dh, retention=%d", enabled, intervalHours, retentionCount))

	// 018-websocket-realtime: broadcast schedule updated
	if m.hub != nil {
		m.hub.Broadcast(apiws.NewEvent(apiws.EventBackupScheduleUpdated, "backups:*", schedule))
	}

	httpx.JSON(w, http.StatusOK, schedule)
}

// ── Maintenance status ──────────────────────────────────────────────────────

// handleMaintenanceStatus returns whether the system is in maintenance mode.
//
// @Summary      Get maintenance status
// @Description  Check whether the system is currently in maintenance mode (e.g. during a restore operation). No authentication required.
// @Tags         backups
// @Produce      json
// @Success      200  {object}  object{maintenance=bool,message=string}
// @Router       /maintenance-status [get]
func (m *Manager) MaintenanceStatus(w http.ResponseWriter, r *http.Request) {
	// This endpoint is public, so return only a bare boolean — no descriptive
	// message that would leak operational detail to unauthenticated clients (F-063).
	httpx.JSON(w, http.StatusOK, map[string]bool{"maintenance": m.IsInMaintenanceMode()})
}

// ── Maintenance mode helpers ────────────────────────────────────────────────

func (m *Manager) setMaintenanceMode(on bool) {
	m.maintenanceMu.Lock()
	m.maintenanceMode = on
	m.maintenanceMu.Unlock()
}

func (m *Manager) IsInMaintenanceMode() bool {
	m.maintenanceMu.RLock()
	defer m.maintenanceMu.RUnlock()
	return m.maintenanceMode
}

// ── Scheduler ───────────────────────────────────────────────────────────────

// StartBackupScheduler starts a background goroutine that checks the backup
// schedule every minute and triggers automatic backups when due.
func (m *Manager) StartScheduler(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				slog.Info("backup: scheduler stopped")
				return
			case <-ticker.C:
				m.checkAndRunScheduledBackup()
			}
		}
	}()
	slog.Info("backup: scheduler started", "check_interval", "1m")
}

func (m *Manager) checkAndRunScheduledBackup() {
	schedule, err := m.store.GetBackupSchedule()
	if err != nil {
		slog.Error("backup: scheduler failed to load schedule", "error", err)
		return
	}

	if !schedule.Enabled || schedule.NextRunAt == nil || time.Now().Before(*schedule.NextRunAt) {
		return
	}

	if !m.backupMu.TryLock() {
		slog.Warn("backup: scheduler skipped, another operation in progress")
		return
	}
	defer m.backupMu.Unlock()

	slog.Info("backup: scheduler triggering automatic backup")
	backup, err := m.performBackup("automatic", "", "system")
	if err != nil {
		slog.Error("backup: scheduled backup failed", "error", err)
		m.logBackupEvent("", "backup:failed:scheduled", fmt.Sprintf("error=%v", err))
		return
	}

	m.logBackupEvent("", fmt.Sprintf("backup:created:%s", backup.ID), fmt.Sprintf("type=automatic, size=%d", backup.FileSize))

	// Update schedule timestamps
	now := time.Now()
	nextRun := now.Add(time.Duration(schedule.IntervalHours) * time.Hour)
	m.store.DB().Model(&models.BackupSchedule{}).Where("id = ?", "default").Updates(map[string]interface{}{
		"last_run_at": now,
		"next_run_at": nextRun,
	})

	// Enforce retention policy
	deletedIDs, err := m.store.DeleteOldestAutomaticBackups(schedule.RetentionCount)
	if err != nil {
		slog.Error("backup: retention cleanup failed", "error", err)
		return
	}
	for _, delID := range deletedIDs {
		fpath := filepath.Join("backups", fmt.Sprintf("backup-%s.db", delID))
		if err := os.Remove(fpath); err != nil && !os.IsNotExist(err) {
			slog.Warn("backup: failed to remove old backup file", "path", fpath, "error", err)
		}
	}
	if len(deletedIDs) > 0 {
		slog.Info("backup: retention cleanup removed old automatic backups", "count", len(deletedIDs))
	}
}

// ── Audit logging helper ────────────────────────────────────────────────────

func (m *Manager) logBackupEvent(userID, action, diff string) {
	entry := &models.AuditLog{
		ID:        uuid.New().String(),
		UserID:    userID,
		Action:    action,
		Diff:      diff,
		Timestamp: time.Now(),
	}
	if err := m.store.CreateAuditLog(entry); err != nil {
		slog.Error("backup: failed to create audit log", "error", err)
	}
}
