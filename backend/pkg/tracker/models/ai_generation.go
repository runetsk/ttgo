package models

import "time"

// LLMProviderConfig stores an LLM provider configuration.
// Multiple rows allowed, including multiple per provider type.
// APIKey is excluded from JSON responses (json:"-") — return masked value via LLMProviderConfigResponse.
type LLMProviderConfig struct {
	ID             string `json:"id"              gorm:"primaryKey"`
	Label          string `json:"label"           gorm:"uniqueIndex;not null"`
	ProviderType   string `json:"provider_type"   gorm:"not null"`       // local | openai | gemini | anthropic
	EndpointURL    string `json:"endpoint_url"`                          // required for local; pre-filled defaults for cloud
	APIKey         string `json:"-"               gorm:"column:api_key"` // never serialised
	ModelName      string `json:"model_name"      gorm:"not null"`
	TimeoutSeconds int    `json:"timeout_seconds" gorm:"not null;default:90"`
	IsDefault      bool   `json:"is_default"      gorm:"default:false"`
	Enabled        bool   `json:"enabled"         gorm:"default:true"`
	// Auto-failure-analysis opt-in. When false, the auto-on-completion hook will
	// NOT send prompts to this provider even if it is the default provider.
	// Defaults to false on existing rows for safe data-transmission behavior.
	AllowAutoFailureAnalysis bool      `json:"allow_auto_failure_analysis" gorm:"default:false;not null"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}

// LLMProviderConfigResponse is the safe, serialisable view of LLMProviderConfig.
// The full api_key is never included — only the last 4 characters are shown.
type LLMProviderConfigResponse struct {
	ID                       string    `json:"id"`
	Label                    string    `json:"label"`
	ProviderType             string    `json:"provider_type"`
	EndpointURL              string    `json:"endpoint_url"`
	APIKeyMasked             string    `json:"api_key_masked"`
	ModelName                string    `json:"model_name"`
	TimeoutSeconds           int       `json:"timeout_seconds"`
	IsDefault                bool      `json:"is_default"`
	Enabled                  bool      `json:"enabled"`
	AllowAutoFailureAnalysis bool      `json:"allow_auto_failure_analysis"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}

// MaskedConfig converts an LLMProviderConfig to its safe response form.
func (c *LLMProviderConfig) MaskedConfig() LLMProviderConfigResponse {
	masked := ""
	if len(c.APIKey) > 4 {
		masked = "****" + c.APIKey[len(c.APIKey)-4:]
	} else if c.APIKey != "" {
		masked = "****"
	}
	return LLMProviderConfigResponse{
		ID:                       c.ID,
		Label:                    c.Label,
		ProviderType:             c.ProviderType,
		EndpointURL:              c.EndpointURL,
		APIKeyMasked:             masked,
		ModelName:                c.ModelName,
		TimeoutSeconds:           c.TimeoutSeconds,
		IsDefault:                c.IsDefault,
		Enabled:                  c.Enabled,
		AllowAutoFailureAnalysis: c.AllowAutoFailureAnalysis,
		CreatedAt:                c.CreatedAt,
		UpdatedAt:                c.UpdatedAt,
	}
}

// AIGenTemplate stores the TestCaseGenerator prompt templates.
// Singleton pattern — single row with fixed ID "singleton".
// Content is for standard (single requirement) generation.
// ParentContent is for parent requirements with child issues — a lighter,
// children-focused template that avoids bloating the prompt.
type AIGenTemplate struct {
	ID                   string    `json:"id"                     gorm:"primaryKey"` // always "singleton"
	Content              string    `json:"content"                gorm:"type:text;not null"`
	DefaultContent       string    `json:"default_content"        gorm:"type:text;not null"`
	ParentContent        string    `json:"parent_content"         gorm:"type:text;not null;default:''"`
	DefaultParentContent string    `json:"default_parent_content" gorm:"type:text;not null;default:''"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// AIGenCoverageConfig stores per-level max_tokens for coverage levels.
// Singleton pattern — single row with fixed ID "singleton".
type AIGenCoverageConfig struct {
	ID                     string    `json:"id"                       gorm:"primaryKey"` // always "singleton"
	EssentialMaxTokens     int       `json:"essential_max_tokens"     gorm:"not null;default:4096"`
	ThoroughMaxTokens      int       `json:"thorough_max_tokens"      gorm:"not null;default:8192"`
	ComprehensiveMaxTokens int       `json:"comprehensive_max_tokens" gorm:"not null;default:16384"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

// AIFailureAnalysisSettings stores admin configuration for the AI failure-analysis feature.
// Singleton pattern — single row with fixed ID "singleton".
type AIFailureAnalysisSettings struct {
	ID                    string    `json:"id"                      gorm:"primaryKey"` // always "singleton"
	EnabledOnCompletion   bool      `json:"enabled_on_completion"   gorm:"not null;default:false"`
	MaxAnalysesPerRun     int       `json:"max_analyses_per_run"    gorm:"not null;default:20"`
	DedupEnabled          bool      `json:"dedup_enabled"           gorm:"not null;default:true"`
	RedactionEnabled      bool      `json:"redaction_enabled"       gorm:"not null;default:true"`
	PromptTemplate        string    `json:"prompt_template"         gorm:"type:text;not null"`
	DefaultPromptTemplate string    `json:"default_prompt_template" gorm:"type:text;not null"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// AIFeatureSettings is the global master switch for all AI capabilities
// (test generation, AI import, failure analysis). Singleton pattern — single
// row with fixed ID "singleton". Enabled defaults to true so AI is on out of
// the box. Enforcement is frontend-only: when disabled the UI hides every AI
// surface, but the AI endpoints remain callable.
type AIFeatureSettings struct {
	ID        string    `json:"id"      gorm:"primaryKey"` // always "singleton"
	Enabled   bool      `json:"enabled" gorm:"not null;default:true"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Verdict categories returned by the AI failure analyzer.
const (
	VerdictProductBug     = "product_bug"
	VerdictFlakyTest      = "flaky_test"
	VerdictEnvironment    = "environment"
	VerdictTestData       = "test_data"
	VerdictInfrastructure = "infrastructure"
	VerdictUnknown        = "unknown"
)

// ValidVerdicts is the set of allowed verdict values (used for parse validation).
var ValidVerdicts = map[string]bool{
	VerdictProductBug: true, VerdictFlakyTest: true, VerdictEnvironment: true,
	VerdictTestData: true, VerdictInfrastructure: true, VerdictUnknown: true,
}

// Confidence levels returned by the AI failure analyzer.
const (
	ConfidenceLow    = "low"
	ConfidenceMedium = "medium"
	ConfidenceHigh   = "high"
)

var ValidConfidences = map[string]bool{
	ConfidenceLow: true, ConfidenceMedium: true, ConfidenceHigh: true,
}

// Job status / trigger constants.
const (
	RunAnalysisJobStatusQueued    = "queued"
	RunAnalysisJobStatusRunning   = "running"
	RunAnalysisJobStatusCompleted = "completed"
	RunAnalysisJobStatusFailed    = "failed"
	RunAnalysisJobStatusCancelled = "cancelled"

	RunAnalysisJobTriggerManual     = "manual"
	RunAnalysisJobTriggerAutoOnDone = "auto_on_completion"
)

// RunResultAnalysis is one (versioned) AI analysis of a RunResult.
// Append-only; newest version = current. Dedup clones have dedup_group_key + source_analysis_id
// populated and reuse the representative's verdict/summary/next_action.
type RunResultAnalysis struct {
	ID                   string    `json:"id"                    gorm:"primaryKey"`
	RunResultID          string    `json:"run_result_id"         gorm:"index;not null"`
	Version              int       `json:"version"               gorm:"not null"`
	Verdict              string    `json:"verdict"               gorm:"not null"`
	Confidence           string    `json:"confidence"            gorm:"not null"`
	Summary              string    `json:"summary"               gorm:"type:text"`
	NextAction           string    `json:"next_action"           gorm:"type:text"`
	Rationale            string    `json:"rationale"             gorm:"type:text"`
	RawResponse          string    `json:"raw_response,omitempty" gorm:"type:text"`
	ModelName            string    `json:"model_name"`
	ProviderID           *string   `json:"provider_id,omitempty"`
	TokenUsagePrompt     int       `json:"token_usage_prompt"`
	TokenUsageCompletion int       `json:"token_usage_completion"`
	DedupGroupKey        *string   `json:"dedup_group_key,omitempty"`
	SourceAnalysisID     *string   `json:"source_analysis_id,omitempty"`
	CreatedBy            *string   `json:"created_by,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
}

// RunAnalysisJob tracks a batch/auto analysis of a TestRun.
type RunAnalysisJob struct {
	ID            string     `json:"id"               gorm:"primaryKey"`
	TestRunID     string     `json:"test_run_id"      gorm:"index;not null"`
	Trigger       string     `json:"trigger"          gorm:"not null"`
	Status        string     `json:"status"           gorm:"index;not null"`
	TotalFailures int        `json:"total_failures"`
	UniqueGroups  int        `json:"unique_groups"`
	AnalyzedCount int        `json:"analyzed_count"`
	CappedAt      int        `json:"capped_at"`
	ErrorMessage  string     `json:"error_message,omitempty" gorm:"type:text"`
	ProviderID    *string    `json:"provider_id,omitempty"`
	ModelName     string     `json:"model_name"`
	CreatedBy     *string    `json:"created_by,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
}

// GeneratedStep is a single step in a generated test case draft.
type GeneratedStep struct {
	Action         string `json:"action"`
	ExpectedResult string `json:"expected_result"`
}

// GeneratedTestCase is the transient DTO returned by the generation and import endpoints.
// Lives only in the HTTP response and frontend state — not persisted.
type GeneratedTestCase struct {
	TempID      string          `json:"temp_id"`
	Name        string          `json:"name"`
	Category    string          `json:"category"`
	Description string          `json:"description"`
	Steps       []GeneratedStep `json:"steps"`
}

// ────────────────────────────────────────────────────────────────────────────
// 014-ai-test-import: Transient DTOs for the AI import flow
// ────────────────────────────────────────────────────────────────────────────

// ParseImportRequest is the payload sent to POST /api/import/parse.
type ParseImportRequest struct {
	Content    string `json:"content"`     // Raw pasted text or file content
	FormatHint string `json:"format_hint"` // Optional: "json", "csv", "markdown_table", "numbered_list", "" (auto-detect)
	FolderID   string `json:"folder_id"`   // Optional: target folder for duplicate name detection
}

// ParseImportResponse is the payload returned by POST /api/import/parse.
type ParseImportResponse struct {
	DetectedFormat string                 `json:"detected_format"` // "json", "csv", "markdown_table", "numbered_list", "ai"
	TestCases      []GeneratedTestCase    `json:"test_cases"`      // Parsed test cases (reuses existing DTO)
	Unparseable    []UnparseableItem      `json:"unparseable"`     // Items that couldn't be parsed
	DuplicateNames []string               `json:"duplicate_names"` // Names matching existing TCs in target folder
	TotalFound     int                    `json:"total_found"`     // Total items found before 50-cap
	Truncated      bool                   `json:"truncated"`       // True if total_found > 50
	Debug          map[string]interface{} `json:"debug,omitempty"` // LLM feedback (only when format is "ai")
}

// UnparseableItem represents a piece of content that the parser could not convert
// into a structured test case.
type UnparseableItem struct {
	LineNumber int    `json:"line_number"` // Approximate line in the original content
	RawText    string `json:"raw_text"`    // Original text that couldn't be parsed
	Reason     string `json:"reason"`      // Why parsing failed
}

// AcceptImportRequest is the payload sent to POST /api/import/accept.
type AcceptImportRequest struct {
	FolderID      string              `json:"folder_id"`                // Required: target folder
	RequirementID string              `json:"requirement_id,omitempty"` // Optional: link to requirement
	Tests         []GeneratedTestCase `json:"tests"`                    // Selected & edited test cases
}

// AcceptImportResponse is the payload returned by POST /api/import/accept.
type AcceptImportResponse struct {
	CreatedIDs []string `json:"created_ids"`
	Count      int      `json:"count"`
	LinkedTo   string   `json:"linked_to,omitempty"` // Requirement ID if linked
}
