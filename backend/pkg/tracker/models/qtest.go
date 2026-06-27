package models

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// ────────────────────────────────────────────────────────────────────────────
// QTest Config (013-qtest-sync)
// ────────────────────────────────────────────────────────────────────────────

// QTestConfig holds the workspace-wide configuration for the optional QTest
// integration. At most one row exists (singleton pattern).
//
// TableName overrides GORM's default "q_test_configs" → "qtest_configs".
type QTestConfig struct {
	ID          string    `json:"id"           gorm:"primaryKey"`
	BaseURL     string    `json:"base_url"`
	Email       string    `json:"email"`
	APIToken    string    `json:"-"            gorm:"column:api_token"` // never serialised
	ProjectID   int64     `json:"project_id"`
	ProjectName string    `json:"project_name"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (QTestConfig) TableName() string { return "qtest_configs" }

// QTestConfigResponse is the safe, serialisable view of QTestConfig.
type QTestConfigResponse struct {
	ID             string                `json:"id"`
	BaseURL        string                `json:"base_url"`
	Email          string                `json:"email"`
	APITokenMasked string                `json:"api_token_masked"`
	ProjectID      int64                 `json:"project_id"`
	ProjectName    string                `json:"project_name"`
	Enabled        bool                  `json:"enabled"`
	Projects       []QTestEnabledProject `json:"projects,omitempty"`
	CreatedAt      time.Time             `json:"created_at"`
	UpdatedAt      time.Time             `json:"updated_at"`
}

// MaskedConfig converts a QTestConfig to its safe response form.
func (c *QTestConfig) MaskedConfig() QTestConfigResponse {
	masked := ""
	if len(c.APIToken) > 4 {
		masked = "****" + c.APIToken[len(c.APIToken)-4:]
	} else if c.APIToken != "" {
		masked = "****"
	}
	return QTestConfigResponse{
		ID:             c.ID,
		BaseURL:        c.BaseURL,
		Email:          c.Email,
		APITokenMasked: masked,
		ProjectID:      c.ProjectID,
		ProjectName:    c.ProjectName,
		Enabled:        c.Enabled,
		CreatedAt:      c.CreatedAt,
		UpdatedAt:      c.UpdatedAt,
	}
}

// ────────────────────────────────────────────────────────────────────────────
// QTest Enabled Projects (multi-project support)
// ────────────────────────────────────────────────────────────────────────────

// QTestEnabledProject represents a QTest project that has been enabled for sync.
type QTestEnabledProject struct {
	ID          string    `json:"id"           gorm:"primaryKey"`
	ProjectID   int64     `json:"project_id"   gorm:"uniqueIndex;not null"`
	ProjectName string    `json:"project_name" gorm:"not null"`
	IsDefault   bool      `json:"is_default"   gorm:"default:false"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (QTestEnabledProject) TableName() string { return "qtest_projects" }

// ────────────────────────────────────────────────────────────────────────────
// QTest Mapping (013-qtest-sync)
// ────────────────────────────────────────────────────────────────────────────

// QTestMapping links a TTGO test case to a QTest test case.
//
// TableName overrides GORM's default "q_test_mappings" → "qtest_mappings".
type QTestMapping struct {
	ID               string     `json:"id"                  gorm:"primaryKey"`
	TestCaseID       string     `json:"test_case_id"        gorm:"uniqueIndex;not null"`
	QTestTestCaseID  int64      `json:"qtest_test_case_id"  gorm:"not null"`
	QTestTestCasePID string     `json:"qtest_test_case_pid"`
	QTestModuleID    int64      `json:"qtest_module_id"`
	QTestModulePath  string     `json:"qtest_module_path"`
	QTestProjectID   int64      `json:"qtest_project_id"    gorm:"index;default:0"`
	QTestURL         string     `json:"qtest_url"`
	ContentHash      string     `json:"content_hash"        gorm:"not null"`
	SyncStatus       string     `json:"sync_status"         gorm:"not null;default:'synced';index"` // synced, changes_pending, broken
	LastSyncedAt     *time.Time `json:"last_synced_at,omitempty"`
	ErrorMessage     string     `json:"error_message,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func (QTestMapping) TableName() string { return "qtest_mappings" }

// ────────────────────────────────────────────────────────────────────────────
// Bulk operation DTOs (013-qtest-sync)
// ────────────────────────────────────────────────────────────────────────────

// QTestBulkResult is the response for upload/sync operations.
type QTestBulkResult struct {
	Total       int                   `json:"total"`
	Succeeded   int                   `json:"succeeded"`
	Failed      int                   `json:"failed"`
	Skipped     int                   `json:"skipped"`
	RateLimited bool                  `json:"rate_limited"`
	Items       []QTestBulkResultItem `json:"items"`
}

// QTestBulkResultItem describes the result for a single test case in a bulk operation.
type QTestBulkResultItem struct {
	TestCaseID      string `json:"test_case_id"`
	TestCaseName    string `json:"test_case_name"`
	Status          string `json:"status"` // success, failed, skipped, rate_limited
	QTestTestCaseID int64  `json:"qtest_test_case_id,omitempty"`
	QTestURL        string `json:"qtest_url,omitempty"`
	Error           string `json:"error,omitempty"`
}

// QTestProject is a lightweight project representation from QTest API.
type QTestProject struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// QTestModule represents a node in QTest's module (folder) hierarchy.
type QTestModule struct {
	ID       int64          `json:"id"`
	Name     string         `json:"name"`
	ParentID *int64         `json:"parent_id,omitempty"`
	Path     string         `json:"path,omitempty"`
	Children []*QTestModule `json:"children,omitempty"`
}

// QTestRemoteTestCase represents a test case fetched from the QTest API.
type QTestRemoteTestCase struct {
	ID          int64             `json:"id"`
	PID         string            `json:"pid"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	ParentID    int64             `json:"parent_id"`
	ModuleID    int64             `json:"module_id,omitempty"`
	ModulePath  string            `json:"module_path,omitempty"`
	Properties  []QTestProperty   `json:"properties,omitempty"`
	Steps       []QTestRemoteStep `json:"steps,omitempty"`
}

// QTestRemoteStep represents a test step from a QTest test case.
type QTestRemoteStep struct {
	Description string `json:"description"`
	Expected    string `json:"expected"`
}

// QTestProperty represents a qTest property that may be mapped to a TTGO custom field.
type QTestProperty struct {
	Name      string          `json:"name"`
	Type      string          `json:"type,omitempty"`
	FieldType string          `json:"field_type,omitempty"`
	Value     json.RawMessage `json:"value,omitempty"`
	ValueText string          `json:"value_text,omitempty"`
	Raw       json.RawMessage `json:"-"`
}

// ────────────────────────────────────────────────────────────────────────────
// Content hash (013-qtest-sync)
// ────────────────────────────────────────────────────────────────────────────

// ComputeTestCaseContentHash produces a deterministic SHA-256 hash of the
// synced fields (name, description, steps) used for change detection.
func ComputeTestCaseContentHash(name, description string, steps []*TestStep) string {
	// Sort steps by OrderIndex to ensure deterministic order
	sorted := make([]*TestStep, len(steps))
	copy(sorted, steps)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].OrderIndex < sorted[j].OrderIndex
	})

	h := sha256.New()
	h.Write([]byte(name))
	h.Write([]byte("\n"))
	h.Write([]byte(description))
	for _, step := range sorted {
		h.Write([]byte("\n"))
		h.Write([]byte(step.Action))
		h.Write([]byte("|"))
		h.Write([]byte(step.ExpectedResult))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
