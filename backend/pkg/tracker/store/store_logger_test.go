package store

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"ttgo/pkg/tracker/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestGormLoggerIgnoresRecordNotFound proves the shared store logger suppresses
// the noisy ErrRecordNotFound trace produced by the failure-analysis worker's
// empty-queue polling (run_analysis_jobs.go:85), while still logging genuine SQL
// errors so real problems stay visible.
func TestGormLoggerIgnoresRecordNotFound(t *testing.T) {
	var buf bytes.Buffer
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormLogger(&buf)})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.RunAnalysisJob{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Mirror NextQueuedAnalysisJob against an empty table: this returns
	// ErrRecordNotFound, which the worker handles gracefully and must NOT be logged.
	var job models.RunAnalysisJob
	err = db.Where("status = ?", models.RunAnalysisJobStatusQueued).
		Order("created_at ASC").First(&job).Error
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}
	if strings.Contains(buf.String(), "record not found") {
		t.Fatalf("ErrRecordNotFound should be suppressed in logs, got:\n%s", buf.String())
	}

	// A genuine SQL error is not ErrRecordNotFound and must still be logged, so the
	// suppression above never silences a real failure.
	buf.Reset()
	if err := db.Exec("SELECT * FROM definitely_not_a_table").Error; err == nil {
		t.Fatal("expected an error querying a missing table")
	}
	if !strings.Contains(buf.String(), "no such table") {
		t.Fatalf("genuine SQL errors must still be logged, got:\n%s", buf.String())
	}
}
