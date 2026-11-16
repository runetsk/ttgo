package store

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
	"ttgo/pkg/tracker/models"

	"log/slog"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// DatabasePath asks SQLite for the main database filename, including when the
// connection was opened with an absolute path or a URI DSN.
func (s *Store) DatabasePath() (string, error) {
	var databases []struct{ Name, File string }
	if err := s.db.Raw("PRAGMA database_list").Scan(&databases).Error; err != nil {
		return "", err
	}
	for _, db := range databases {
		if db.Name == "main" && db.File != "" {
			return db.File, nil
		}
	}
	return "", fmt.Errorf("backups require a file-backed database")
}

// BackupToFile writes a self-contained snapshot of the live database to dest via
// VACUUM INTO: one read transaction produces a consistent, compact copy with no
// WAL sidecar, so a backup taken while writes are in flight is still coherent.
// dest must not already exist (callers use a fresh unique path).
func (s *Store) BackupToFile(dest string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// VACUUM INTO cannot run inside an explicit transaction.
	return s.db.Exec("VACUUM INTO ?", dest).Error
}

// RestoreDatabase replaces the live database with source. It keeps a file-level
// rollback copy of the current database, closes SQLite (so no WAL can replay over
// the restored bytes), installs the replacement, then reopens and validates it.
// If the replacement fails to open or validate, the previous database is restored
// from the rollback copy before returning the error; if that rollback also fails,
// the database is left closed and the error says so (the caller keeps maintenance
// active). The caller is responsible for draining in-flight work beforehand.
func (s *Store) RestoreDatabase(source string) error {
	path, err := s.DatabasePath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)

	// Stage the incoming database beside the live file.
	stagePath, err := stageCopy(source, dir, ".ttgo-restore-*.db")
	if err != nil {
		return err
	}
	defer os.Remove(stagePath)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Fold the WAL into the main file, then copy it as a rollback image. We cannot
	// rely on the pre-restore safety backup for rollback: restoring an older
	// database can erase that backup's own metadata row.
	_ = s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)").Error
	rollbackPath, err := stageCopy(path, dir, ".ttgo-rollback-*.db")
	if err != nil {
		return err
	}
	defer os.Remove(rollbackPath)

	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	if err := sqlDB.Close(); err != nil {
		return err
	}
	// Discard the old WAL/SHM so SQLite cannot replay them over the new main file.
	os.Remove(path + "-wal")
	os.Remove(path + "-shm")

	if err := os.Rename(stagePath, path); err != nil {
		// The live file is still in place; reopen it.
		_ = s.reopenDatabase(path)
		return fmt.Errorf("failed to install restored database: %w", err)
	}

	if err := s.reopenDatabase(path); err != nil {
		return s.rollbackDatabase(path, rollbackPath, fmt.Errorf("restored database failed to open: %w", err))
	}
	if err := s.validateOpenDB(); err != nil {
		return s.rollbackDatabase(path, rollbackPath, fmt.Errorf("restored database failed validation: %w", err))
	}
	return nil
}

// stageCopy copies src to a fresh temp file in dir (named by pattern) and returns
// its path.
func stageCopy(src, dir, pattern string) (string, error) {
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", err
	}
	p := f.Name()
	if cerr := f.Close(); cerr != nil {
		os.Remove(p)
		return "", cerr
	}
	if _, err := CopyFile(src, p); err != nil {
		os.Remove(p)
		return "", err
	}
	return p, nil
}

// rollbackDatabase reinstalls prev over path and reopens it, returning cause. Must
// be called with s.mu held. If the rollback itself fails the database is left
// closed and the returned error wraps both failures.
func (s *Store) rollbackDatabase(path, prev string, cause error) error {
	if s.db != nil {
		if sqlDB, err := s.db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}
	os.Remove(path + "-wal")
	os.Remove(path + "-shm")
	if _, cerr := CopyFile(prev, path); cerr != nil {
		return fmt.Errorf("%w; rollback also failed: %v", cause, cerr)
	}
	if rerr := s.reopenDatabase(path); rerr != nil {
		return fmt.Errorf("%w; rollback reopen failed: %v", cause, rerr)
	}
	return cause
}

// validateOpenDB sanity-checks the freshly reopened database. Must be called with
// s.mu held.
func (s *Store) validateOpenDB() error {
	var n int64
	return s.db.Raw("SELECT count(*) FROM users").Scan(&n).Error
}

// CreateBackup inserts a new backup record with in_progress status.
func (s *Store) CreateBackup(id, backupType, filePath, creatorID, creatorName string) (*models.Backup, error) {
	backup := &models.Backup{
		ID:          id,
		Type:        backupType,
		Status:      "in_progress",
		FilePath:    filePath,
		CreatorID:   creatorID,
		CreatorName: creatorName,
		CreatedAt:   time.Now(),
	}
	if err := s.db.Create(backup).Error; err != nil {
		return nil, err
	}
	return backup, nil
}

// CompleteBackup marks a backup as completed with file size and path.
func (s *Store) CompleteBackup(id string, fileSize int64, filePath string) error {
	now := time.Now()
	return s.db.Model(&models.Backup{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":       "completed",
		"file_size":    fileSize,
		"file_path":    filePath,
		"completed_at": now,
	}).Error
}

// FailBackup marks a backup as failed with an error message.
func (s *Store) FailBackup(id string, errMsg string) error {
	return s.db.Model(&models.Backup{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":        "failed",
		"error_message": errMsg,
	}).Error
}

// GetBackup retrieves a backup by ID.
func (s *Store) GetBackup(id string) (*models.Backup, error) {
	var backup models.Backup
	if err := s.db.Where("id = ?", id).First(&backup).Error; err != nil {
		return nil, err
	}
	return &backup, nil
}

// ListBackups returns all backups ordered by created_at DESC.
func (s *Store) ListBackups() ([]models.Backup, error) {
	var backups []models.Backup
	if err := s.db.Order("created_at DESC").Find(&backups).Error; err != nil {
		return nil, err
	}
	return backups, nil
}

// DeleteBackup deletes a backup record by ID.
func (s *Store) DeleteBackup(id string) error {
	return s.db.Where("id = ?", id).Delete(&models.Backup{}).Error
}

// GetBackupSchedule retrieves the singleton schedule row, creating defaults if absent.
func (s *Store) GetBackupSchedule() (*models.BackupSchedule, error) {
	var schedule models.BackupSchedule
	err := s.db.Where("id = ?", "default").First(&schedule).Error
	if err != nil {
		// Create default if not found
		schedule = models.BackupSchedule{
			ID:             "default",
			Enabled:        false,
			IntervalHours:  24,
			RetentionCount: 7,
			UpdatedAt:      time.Now(),
		}
		if err := s.db.Create(&schedule).Error; err != nil {
			return nil, err
		}
	}
	return &schedule, nil
}

// UpdateBackupSchedule updates the singleton schedule row.
func (s *Store) UpdateBackupSchedule(enabled bool, intervalHours, retentionCount int) (*models.BackupSchedule, error) {
	now := time.Now()
	updates := map[string]interface{}{
		"enabled":         enabled,
		"interval_hours":  intervalHours,
		"retention_count": retentionCount,
		"updated_at":      now,
	}
	if enabled {
		nextRun := now.Add(time.Duration(intervalHours) * time.Hour)
		updates["next_run_at"] = nextRun
	}

	// Upsert: update existing or create new
	result := s.db.Model(&models.BackupSchedule{}).Where("id = ?", "default").Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		schedule := &models.BackupSchedule{
			ID:             "default",
			Enabled:        enabled,
			IntervalHours:  intervalHours,
			RetentionCount: retentionCount,
			UpdatedAt:      now,
		}
		if enabled {
			nextRun := now.Add(time.Duration(intervalHours) * time.Hour)
			schedule.NextRunAt = &nextRun
		}
		if err := s.db.Create(schedule).Error; err != nil {
			return nil, err
		}
	}

	return s.GetBackupSchedule()
}

// DeleteOldestAutomaticBackups keeps only the newest keepCount automatic backups.
// Returns the IDs of deleted records for file cleanup by the caller.
func (s *Store) DeleteOldestAutomaticBackups(keepCount int) ([]string, error) {
	var all []models.Backup
	if err := s.db.Where("type = ? AND status = ?", "automatic", "completed").
		Order("created_at DESC").Find(&all).Error; err != nil {
		return nil, err
	}

	if len(all) <= keepCount {
		return nil, nil
	}

	toDelete := all[keepCount:]
	var ids []string
	for _, b := range toDelete {
		ids = append(ids, b.ID)
	}

	if err := s.db.Where("id IN ?", ids).Delete(&models.Backup{}).Error; err != nil {
		return nil, err
	}

	return ids, nil
}

// CopyFile copies src to dst and returns the number of bytes written.
func CopyFile(src, dst string) (int64, error) {
	in, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer out.Close()

	n, err := io.Copy(out, in)
	if err != nil {
		return 0, err
	}
	return n, out.Close()
}

// ValidateBackupFile checks that a file is a valid TTGO SQLite database.
func ValidateBackupFile(filePath string) error {
	// Check SQLite header (first 16 bytes)
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("cannot open file: %w", err)
	}
	header := make([]byte, 16)
	n, err := f.Read(header)
	f.Close()
	if err != nil || n < 16 {
		return fmt.Errorf("file too small or unreadable")
	}
	if string(header) != "SQLite format 3\x00" {
		return fmt.Errorf("not a valid SQLite database file")
	}

	// Verify expected TTGO tables exist
	dsn := fmt.Sprintf("%s?_busy_timeout=5000&mode=ro", filePath)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("cannot open as SQLite database: %w", err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	requiredTables := []string{"test_cases", "users", "folders"}
	for _, table := range requiredTables {
		var count int64
		row := db.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", table).Row()
		if err := row.Scan(&count); err != nil || count == 0 {
			return fmt.Errorf("missing required table '%s' — not a valid TTGO backup", table)
		}
	}

	return nil
}

// CheckDiskSpace verifies there's enough free space in the target directory.
// requiredBytes is the minimum free space needed (typically the DB file size).
func CheckDiskSpace(dir string, requiredBytes int64) error {
	// Use a generous 2x multiplier to be safe
	needed := requiredBytes * 2
	if needed < 10*1024*1024 {
		needed = 10 * 1024 * 1024 // minimum 10 MB
	}

	// Get filesystem free space (platform-specific; see disk_unix.go / disk_windows.go).
	available, ok := diskFreeBytes(dir)
	if !ok {
		// If we can't check, proceed anyway (best effort)
		slog.Warn("backup: cannot check disk space", "dir", dir)
		return nil
	}

	if available < needed {
		return fmt.Errorf("insufficient disk space: need %d MB, available %d MB",
			needed/(1024*1024), available/(1024*1024))
	}
	return nil
}

// SignFile returns an HMAC over the file at path, for backup integrity (F-017).
func (s *Store) SignFile(path string) (string, error) {
	if s.box == nil {
		return "", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return s.box.Sign(data), nil
}

// VerifyFileBytes reports whether sig is a valid signature for data.
func (s *Store) VerifyFileBytes(data []byte, sig string) bool {
	if s.box == nil {
		return true // no key configured — cannot verify, do not block
	}
	return s.box.Verify(data, sig)
}

// VerifyFile reports whether the file at path matches sig.
func (s *Store) VerifyFile(path, sig string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return s.VerifyFileBytes(data, sig), nil
}

// SetBackupSignature records the HMAC signature for a completed backup.
func (s *Store) SetBackupSignature(id, sig string) error {
	return s.db.Model(&models.Backup{}).Where("id = ?", id).Update("signature", sig).Error
}

// PruneBackupsBeyond deletes the oldest backup records beyond maxKeep across ALL
// types (manual, automatic, pre-restore) and returns their ids so the caller can
// remove the files. The existing automatic-only retention left manual/pre-restore
// backups to accumulate without bound (F-041).
func (s *Store) PruneBackupsBeyond(maxKeep int) ([]string, error) {
	if maxKeep < 1 {
		maxKeep = 1
	}
	var ids []string
	// LIMIT -1 OFFSET n => "everything past the n newest" in SQLite.
	if err := s.db.Model(&models.Backup{}).
		Order("created_at DESC").
		Limit(-1).Offset(maxKeep).
		Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	if err := s.db.Delete(&models.Backup{}, "id IN ?", ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}
