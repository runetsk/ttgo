package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// seedPreTypeSafeSchema creates run_result_analyses and run_results exactly as they
// existed before this feature (no engine/suggestion/snapshot-engine columns) and inserts
// one analysis per verdict plus one snapshotted triage decision.
func seedPreTypeSafeSchema(t *testing.T, dsn string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	// Parents first: store.New opens with _foreign_keys=on and AutoMigrate's table rebuild
	// checks them, so every run and result the fixture references must exist.
	require.NoError(t, db.Exec(`CREATE TABLE test_runs (id TEXT PRIMARY KEY, name TEXT, status TEXT, created_at DATETIME, updated_at DATETIME)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO test_runs (id, name, status) VALUES ('run1', 'legacy', 'completed')`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE run_result_analyses (
		id TEXT PRIMARY KEY, run_result_id TEXT NOT NULL, version INTEGER NOT NULL,
		verdict TEXT NOT NULL, confidence TEXT NOT NULL, summary TEXT, next_action TEXT,
		rationale TEXT, raw_response TEXT, model_name TEXT, provider_id TEXT,
		token_usage_prompt INTEGER, token_usage_completion INTEGER,
		dedup_group_key TEXT, source_analysis_id TEXT, created_by TEXT, created_at DATETIME)`).Error)
	for i, v := range []string{"product_bug", "flaky_test", "test_data", "environment", "infrastructure", "unknown"} {
		require.NoError(t, db.Exec(`INSERT INTO run_result_analyses
			(id, run_result_id, version, verdict, confidence, created_at)
			VALUES (?, ?, 1, ?, 'medium', '2026-09-01')`, "a"+string(rune('0'+i)), "rr"+string(rune('0'+i)), v).Error)
	}
	require.NoError(t, db.Exec(`CREATE TABLE run_results (
		id TEXT PRIMARY KEY, test_run_id TEXT, test_case_id TEXT, attempt_number INTEGER,
		test_name_snapshot TEXT, status TEXT, defect_type TEXT,
		suggested_verdict TEXT, suggested_defect_type TEXT, suggested_confidence TEXT,
		decided_at DATETIME, created_at DATETIME, updated_at DATETIME)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO run_results
		(id, test_run_id, attempt_number, status, defect_type, suggested_verdict, suggested_defect_type, suggested_confidence, decided_at)
		VALUES ('rr-decided', 'run1', 1, 'FAIL', 'product_bug', 'product_bug', 'product_bug', 'high', '2026-09-02'),
		       ('rr-undecided', 'run1', 1, 'FAIL', 'to_investigate', '', '', '', NULL),
		       ('rr0', 'run1', 1, 'FAIL', 'to_investigate', '', '', '', NULL),
		       ('rr1', 'run1', 1, 'FAIL', 'to_investigate', '', '', '', NULL),
		       ('rr2', 'run1', 1, 'FAIL', 'to_investigate', '', '', '', NULL),
		       ('rr3', 'run1', 1, 'FAIL', 'to_investigate', '', '', '', NULL),
		       ('rr4', 'run1', 1, 'FAIL', 'to_investigate', '', '', '', NULL),
		       ('rr5', 'run1', 1, 'FAIL', 'to_investigate', '', '', '', NULL)`).Error)
	sqlDB, _ := db.DB()
	require.NoError(t, sqlDB.Close())
}

func TestTypeSafeMigration_BackfillsLegacyRows(t *testing.T) {
	dir := t.TempDir()
	dsn := filepath.Join(dir, "ttgo.db")
	seedPreTypeSafeSchema(t, dsn)
	wd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(wd)

	s, err := New(dsn)
	require.NoError(t, err)
	defer s.Close()

	type row struct {
		Verdict             string
		Engine              string
		SuggestedDefectType string
		ConfidenceScore     *float64
		NarrativeStatus     string
	}
	var rows []row
	require.NoError(t, s.db.Raw(`SELECT verdict, engine, suggested_defect_type, confidence_score, narrative_status
		FROM run_result_analyses ORDER BY id`).Scan(&rows).Error)
	want := map[string]string{
		"product_bug": "product_bug", "flaky_test": "automation_bug", "test_data": "automation_bug",
		"environment": "system_issue", "infrastructure": "system_issue", "unknown": "",
	}
	require.Len(t, rows, 6)
	for _, r := range rows {
		require.Equal(t, "generative", r.Engine, r.Verdict)
		require.Equal(t, want[r.Verdict], r.SuggestedDefectType, r.Verdict)
		require.Nil(t, r.ConfidenceScore, "historical numeric confidence must stay NULL")
		require.Equal(t, "ok", r.NarrativeStatus)
	}

	var decidedEngine, undecidedEngine string
	require.NoError(t, s.db.Raw(`SELECT suggested_engine FROM run_results WHERE id = 'rr-decided'`).Scan(&decidedEngine).Error)
	require.NoError(t, s.db.Raw(`SELECT suggested_engine FROM run_results WHERE id = 'rr-undecided'`).Scan(&undecidedEngine).Error)
	require.Equal(t, "generative", decidedEngine)
	require.Equal(t, "", undecidedEngine, "rows without a snapshot are not attributed to an engine")

	// Idempotent: running both backfills again changes nothing.
	require.NoError(t, s.backfillAnalysisSuggestions())
	require.NoError(t, s.backfillSnapshotEngine())
	var again []row
	require.NoError(t, s.db.Raw(`SELECT verdict, engine, suggested_defect_type, confidence_score, narrative_status
		FROM run_result_analyses ORDER BY id`).Scan(&again).Error)
	require.Equal(t, rows, again)
}

// TestTypeSafeMigration_NewSwitchesDefaultOnExistingRow: a database whose TypeSafe settings
// row predates the explanation switch and the takeover threshold gets both columns with the
// behaviour-preserving defaults (explanations on, never hand a verdict to the LLM), and keeps
// the values an admin had already saved.
func TestTypeSafeMigration_NewSwitchesDefaultOnExistingRow(t *testing.T) {
	dir := t.TempDir()
	dsn := filepath.Join(dir, "ttgo.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE TABLE `type_safe_settings` (`id` text,`enabled` numeric NOT NULL DEFAULT false,`api_key` text,"+
		"`model` text NOT NULL DEFAULT \"jev-1.13.0\",`timeout_seconds` integer NOT NULL DEFAULT 30,"+
		"`verdict_engine_enabled` numeric NOT NULL DEFAULT true,`semantic_dedup_enabled` numeric NOT NULL DEFAULT true,"+
		"`allow_auto_failure_analysis` numeric NOT NULL DEFAULT false,`created_at` datetime,`updated_at` datetime,PRIMARY KEY (`id`))").Error)
	require.NoError(t, db.Exec(`INSERT INTO type_safe_settings
		(id, enabled, api_key, model, timeout_seconds, verdict_engine_enabled, semantic_dedup_enabled, allow_auto_failure_analysis)
		VALUES ('singleton', true, '', 'jev-latest', 45, true, false, true)`).Error)
	sqlDB, _ := db.DB()
	require.NoError(t, sqlDB.Close())

	wd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(wd)
	s, err := New(dsn)
	require.NoError(t, err)
	defer s.Close()

	got, err := s.GetTypeSafeSettings()
	require.NoError(t, err)
	require.True(t, got.NarrativeEnabled, "existing installs keep their explanations")
	require.Equal(t, 0, got.EscalateBelowPct, "existing installs never hand verdicts to the LLM until an admin opts in")
	require.True(t, got.Enabled)
	require.Equal(t, "jev-latest", got.Model)
	require.Equal(t, 45, got.TimeoutSeconds)
	require.False(t, got.SemanticDedupEnabled)
	require.True(t, got.AllowAutoFailureAnalysis)
}
