package models

import "time"

// Backup represents a point-in-time snapshot of the system database.
type Backup struct {
	ID           string     `json:"id" gorm:"primaryKey"`
	Type         string     `json:"type" gorm:"not null;index:idx_backups_type"` // "manual", "automatic", or "pre-restore"
	Status       string     `json:"status" gorm:"not null"`                      // "in_progress", "completed", or "failed"
	FileSize     int64      `json:"file_size"`                                   // bytes, populated on completion
	FilePath     string     `json:"file_path" gorm:"not null"`                   // relative path to backup file
	CreatorID    string     `json:"creator_id,omitempty"`                        // user ID (NULL for automatic)
	CreatorName  string     `json:"creator_name,omitempty"`                      // denormalized display name
	ErrorMessage string     `json:"error_message,omitempty"`                     // error details if failed
	Signature    string     `json:"signature,omitempty"`                         // HMAC over the file bytes (F-017)
	CreatedAt    time.Time  `json:"created_at" gorm:"not null;index:idx_backups_created_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

// BackupSchedule is a singleton configuration for automatic backups.
type BackupSchedule struct {
	ID             string     `json:"id" gorm:"primaryKey"` // fixed "default"
	Enabled        bool       `json:"enabled" gorm:"not null;default:false"`
	IntervalHours  int        `json:"interval_hours" gorm:"not null;default:24"`
	RetentionCount int        `json:"retention_count" gorm:"not null;default:7"`
	LastRunAt      *time.Time `json:"last_run_at,omitempty"`
	NextRunAt      *time.Time `json:"next_run_at,omitempty"`
	UpdatedAt      time.Time  `json:"updated_at" gorm:"not null"`
}
