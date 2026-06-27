package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T015: Assert AutoMigrate works and CustomFieldValue table exists exactly once (no duplicate error)
func TestAutoMigrateNoDuplicateCustomFieldValue(t *testing.T) {
	s, err := New(":memory:")
	require.NoError(t, err, "New store should not fail")
	assert.NotNil(t, s)

	// If there were a duplicate AutoMigrate for CustomFieldValue, the second call
	// could fail or cause issues. Calling New twice with the same :memory: db would
	// fail anyway, so we just verify the schema is created correctly.
	// The fix was to remove the duplicate &models.CustomFieldValue{} from AutoMigrate.
	var tableNames []string
	rows, err := s.db.Raw("SELECT name FROM sqlite_master WHERE type='table' AND name='custom_field_values'").Rows()
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var name string
		_ = rows.Scan(&name)
		tableNames = append(tableNames, name)
	}
	assert.Equal(t, 1, len(tableNames), "custom_field_values table should exist exactly once")
}

// T015: Verify new tables are created by AutoMigrate
func TestAutoMigrateNewTables(t *testing.T) {
	s, err := New(":memory:")
	require.NoError(t, err)

	expectedTables := []string{
		"api_tokens",
		"webhook_configs",
		"webhook_dispatch_logs",
		"run_metrics",
		"flaky_stats",
	}

	for _, table := range expectedTables {
		var count int
		s.db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count)
		assert.Equal(t, 1, count, "table %s should exist", table)
	}
}

// T015: Verify FTS5 virtual table is created
func TestFTS5TableCreated(t *testing.T) {
	s, err := New(":memory:")
	require.NoError(t, err)

	var count int
	s.db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='test_cases_fts'").Scan(&count)
	assert.Equal(t, 1, count, "test_cases_fts FTS5 table should exist")
}
