package models

import (
	"fmt"
	"time"
)

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
	AllowAutoFailureAnalysis bool `json:"allow_auto_failure_analysis" gorm:"default:false;not null"`
	// Optional per-million-token prices (USD) for configured-cost estimation.
	// nil = unconfigured; cost reporting stays empty for this provider.
	PromptPricePerMTok     *float64  `json:"prompt_price_per_mtok"     gorm:"column:prompt_price_per_mtok;default:null"`
	CompletionPricePerMTok *float64  `json:"completion_price_per_mtok" gorm:"column:completion_price_per_mtok;default:null"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`

	// APIKeyStatus is set by the store when it reads the row: missing | ok | undecryptable.
	// A key that can't be decrypted is read back as "", never as its ciphertext.
	APIKeyStatus string `json:"-" gorm:"-"`
}

// LLMProviderConfigResponse is the safe, serialisable view of LLMProviderConfig.
// The full api_key is never included — only the last 4 characters are shown.
type LLMProviderConfigResponse struct {
	ID                       string    `json:"id"`
	Label                    string    `json:"label"`
	ProviderType             string    `json:"provider_type"`
	EndpointURL              string    `json:"endpoint_url"`
	APIKeyMasked             string    `json:"api_key_masked"`
	APIKeyStatus             string    `json:"api_key_status"` // missing | ok | undecryptable
	ModelName                string    `json:"model_name"`
	TimeoutSeconds           int       `json:"timeout_seconds"`
	IsDefault                bool      `json:"is_default"`
	Enabled                  bool      `json:"enabled"`
	AllowAutoFailureAnalysis bool      `json:"allow_auto_failure_analysis"`
	PromptPricePerMTok       *float64  `json:"prompt_price_per_mtok"`
	CompletionPricePerMTok   *float64  `json:"completion_price_per_mtok"`
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
		APIKeyStatus:             secretStatus(c.APIKeyStatus, c.APIKey),
		ModelName:                c.ModelName,
		TimeoutSeconds:           c.TimeoutSeconds,
		IsDefault:                c.IsDefault,
		Enabled:                  c.Enabled,
		AllowAutoFailureAnalysis: c.AllowAutoFailureAnalysis,
		PromptPricePerMTok:       c.PromptPricePerMTok,
		CompletionPricePerMTok:   c.CompletionPricePerMTok,
		CreatedAt:                c.CreatedAt,
		UpdatedAt:                c.UpdatedAt,
	}
}

// KeyError is what a call that sends the key to the provider must return instead of calling:
// non-nil only when the stored key can't be decrypted. Safe on a nil config.
func (c *LLMProviderConfig) KeyError() error {
	if c == nil {
		return nil
	}
	return secretError(fmt.Sprintf("LLM provider %q API key", c.Label), c.APIKeyStatus)
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

// Failure groups a job analyzes at once. Each group's TypeSafe and LLM calls run in their own
// goroutine; the store writes stay on one. The cap keeps a job inside common provider rate limits.
const (
	DefaultParallelGroups = 4
	MaxParallelGroups     = 8
)

// AIFailureAnalysisSettings stores admin configuration for the AI failure-analysis feature.
// Singleton pattern — single row with fixed ID "singleton".
type AIFailureAnalysisSettings struct {
	ID                    string    `json:"id"                      gorm:"primaryKey"` // always "singleton"
	EnabledOnCompletion   bool      `json:"enabled_on_completion"   gorm:"not null;default:false"`
	MaxAnalysesPerRun     int       `json:"max_analyses_per_run"    gorm:"not null;default:20"`
	ParallelGroups        int       `json:"parallel_groups"         gorm:"not null;default:4"` // failure groups analyzed at once by a job, 1..MaxParallelGroups
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
// the box. When disabled the UI hides every AI surface; failure analysis also
// enforces it server-side (no job runs, single-result analysis and Explain
// answer 409, nothing is queued on run completion). Test generation and AI
// import still rely on the frontend check.
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

// SuggestionSourceVerdict marks a suggestion derived from the verdict (see RunResultAnalysis).
const SuggestionSourceVerdict = "verdict"

// SuggestedDefectType maps an AI failure-analysis verdict to the defect_type it suggests.
// The mapping is lossy (6 verdicts -> 4 defect types): flaky_test and test_data both suggest
// "automation_bug"; environment and infrastructure both suggest "system_issue". "unknown" and
// any unrecognized value yield "" — no suggestion is offered rather than a wrong one.
// This is the single source of truth for the mapping; the frontend never maps.
func SuggestedDefectType(verdict string) string {
	switch verdict {
	case VerdictProductBug:
		return "product_bug"
	case VerdictFlakyTest, VerdictTestData:
		return "automation_bug"
	case VerdictEnvironment, VerdictInfrastructure:
		return "system_issue"
	default:
		return ""
	}
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

// Engines that can produce a failure-analysis verdict.
const (
	AnalysisEngineGenerative = "generative" // the configured chat LLM decided the verdict
	AnalysisEngineTypeSafe   = "typesafe"   // TypeSafe.ai System One decided; the LLM only narrated
	// AnalysisEngineTypeSafeDerived is a triage-snapshot bucket, not an analysis engine: a
	// TypeSafe row whose suggestion was derived from a confident verdict because the
	// defect-type question abstained. Graded apart from question-answered suggestions.
	AnalysisEngineTypeSafeDerived = "typesafe-derived"
)

// Narrative outcomes for a TypeSafe-decided analysis. Generative rows are always "ok".
const (
	NarrativeStatusOK          = "ok"
	NarrativeStatusUnavailable = "unavailable" // no narrative provider, or it failed
	NarrativeStatusUnparseable = "unparseable" // narrative came back but was not valid JSON twice
	NarrativeStatusSkipped     = "skipped"     // explanations switched off in TypeSafe settings; decision only
)

// Decision outcome of an analysis attempt. "unknown" is a model's answer; a failed attempt
// (provider error, reply cut off or unreadable twice, TypeSafe unavailable with no LLM
// fallback) produced no decision at all.
const (
	DecisionStatusOK     = "ok"
	DecisionStatusFailed = "failed"
)

// How a dedup clone was attached to its representative.
const (
	DedupMethodSignature = "signature" // identical SHA-1 signature (regex-normalized message)
	DedupMethodSemantic  = "semantic"  // TypeSafe judged the two failures to share a cause
)

// Job status / trigger constants.
const (
	RunAnalysisJobStatusQueued    = "queued"
	RunAnalysisJobStatusRunning   = "running"
	RunAnalysisJobStatusCompleted = "completed"
	RunAnalysisJobStatusFailed    = "failed"
	RunAnalysisJobStatusCancelled = "cancelled"
	// RunAnalysisJobStatusSkipped: an automatic analysis that was never queued because it would
	// have taken the month over the AI budget. Terminal from the start; never counts as active.
	RunAnalysisJobStatusSkipped = "skipped"

	RunAnalysisJobSkipReasonBudget = "budget"

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

	// Engine that produced Verdict. Legacy rows read as generative (column default).
	Engine string `json:"engine" gorm:"not null;default:'generative'"`
	// TypeSafe verdict confidence 0..1; NULL for generative rows (never invented).
	ConfidenceScore      *float64 `json:"confidence_score,omitempty"`
	VerdictProbabilities string   `json:"verdict_probabilities,omitempty" gorm:"type:text"`

	// SuggestedDefectType is PERSISTED and is the single source of truth for the suggestion.
	// typesafe rows: the decided option, or "" when TypeSafe abstained.
	// generative rows: SuggestedDefectType(Verdict), written at analysis time (and backfilled).
	// No read path may derive it from Verdict any more.
	SuggestedDefectType           string   `json:"suggested_defect_type" gorm:"default:''"`
	SuggestedDefectTypeConfidence *float64 `json:"suggested_defect_type_confidence,omitempty"`
	// SuggestionSource is "" when the suggestion came from the defect-type question (or the
	// generative mapping) and SuggestionSourceVerdict when it was derived from a confident
	// TypeSafe verdict because the question abstained (policy v4, verdict >= 0.90) or abstained
	// or disagreed (v5, verdict >= failureanalysis.VerdictDecidesSuggestionMin).
	SuggestionSource        string `json:"suggestion_source,omitempty" gorm:"default:''"`
	DefectTypeProbabilities string `json:"defect_type_probabilities,omitempty" gorm:"type:text"`

	NarrativeStatus     string `json:"narrative_status" gorm:"not null;default:'ok'"`
	PolicyVersion       string `json:"policy_version,omitempty"`
	TypeSafeInputTokens int    `json:"typesafe_input_tokens"`

	// Grouping provenance. "" on representatives; on clones: how they were grouped and,
	// for semantic clones, the probability/model/policy that justified the merge.
	DedupMethod        string   `json:"dedup_method,omitempty" gorm:"default:''"`
	DedupPSame         *float64 `json:"dedup_p_same,omitempty"`
	DedupModel         string   `json:"dedup_model,omitempty"`
	DedupPolicyVersion string   `json:"dedup_policy_version,omitempty"`

	// DecisionStatus separates a decision ("ok") from an attempt that produced none
	// ("failed"). A failed row keeps Verdict "unknown" only for compatibility: readers must
	// check this field, never the verdict, an empty model name or the summary text.
	DecisionStatus string `json:"decision_status" gorm:"not null;default:'ok'"`
	// ErrorCategory names why a failed attempt failed (timeout, rate_limit, truncated,
	// unparseable, ...), or why a kept TypeSafe decision has no LLM takeover.
	ErrorCategory string `json:"error_category,omitempty" gorm:"default:''"`
	// JobID is the analysis job that produced the row; nil for single-result analyses.
	JobID *string `json:"job_id,omitempty" gorm:"index"`
	// Takeover provenance: TypeSafe's own decision when its confidence was below the
	// takeover threshold and the LLM decided instead. Empty on every other row.
	TakeoverFromVerdict    string   `json:"takeover_from_verdict,omitempty" gorm:"default:''"`
	TakeoverFromConfidence *float64 `json:"takeover_from_confidence,omitempty"`
	TakeoverFromDefectType string   `json:"takeover_from_defect_type,omitempty" gorm:"default:''"`
	// Stage timing and call accounting for this attempt (representatives only; clones are 0).
	DecisionMs   int    `json:"decision_ms,omitempty" gorm:"column:decision_ms"`
	LLMMs        int    `json:"llm_ms,omitempty" gorm:"column:llm_ms"`
	LLMCalls     int    `json:"llm_calls,omitempty" gorm:"column:llm_calls"`
	FinishReason string `json:"finish_reason,omitempty" gorm:"column:finish_reason"`
}

// Failed reports whether the analysis attempt produced no decision.
func (a *RunResultAnalysis) Failed() bool {
	return a != nil && a.DecisionStatus == DecisionStatusFailed
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

	SemanticInputTokens int `json:"semantic_input_tokens" gorm:"default:0"` // TypeSafe tokens used by semantic grouping

	// Pipeline is a JSON snapshot of the route the job ran with (decider, narrator,
	// explanations, takeover threshold, fallback, reply cap), PipelineLabel its short name.
	Pipeline      string `json:"pipeline,omitempty" gorm:"type:text"`
	PipelineLabel string `json:"pipeline_label,omitempty"`
	// RetryFailedOnly: re-analyze only the groups whose current analysis failed.
	RetryFailedOnly bool `json:"retry_failed_only" gorm:"not null;default:false"`

	// Skip record (status skipped): why, and the figures the decision used — this run's
	// estimate, the month's spend so far and the monthly budget then in force.
	SkipReason      string   `json:"skip_reason" gorm:"not null;default:''"`
	SkipEstimateUSD *float64 `json:"skip_estimate_usd"`
	SkipSpentUSD    *float64 `json:"skip_spent_usd"`
	SkipBudgetUSD   *float64 `json:"skip_budget_usd"`
}

// RunAnalysisJobOutcomes summarises what a job's representative analyses produced.
type RunAnalysisJobOutcomes struct {
	Groups             int `json:"groups"`              // representative analyses the job stored
	Decided            int `json:"decided"`             // a decision, including a valid "unknown"
	Failed             int `json:"failed"`              // no decision: the attempt failed
	Unknown            int `json:"unknown"`             // decided "unknown" (a valid abstention)
	NoExplanation      int `json:"no_explanation"`      // decided, explanation unavailable or unreadable
	ExplanationSkipped int `json:"explanation_skipped"` // decided, explanations switched off
	TakenOver          int `json:"taken_over"`          // decided by the LLM below the TypeSafe threshold
	FailedRows         int `json:"failed_rows"`         // failing results left without a decision

	// FailedConfiguration counts representatives that failed on a settings problem (missing
	// or undecryptable key, unknown model id): retrying cannot help until the settings change.
	FailedConfiguration int `json:"failed_configuration"`
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
	SourceRefs  []string        `json:"source_refs,omitempty"`
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
