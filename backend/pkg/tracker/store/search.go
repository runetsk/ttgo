package store

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// createFTS5Tables creates the FTS5 virtual table and sync triggers for test cases.
func createFTS5Tables(db *gorm.DB) error {
	stmts := []string{
		`CREATE VIRTUAL TABLE IF NOT EXISTS test_cases_fts USING fts5(
			name,
			description,
			content=test_cases,
			content_rowid=rowid,
			prefix='2 3'
		)`,
		`CREATE TRIGGER IF NOT EXISTS test_cases_ai AFTER INSERT ON test_cases BEGIN
			INSERT INTO test_cases_fts(rowid, name, description) VALUES (new.rowid, new.name, new.description);
		END`,
		`CREATE TRIGGER IF NOT EXISTS test_cases_ad AFTER DELETE ON test_cases BEGIN
			INSERT INTO test_cases_fts(test_cases_fts, rowid, name, description) VALUES ('delete', old.rowid, old.name, old.description);
		END`,
		`CREATE TRIGGER IF NOT EXISTS test_cases_au AFTER UPDATE ON test_cases BEGIN
			INSERT INTO test_cases_fts(test_cases_fts, rowid, name, description) VALUES ('delete', old.rowid, old.name, old.description);
			INSERT INTO test_cases_fts(rowid, name, description) VALUES (new.rowid, new.name, new.description);
		END`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			if strings.Contains(err.Error(), "no such module: fts5") {
				return fmt.Errorf("FTS5 module unavailable: this binary was built without the sqlite_fts5 build tag; run `make setup` or build via `make build`/`make test` — see CLAUDE.md: %w", err)
			}
			return fmt.Errorf("FTS5 setup: %w", err)
		}
	}
	return nil
}

// createPerformanceIndexes creates composite indexes on run_results for analytics queries.
func createPerformanceIndexes(db *gorm.DB) error {
	stmts := []string{
		`CREATE INDEX IF NOT EXISTS idx_run_results_test_case ON run_results(test_case_id)`,
		`CREATE INDEX IF NOT EXISTS idx_run_results_status ON run_results(status)`,
		`CREATE INDEX IF NOT EXISTS idx_run_results_start_time ON run_results(start_time)`,
		`CREATE INDEX IF NOT EXISTS idx_run_results_composite ON run_results(test_case_id, status, start_time)`,
		`CREATE INDEX IF NOT EXISTS idx_webhook_dispatch_logs_wh ON webhook_dispatch_logs(webhook_id, dispatched_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_timestamp ON audit_logs(timestamp)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("index setup: %w", err)
		}
	}
	return nil
}

// SearchResult holds a search result entry.
type SearchResult struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	FolderID    string `json:"folder_id"`
}

// sanitizeFTSQuery escapes the query for safe use in FTS5.
func sanitizeFTSQuery(q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return ""
	}
	// Escape double quotes, then wrap in quotes for phrase search with prefix
	escaped := strings.ReplaceAll(q, `"`, `""`)
	return fmt.Sprintf(`"%s"*`, escaped)
}

// SearchTestCases performs FTS5 full-text search against test case names and descriptions.
func (s *Store) SearchTestCases(query string, limit, offset int) ([]SearchResult, int, error) {
	if query == "" {
		return []SearchResult{}, 0, nil
	}
	ftsQuery := sanitizeFTSQuery(query)
	if limit <= 0 {
		limit = 50
	}

	var results []SearchResult
	var total int64

	baseSQL := `
		SELECT tc.id, tc.name, tc.description, tc.folder_id
		FROM test_cases tc
		JOIN test_cases_fts fts ON tc.rowid = fts.rowid
		WHERE test_cases_fts MATCH ?`

	countSQL := `SELECT COUNT(*) FROM test_cases tc JOIN test_cases_fts fts ON tc.rowid = fts.rowid WHERE test_cases_fts MATCH ?`

	if err := s.db.Raw(countSQL, ftsQuery).Scan(&total).Error; err != nil {
		return nil, 0, err
	}

	rows, err := s.db.Raw(baseSQL+` LIMIT ? OFFSET ?`, ftsQuery, limit, offset).Rows()
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.ID, &r.Name, &r.Description, &r.FolderID); err != nil {
			return nil, 0, err
		}
		results = append(results, r)
	}

	if results == nil {
		results = []SearchResult{}
	}
	return results, int(total), nil
}

// RebuildFTSIndex rebuilds the FTS5 index for all test cases (e.g. after bulk import).
func (s *Store) RebuildFTSIndex() error {
	return s.db.Exec(`INSERT INTO test_cases_fts(test_cases_fts) VALUES ('rebuild')`).Error
}

// PopulateFTSIndex backfills the FTS5 index from existing test_cases rows.
func (s *Store) PopulateFTSIndex() error {
	return s.db.Exec(`
		INSERT OR IGNORE INTO test_cases_fts(rowid, name, description)
		SELECT rowid, name, description FROM test_cases
	`).Error
}
