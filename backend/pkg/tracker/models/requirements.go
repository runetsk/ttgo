package models

import (
	"errors"
	"fmt"
	"time"
)

// ErrDuplicateRequirementIdentifier is returned when a requirement with the
// same identifier already exists.
var ErrDuplicateRequirementIdentifier = errors.New("a requirement with this identifier already exists")

// ErrDuplicateLink is returned when the same test case is linked to the same
// requirement more than once.
var ErrDuplicateLink = errors.New("this test case is already linked to the requirement")

// Requirement represents an external or internal requirement, user story,
// or Jira ticket that test cases can be traced to.
// 011-jira-confluence-import: added source metadata fields for import tracking.
type Requirement struct {
	ID               string     `json:"id"          gorm:"primaryKey"`
	Identifier       string     `json:"identifier"  gorm:"uniqueIndex;not null"` // e.g. "PROJ-123"
	Title            string     `json:"title"       gorm:"not null"`
	Description      string     `json:"description" gorm:"type:text"`
	ParentID         *string    `json:"parent_id"   gorm:"index;default:null"`
	SourceType       string     `json:"source_type" gorm:"index;index:idx_requirement_source;default:''"` // "" (manual), "jira", or "confluence"
	SourceKey        string     `json:"source_key"  gorm:"index;index:idx_requirement_source;default:''"` // Jira ticket key or Confluence page ID
	SourceURL        string     `json:"source_url"  gorm:"default:''"`                                    // Full URL to source
	ImportedAt       *time.Time `json:"imported_at" gorm:"default:null"`                                  // Timestamp of last import/sync
	LastJiraPostHash string     `json:"-"           gorm:"default:''"`                                    // dedup marker for post-to-jira (F-056)
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// RequirementTestCaseLink is the junction table recording that a test case
// is linked to a requirement (many-to-many).
type RequirementTestCaseLink struct {
	ID            string    `json:"id"             gorm:"primaryKey"`
	RequirementID string    `json:"requirement_id" gorm:"index:idx_req_link_composite,unique;not null"`
	TestCaseID    string    `json:"test_case_id"   gorm:"index:idx_req_link_composite,unique;index;not null"`
	CreatedAt     time.Time `json:"created_at"`
}

// JiraConfig holds the workspace-wide configuration for the optional Jira
// integration. At most one row exists (singleton pattern).
// The APIToken field is excluded from JSON responses to prevent accidental
// leakage — return a masked value via a dedicated response DTO instead.
type JiraConfig struct {
	ID                string    `json:"id"                  gorm:"primaryKey"`
	BaseURL           string    `json:"base_url"`                                    // e.g. "https://company.atlassian.net"
	Email             string    `json:"email"`                                       // Jira user email for Basic Auth
	APIToken          string    `json:"-"                   gorm:"column:api_token"` // never serialised
	Enabled           bool      `json:"enabled"`
	DefaultProjectKey string    `json:"default_project_key" gorm:"default:''"`    // 008-jira-integration
	DefaultIssueType  string    `json:"default_issue_type"  gorm:"default:'Bug'"` // 008-jira-integration
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// JiraConfigResponse is the safe, serialisable view of JiraConfig returned by the API.
// The full api_token is never included — only the last 4 characters are shown (FR-012).
type JiraConfigResponse struct {
	ID                string    `json:"id"`
	BaseURL           string    `json:"base_url"`
	Email             string    `json:"email"`
	APITokenMasked    string    `json:"api_token_masked"`
	Enabled           bool      `json:"enabled"`
	DefaultProjectKey string    `json:"default_project_key"`
	DefaultIssueType  string    `json:"default_issue_type"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// MaskedConfig converts a JiraConfig to its safe response form.
// The API token is shown as "****XXXX" (last 4 chars only) per FR-012.
func (c *JiraConfig) MaskedConfig() JiraConfigResponse {
	masked := ""
	if len(c.APIToken) > 4 {
		masked = "****" + c.APIToken[len(c.APIToken)-4:]
	} else if c.APIToken != "" {
		masked = "****"
	}
	return JiraConfigResponse{
		ID:                c.ID,
		BaseURL:           c.BaseURL,
		Email:             c.Email,
		APITokenMasked:    masked,
		Enabled:           c.Enabled,
		DefaultProjectKey: c.DefaultProjectKey,
		DefaultIssueType:  c.DefaultIssueType,
		CreatedAt:         c.CreatedAt,
		UpdatedAt:         c.UpdatedAt,
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Confluence Config (011-jira-confluence-import)
// ────────────────────────────────────────────────────────────────────────────

// ConfluenceConfig holds the workspace-wide configuration for the optional
// Confluence integration. Singleton pattern, identical to JiraConfig.
type ConfluenceConfig struct {
	ID        string    `json:"id"        gorm:"primaryKey"`
	BaseURL   string    `json:"base_url"`
	Email     string    `json:"email"`
	APIToken  string    `json:"-"         gorm:"column:api_token"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ConfluenceConfigResponse is the safe, serialisable view of ConfluenceConfig.
type ConfluenceConfigResponse struct {
	ID        string    `json:"id"`
	BaseURL   string    `json:"base_url"`
	Email     string    `json:"email"`
	HasToken  bool      `json:"has_token"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ToResponse converts a ConfluenceConfig to its safe response form.
func (c *ConfluenceConfig) ToResponse() ConfluenceConfigResponse {
	return ConfluenceConfigResponse{
		ID:        c.ID,
		BaseURL:   c.BaseURL,
		Email:     c.Email,
		HasToken:  c.APIToken != "",
		Enabled:   c.Enabled,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

// ErrSourceAlreadyImported is returned when a requirement with the same
// source_type + source_key already exists.
var ErrSourceAlreadyImported = fmt.Errorf("a requirement from this source already exists")

// JiraTicketResult is returned by the Jira ticket-fetch proxy endpoint.
// success:false is returned (with HTTP 200) on any Jira-side failure so that
// the frontend can show an inline warning and fall back to manual entry (FR-011c).
// 011-jira-confluence-import: added key, status, url, already_imported, existing_requirement_id.
type JiraTicketResult struct {
	Success               bool    `json:"success"`
	Identifier            string  `json:"identifier"`
	Key                   string  `json:"key"`
	Title                 string  `json:"title"`
	Description           string  `json:"description"`
	Status                string  `json:"status"`
	URL                   string  `json:"url"`
	AlreadyImported       bool    `json:"already_imported"`
	ExistingRequirementID *string `json:"existing_requirement_id"`
	Error                 string  `json:"error"`
}

// JiraTicketChild is a lightweight representation of a child issue found under
// an epic or parent ticket, used for import previews and context assembly.
type JiraTicketChild struct {
	Key             string `json:"key"`
	Title           string `json:"title"`
	Status          string `json:"status"`
	URL             string `json:"url"`
	AlreadyImported bool   `json:"already_imported"`
}

// --- Traceability matrix response shapes (not persisted) ---

// LinkedTC is a lightweight reference to a test case inside a matrix row.
type LinkedTC struct {
	TestCaseID   string `json:"test_case_id"`
	TestCaseName string `json:"test_case_name"`
}

// MatrixRow represents one requirement and all its linked test cases as shown
// in the traceability matrix.
type MatrixRow struct {
	RequirementID   string     `json:"requirement_id"`
	Identifier      string     `json:"identifier"`
	Title           string     `json:"title"`
	Description     string     `json:"description"`
	Covered         bool       `json:"covered"` // true if len(LinkedTestCases) > 0
	LinkedTestCases []LinkedTC `json:"linked_test_cases"`
}

// CoverageSummary aggregates coverage statistics derived from the full matrix.
type CoverageSummary struct {
	Total      int     `json:"total"`
	Covered    int     `json:"covered"`
	Uncovered  int     `json:"uncovered"`
	Percentage float64 `json:"percentage"` // 0.0–100.0
}

// MatrixResponse is the top-level payload returned by GET /api/traceability.
type MatrixResponse struct {
	Rows    []*MatrixRow    `json:"rows"`
	Summary CoverageSummary `json:"summary"`
}
