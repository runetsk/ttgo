package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func seedLegacyDefectLinks(t *testing.T, dsn string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	// Minimal parent tables with only valid IDs — tc3/tcX deliberately omitted to create orphans.
	require.NoError(t, db.Exec(`CREATE TABLE test_cases (id TEXT PRIMARY KEY)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO test_cases (id) VALUES ('tc1'), ('tc2')`).Error)

	require.NoError(t, db.Exec(`CREATE TABLE run_results (id TEXT PRIMARY KEY)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO run_results (id) VALUES ('rr1'), ('rr2')`).Error)

	// Legacy defect_links table (Jira-keyed schema).
	require.NoError(t, db.Exec(`CREATE TABLE defect_links (
		id TEXT PRIMARY KEY, test_case_id TEXT, run_result_id TEXT,
		jira_issue_key TEXT NOT NULL, last_known_summary TEXT, last_known_status TEXT,
		last_known_priority TEXT, last_known_assignee TEXT, last_known_url TEXT,
		status_category TEXT, comment_pending INTEGER, pending_comment_text TEXT,
		last_synced_at DATETIME, created_at DATETIME, updated_at DATETIME)`).Error)

	// l1: tc1/rr1 both valid — kept as-is, deduped with l2 under PROJ-1.
	// l2: tc2/rr2 both valid — dedupes with l1 under PROJ-1.
	// l3: tc3 orphan (missing in test_cases), rr NULL → both nil after repair → DROPPED.
	// l4: tcX orphan (missing in test_cases), rr1 valid → tc nulled, rr kept → link KEPT with test_case_id NULL.
	require.NoError(t, db.Exec(`INSERT INTO defect_links
		(id,test_case_id,run_result_id,jira_issue_key,last_known_summary,status_category,last_known_priority,last_known_url,created_at,updated_at)
		VALUES
		('l1','tc1','rr1','PROJ-1','Login broken','indeterminate','High','https://x/browse/PROJ-1','2026-01-01','2026-01-02'),
		('l2','tc2','rr2','PROJ-1','Login broken','indeterminate','High','https://x/browse/PROJ-1','2026-01-03','2026-01-04'),
		('l3','tc3',NULL,'PROJ-9','Done bug','done','Low','https://x/browse/PROJ-9','2026-01-05','2026-01-05'),
		('l4','tcX','rr1','PROJ-9','Done bug','done','Low','https://x/browse/PROJ-9','2026-01-06','2026-01-06')`).Error)

	sqlDB, _ := db.DB()
	require.NoError(t, sqlDB.Close())
}

func TestMigrateDefects_LegacyConversion(t *testing.T) {
	dir := t.TempDir()
	dsn := filepath.Join(dir, "ttgo.db")
	seedLegacyDefectLinks(t, dsn)

	wd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir)) // New() creates backups/ in cwd
	defer os.Chdir(wd)

	s, err := New(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	// 2 defects: PROJ-1 (deduped from l1+l2) and PROJ-9.
	var defects []struct{ ID, Title, Status, Severity, ExternalKey string }
	require.NoError(t, s.db.Raw(`SELECT id,title,status,severity,external_key FROM defects ORDER BY external_key`).Scan(&defects).Error)
	require.Len(t, defects, 2, "PROJ-1 deduped + PROJ-9 = 2 defects")
	assert.Equal(t, "open", defects[0].Status)    // indeterminate -> open
	assert.Equal(t, "major", defects[0].Severity) // High -> major
	assert.Equal(t, "closed", defects[1].Status)  // done -> closed

	// Exactly 3 links remain: l1, l2, l4 — l3 dropped (both refs nil after repair).
	var linkCount int64
	s.db.Raw(`SELECT count(*) FROM defect_links`).Scan(&linkCount)
	assert.Equal(t, int64(3), linkCount, "l3 should be dropped; l1, l2, l4 survive")

	// The surviving PROJ-9 link (from l4) has test_case_id NULL and run_result_id = 'rr1'.
	proj9DefectID := defects[1].ID
	var proj9Links []struct {
		TestCaseID  *string
		RunResultID *string
	}
	require.NoError(t, s.db.Raw(
		`SELECT test_case_id, run_result_id FROM defect_links WHERE defect_id = ?`, proj9DefectID,
	).Scan(&proj9Links).Error)
	require.Len(t, proj9Links, 1, "only l4 survives for PROJ-9 (l3 dropped)")
	assert.Nil(t, proj9Links[0].TestCaseID, "tc was orphaned — should be nulled")
	assert.NotNil(t, proj9Links[0].RunResultID)
	assert.Equal(t, "rr1", *proj9Links[0].RunResultID)

	// No surviving link may reference tc3 or tcX.
	var badLinks int64
	s.db.Raw(`SELECT count(*) FROM defect_links WHERE test_case_id IN ('tc3','tcX')`).Scan(&badLinks)
	assert.Equal(t, int64(0), badLinks, "no link may reference orphaned test_case_id values")

	// Legacy table must be dropped.
	var legacy int64
	s.db.Raw(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='defect_links_legacy'`).Scan(&legacy)
	assert.Equal(t, int64(0), legacy)

	// Idempotency: re-running bootstrapDB must not change counts.
	require.NoError(t, s.bootstrapDB())
	var defectCount int64
	s.db.Raw(`SELECT count(*) FROM defects`).Scan(&defectCount)
	assert.Equal(t, int64(2), defectCount)
	var linkCount2 int64
	s.db.Raw(`SELECT count(*) FROM defect_links`).Scan(&linkCount2)
	assert.Equal(t, int64(3), linkCount2)
}
