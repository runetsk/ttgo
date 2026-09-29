package models

import "time"

// TypeSafeSettingsID is the fixed primary key of the single typesafe_settings row.
const TypeSafeSettingsID = "singleton"

// TypeSafeDefaultModel is pinned (not the moving "jev-latest" alias) because confidence
// thresholds are tuned per model version.
const TypeSafeDefaultModel = "jev-1.13.0"

// TypeSafeDefaultPricePerMTok is the seeded USD price per million TypeSafe input tokens.
const TypeSafeDefaultPricePerMTok = 0.042

// API key status values reported by TypeSafeSettingsResponse.
const (
	TypeSafeKeyStatusMissing       = "missing"
	TypeSafeKeyStatusOK            = "ok"
	TypeSafeKeyStatusUndecryptable = "undecryptable"
)

// TypeSafeSettings is the global TypeSafe.ai configuration: one hosted account, one key,
// per-feature switches. Singleton row with ID "singleton". Enabled is the vendor master
// switch; nothing is sent to TypeSafe while it is false.
type TypeSafeSettings struct {
	ID             string `json:"id" gorm:"primaryKey"`
	Enabled        bool   `json:"enabled" gorm:"not null;default:false"`
	APIKey         string `json:"-" gorm:"column:api_key"` // encrypted at rest (strict), never serialised
	Model          string `json:"model" gorm:"not null;default:'jev-1.13.0'"`
	TimeoutSeconds int    `json:"timeout_seconds" gorm:"not null;default:30"`
	// PricePerMTok is the USD price per million TypeSafe input tokens. Failure analysis uses it
	// to price each TypeSafe call in the cost ledger and to estimate a start against the AI
	// budgets. 0 = free.
	PricePerMTok float64 `json:"price_per_mtok" gorm:"column:price_per_mtok;not null;default:0.042"`
	// VerdictEngineEnabled: TypeSafe decides verdict + defect-type suggestion.
	VerdictEngineEnabled bool `json:"verdict_engine_enabled" gorm:"not null;default:true"`
	// NarrativeEnabled: after TypeSafe decides, the default LLM provider writes the summary,
	// next action and rationale. Off = the classification is stored without an explanation.
	// It does not by itself stop LLM calls: the takeover threshold and the fallback below
	// still use the LLM when they are on. Ignored while VerdictEngineEnabled is off (the LLM
	// then decides and explains in one call).
	NarrativeEnabled bool `json:"narrative_enabled" gorm:"not null;default:true"`
	// LLMFallbackEnabled: when TypeSafe cannot answer (network, rate limit, oversized state),
	// the default LLM decides instead. Off = the attempt is recorded as failed, so failure
	// data never reaches the LLM on that path. With NarrativeEnabled off and
	// EscalateBelowPct 0 as well, analysis is TypeSafe-only and needs no LLM provider.
	LLMFallbackEnabled bool `json:"llm_fallback_enabled" gorm:"not null;default:true"`
	// EscalateBelowPct: when TypeSafe's verdict confidence is below this percentage, the
	// default LLM provider decides instead, exactly as the generative pipeline would, and the
	// analysis notes TypeSafe's answer. 0 (the default) never escalates. Ignored while
	// VerdictEngineEnabled is off or no LLM provider is configured.
	EscalateBelowPct int `json:"escalate_below_pct" gorm:"not null;default:0"`
	// SemanticDedupEnabled: TypeSafe merges failure groups sharing a cause (also needs
	// AIFailureAnalysisSettings.DedupEnabled).
	SemanticDedupEnabled bool `json:"semantic_dedup_enabled" gorm:"not null;default:true"`
	// AllowAutoFailureAnalysis is the per-vendor data-transmission consent for jobs
	// triggered automatically on run completion. Manual jobs need only Enabled.
	// Beyond failure analysis (Wave 5): each use of TypeSafe elsewhere in ttgo has its own switch.
	// ImportStructureEnabled: classify the lines of an unrecognized import before the LLM fallback.
	// DraftReviewEnabled: rate AI-generated drafts and check their duplicates.
	// DefectAssistEnabled: suggest a new defect's severity and find duplicate defects.
	// SearchRerankEnabled: re-rank the first page of test-case search (one request per search).
	ImportStructureEnabled   bool      `json:"import_structure_enabled" gorm:"not null;default:true"`
	DraftReviewEnabled       bool      `json:"draft_review_enabled" gorm:"not null;default:true"`
	DefectAssistEnabled      bool      `json:"defect_assist_enabled" gorm:"not null;default:true"`
	SearchRerankEnabled      bool      `json:"search_rerank_enabled" gorm:"not null;default:false"`
	AllowAutoFailureAnalysis bool      `json:"allow_auto_failure_analysis" gorm:"not null;default:false"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}

// TypeSafeSettingsResponse is the safe view: no key, a masked tail and a status.
type TypeSafeSettingsResponse struct {
	ID                       string    `json:"id"`
	Enabled                  bool      `json:"enabled"`
	APIKeyMasked             string    `json:"api_key_masked"`
	APIKeyStatus             string    `json:"api_key_status"` // missing | ok | undecryptable
	Model                    string    `json:"model"`
	TimeoutSeconds           int       `json:"timeout_seconds"`
	PricePerMTok             float64   `json:"price_per_mtok"`
	VerdictEngineEnabled     bool      `json:"verdict_engine_enabled"`
	NarrativeEnabled         bool      `json:"narrative_enabled"`
	LLMFallbackEnabled       bool      `json:"llm_fallback_enabled"`
	EscalateBelowPct         int       `json:"escalate_below_pct"`
	SemanticDedupEnabled     bool      `json:"semantic_dedup_enabled"`
	AllowAutoFailureAnalysis bool      `json:"allow_auto_failure_analysis"`
	ImportStructureEnabled   bool      `json:"import_structure_enabled"`
	DraftReviewEnabled       bool      `json:"draft_review_enabled"`
	DefectAssistEnabled      bool      `json:"defect_assist_enabled"`
	SearchRerankEnabled      bool      `json:"search_rerank_enabled"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}

// TypeSafeSettingsPatch is a partial update; nil fields are left alone. A nil or empty
// APIKey preserves the stored key; ClearAPIKey blanks it. Both together is an error.
type TypeSafeSettingsPatch struct {
	Enabled                  *bool    `json:"enabled"`
	APIKey                   *string  `json:"api_key"`
	ClearAPIKey              bool     `json:"clear_api_key"`
	Model                    *string  `json:"model"`
	TimeoutSeconds           *int     `json:"timeout_seconds"`
	PricePerMTok             *float64 `json:"price_per_mtok"`
	VerdictEngineEnabled     *bool    `json:"verdict_engine_enabled"`
	NarrativeEnabled         *bool    `json:"narrative_enabled"`
	LLMFallbackEnabled       *bool    `json:"llm_fallback_enabled"`
	EscalateBelowPct         *int     `json:"escalate_below_pct"`
	SemanticDedupEnabled     *bool    `json:"semantic_dedup_enabled"`
	AllowAutoFailureAnalysis *bool    `json:"allow_auto_failure_analysis"`
	ImportStructureEnabled   *bool    `json:"import_structure_enabled"`
	DraftReviewEnabled       *bool    `json:"draft_review_enabled"`
	DefectAssistEnabled      *bool    `json:"defect_assist_enabled"`
	SearchRerankEnabled      *bool    `json:"search_rerank_enabled"`
}
