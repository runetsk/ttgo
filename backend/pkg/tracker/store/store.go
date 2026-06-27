package store

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"ttgo/internal/safehttp"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/secretbox"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Store struct {
	db            *gorm.DB
	mu            sync.RWMutex // protects db during ReopenDB
	qtestImportMu sync.Mutex
	analysisMu    sync.Mutex     // serializes version-bumps in CreateAnalysis (ai-failure-analysis)
	httpClient    *http.Client   // shared HTTP client for external API calls (Jira, QTest, etc.)
	box           *secretbox.Box // at-rest encryption for integration/LLM secrets (F-016)
}

// encryptSecret returns the value encrypted for storage. Idempotent and
// best-effort: empty strings and already-encrypted values pass through, and a
// crypto error never blocks the write (the value is stored as-is).
func (s *Store) encryptSecret(v string) string {
	if s.box == nil || v == "" {
		return v
	}
	if enc, err := s.box.Encrypt(v); err == nil {
		return enc
	}
	return v
}

// decryptSecret reverses encryptSecret; plaintext (pre-encryption) values pass
// through unchanged.
func (s *Store) decryptSecret(v string) string {
	if s.box == nil {
		return v
	}
	if dec, err := s.box.Decrypt(v); err == nil {
		return dec
	}
	return v
}

func New(dsn string) (*Store, error) {
	// Add busy timeout, WAL mode, and foreign keys for performance and integrity
	dsnWithParams := fmt.Sprintf("%s?_busy_timeout=5000&_journal_mode=wal&_foreign_keys=on", dsn)
	db, err := gorm.Open(sqlite.Open(dsnWithParams), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// 015-database-backups: ensure backups directory exists
	if err := os.MkdirAll("backups", 0755); err != nil {
		return nil, fmt.Errorf("failed to create backups directory: %w", err)
	}

	box, err := secretbox.LoadOrCreate("TTGO_ENCRYPTION_KEY", "secret.key")
	if err != nil {
		return nil, fmt.Errorf("failed to load encryption key: %w", err)
	}

	s := &Store{
		db: db,
		// SSRF-guarded client for outbound integration calls (Jira/Confluence/QTest).
		// Blocks cloud-metadata/link-local but allows self-hosted private hosts (F-003).
		httpClient: safehttp.IntegrationClient(15 * time.Second),
		box:        box,
	}

	if err := s.bootstrapDB(); err != nil {
		return nil, err
	}

	return s, nil
}

// bootstrapDB runs all schema migrations, indexes, FTS setup, and template seeding.
// Called on initial startup and after ReopenDB to ensure the DB is fully initialized.
func (s *Store) bootstrapDB() error {
	db := s.db

	// T006: removed duplicate &models.CustomFieldValue{}
	// T008: added new models for API tokens, webhooks, analytics
	// 004-user-auth: added User and UserSession models
	if err := db.AutoMigrate(
		&models.Folder{},
		&models.Category{},
		&models.TestCase{},
		&models.CategoryTestCase{},
		&models.TestStep{},
		&models.CustomFieldDefinition{},
		&models.CustomFieldValue{},
		&models.AuditLog{},
		&models.RunFolder{},
		&models.TestRun{},
		&models.RunResult{},
		&models.ApiToken{},
		&models.WebhookConfig{},
		&models.WebhookDispatchLog{},
		&models.RunMetric{},
		&models.FlakyStat{},
		&models.User{},
		&models.UserSession{},
		&models.DemoSeed{},
		&models.TestCaseVersion{},           // 006-test-case-versioning
		&models.Requirement{},               // 007-req-traceability
		&models.RequirementTestCaseLink{},   // 007-req-traceability
		&models.JiraConfig{},                // 007-req-traceability / 008-jira-integration
		&models.DefectLink{},                // 008-jira-integration
		&models.LLMProviderConfig{},         // 010-ai-test-generation
		&models.AIGenTemplate{},             // 010-ai-test-generation
		&models.AIGenCoverageConfig{},       // 010-ai-test-generation: coverage levels
		&models.ConfluenceConfig{},          // 011-jira-confluence-import
		&models.QTestConfig{},               // 013-qtest-sync
		&models.QTestMapping{},              // 013-qtest-sync
		&models.QTestEnabledProject{},       // 013-qtest-sync: multi-project
		&models.Backup{},                    // 015-database-backups
		&models.BackupSchedule{},            // 015-database-backups
		&models.Comment{},                   // comments on runs and results
		&models.RunResultAnalysis{},         // ai-failure-analysis: versioned analysis records
		&models.RunAnalysisJob{},            // ai-failure-analysis: batch/auto job tracking
		&models.AIFailureAnalysisSettings{}, // ai-failure-analysis: admin config singleton
		&models.AIFeatureSettings{},         // ai-features-toggle: global AI master switch
	); err != nil {
		return fmt.Errorf("failed to migrate schema: %w", err)
	}

	// Dead-code cleanup: drop orphaned executions table (replaced by run_results).
	db.Exec(`DROP TABLE IF EXISTS executions`)

	// 016-retries: partial unique index for attempt_number (GORM can't do partial indexes via tags)
	db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_run_results_attempt ON run_results(test_run_id, test_case_id, attempt_number) WHERE test_case_id IS NOT NULL`)

	// ai-failure-analysis: composite index so "latest version per run_result" is a single-row lookup.
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_run_result_analyses_result_version
		ON run_result_analyses (run_result_id, version DESC)`).Error; err != nil {
		return fmt.Errorf("failed to create run_result_analyses index: %w", err)
	}

	// 008-jira-integration: unique constraint on (test_case_id, jira_issue_key) — FR-013.
	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uq_defect_link ON defect_links (test_case_id, jira_issue_key)`).Error; err != nil {
		return fmt.Errorf("failed to create defect_links unique index: %w", err)
	}

	// 013-qtest-sync: unique constraint on test_case_id and status filtering index.
	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uq_qtest_mapping_test_case ON qtest_mappings (test_case_id)`).Error; err != nil {
		return fmt.Errorf("failed to create qtest_mappings unique index: %w", err)
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_qtest_mapping_status ON qtest_mappings (sync_status)`).Error; err != nil {
		return fmt.Errorf("failed to create qtest_mappings status index: %w", err)
	}

	// ai-failure-analysis: at most one active (queued/running) analysis job per run,
	// enforced at the DB level so concurrent enqueues cannot both insert (F-008).
	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uq_run_analysis_active
		ON run_analysis_jobs (test_run_id) WHERE status IN ('queued','running')`).Error; err != nil {
		return fmt.Errorf("failed to create run_analysis_jobs active-job index: %w", err)
	}

	// 007-req-traceability: one requirement per imported source, so concurrent
	// imports of the same source key cannot create duplicates (F-054).
	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uq_requirement_source
		ON requirements (source_type, source_key) WHERE source_key != ''`).Error; err != nil {
		return fmt.Errorf("failed to create requirements source unique index: %w", err)
	}

	// FTS5 must be compiled into the SQLite driver via the sqlite_fts5 build
	// tag. Fail fast with an actionable message instead of the cryptic
	// "no such module: fts5" that surfaces below otherwise.
	if !fts5BuildTagSet {
		return fmt.Errorf("FTS5 unavailable: this binary was built without the sqlite_fts5 build tag; run `make setup` (sets GOFLAGS) or build via `make build`/`make test` — see CLAUDE.md")
	}

	// T009+T010: create FTS5 tables, triggers, and performance indexes
	if err := createFTS5Tables(db); err != nil {
		return fmt.Errorf("failed to create FTS5 tables: %w", err)
	}
	if err := createPerformanceIndexes(db); err != nil {
		return fmt.Errorf("failed to create performance indexes: %w", err)
	}

	// Backfill FTS index for any pre-existing test cases (e.g. after DB upgrade)
	if err := db.Exec(`
		INSERT OR IGNORE INTO test_cases_fts(rowid, name, description)
		SELECT rowid, name, description FROM test_cases
	`).Error; err != nil {
		return fmt.Errorf("failed to backfill FTS index: %w", err)
	}

	// 010-ai-test-generation: seed default prompt template on startup if not present,
	// and keep the "default_content" column in sync so "Reset to Default" always
	// reflects the latest built-in template.
	tmpl, err := s.GetOrCreateDefaultTemplate()
	if err != nil {
		return fmt.Errorf("failed to seed AI generation template: %w", err)
	}
	if tmpl.DefaultContent != defaultPromptTemplate {
		db.Model(tmpl).Update("default_content", defaultPromptTemplate)
	}
	if tmpl.DefaultParentContent != defaultParentPromptTemplate {
		updates := map[string]interface{}{"default_parent_content": defaultParentPromptTemplate}
		// If parent_content was never customized (empty or matches old default), also update it
		if tmpl.ParentContent == "" || tmpl.ParentContent == tmpl.DefaultParentContent {
			updates["parent_content"] = defaultParentPromptTemplate
		}
		db.Model(tmpl).Updates(updates)
	}

	// ai-failure-analysis: seed default settings on startup if not present,
	// and keep default_prompt_template column in sync so "Reset to default" works.
	if err := s.seedFailureAnalysisSettings(); err != nil {
		return fmt.Errorf("failed to seed AI failure analysis settings: %w", err)
	}

	// Encrypt any pre-existing plaintext integration/LLM secrets at rest (F-016).
	if err := s.backfillEncryptSecrets(); err != nil {
		return fmt.Errorf("failed to backfill secret encryption: %w", err)
	}

	// Sign any pre-existing (unsigned) backup files so id-restore can fail closed
	// on unsigned/tampered backups without rejecting legitimate legacy ones (F-017).
	if err := s.backfillBackupSignatures(); err != nil {
		return fmt.Errorf("failed to backfill backup signatures: %w", err)
	}

	return nil
}

// backfillBackupSignatures signs completed backups that have no signature yet.
func (s *Store) backfillBackupSignatures() error {
	if s.box == nil {
		return nil
	}
	var backups []models.Backup
	if err := s.db.Where("status = ? AND (signature = '' OR signature IS NULL)", "completed").Find(&backups).Error; err != nil {
		return err
	}
	for _, b := range backups {
		path := filepath.Join("backups", "backup-"+b.ID+".db")
		if _, err := os.Stat(path); err != nil {
			continue // file gone — nothing to sign (restore would fail anyway)
		}
		sig, err := s.SignFile(path)
		if err != nil || sig == "" {
			continue
		}
		if err := s.SetBackupSignature(b.ID, sig); err != nil {
			return err
		}
	}
	return nil
}

// backfillEncryptSecrets encrypts any config secrets still stored in plaintext.
// Idempotent (skips already-encrypted values), so it is safe to run on every boot
// and after a restore.
func (s *Store) backfillEncryptSecrets() error {
	if s.box == nil {
		return nil
	}
	encryptColumn := func(model interface{}, id, column, value string) error {
		if value == "" || secretbox.IsEncrypted(value) {
			return nil
		}
		return s.db.Model(model).Where("id = ?", id).Update(column, s.encryptSecret(value)).Error
	}

	var jira models.JiraConfig
	if err := s.db.First(&jira, "id = ?", jiraConfigSingletonID).Error; err == nil {
		if err := encryptColumn(&models.JiraConfig{}, jira.ID, "api_token", jira.APIToken); err != nil {
			return err
		}
	}
	var conf models.ConfluenceConfig
	if err := s.db.First(&conf, "id = ?", confluenceConfigSingletonID).Error; err == nil {
		if err := encryptColumn(&models.ConfluenceConfig{}, conf.ID, "api_token", conf.APIToken); err != nil {
			return err
		}
	}
	var qt models.QTestConfig
	if err := s.db.First(&qt, "id = ?", qtestConfigSingletonID).Error; err == nil {
		if err := encryptColumn(&models.QTestConfig{}, qt.ID, "api_token", qt.APIToken); err != nil {
			return err
		}
	}
	var providers []models.LLMProviderConfig
	if err := s.db.Find(&providers).Error; err != nil {
		return err
	}
	for _, p := range providers {
		if err := encryptColumn(&models.LLMProviderConfig{}, p.ID, "api_key", p.APIKey); err != nil {
			return err
		}
	}
	return nil
}

// DB exposes the underlying *gorm.DB for raw operations (e.g., PRAGMA calls).
func (s *Store) DB() *gorm.DB {
	return s.db
}

// ReopenDB closes the current database connection and reopens it at the given DSN.
// This is used after a restore operation replaces the database file on disk.
// Thread-safe: acquires an exclusive lock to prevent concurrent DB access during swap.
func (s *Store) ReopenDB(dsn string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Close the existing connection
	sqlDB, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}

	// Reopen with the same parameters
	dsnWithParams := fmt.Sprintf("%s?_busy_timeout=5000&_journal_mode=wal&_foreign_keys=on", dsn)
	db, err := gorm.Open(sqlite.Open(dsnWithParams), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to reopen database: %w", err)
	}

	s.db = db

	// Re-run full bootstrap (schema, indexes, FTS, templates) against the restored DB
	if err := s.bootstrapDB(); err != nil {
		return fmt.Errorf("failed to bootstrap after reopen: %w", err)
	}

	return nil
}

// SeedAdminIfNeeded creates an initial admin user if none exists.
// adminEmail and adminPassword must both be non-empty; returns an error
// if the server would start without any admin user.
func (s *Store) SeedAdminIfNeeded(adminEmail, adminPassword string) error {
	var count int64
	s.db.Model(&models.User{}).Where("role = ? AND active = ?", "admin", true).Count(&count)
	if count > 0 {
		return nil // at least one active admin exists already
	}

	if strings.TrimSpace(adminEmail) == "" || strings.TrimSpace(adminPassword) == "" {
		return fmt.Errorf("no admin user exists: set ADMIN_EMAIL and ADMIN_PASSWORD environment variables to seed the first admin account")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), 12)
	if err != nil {
		return fmt.Errorf("failed to hash admin password: %w", err)
	}

	admin := &models.User{
		ID:             uuid.New().String(),
		Email:          strings.ToLower(strings.TrimSpace(adminEmail)),
		DisplayName:    "Admin",
		HashedPassword: string(hash),
		Role:           "admin",
		Active:         true,
	}
	if err := s.db.Create(admin).Error; err != nil {
		return fmt.Errorf("failed to seed admin user: %w", err)
	}
	return nil
}
