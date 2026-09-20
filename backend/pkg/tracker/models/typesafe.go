package models

import "time"

// TypeSafeSettingsID is the fixed primary key of the single typesafe_settings row.
const TypeSafeSettingsID = "singleton"

// TypeSafeDefaultModel is pinned (not the moving "jev-latest" alias) because confidence
// thresholds are tuned per model version.
const TypeSafeDefaultModel = "jev-1.13.0"

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
	// VerdictEngineEnabled: TypeSafe decides verdict + defect-type suggestion.
	VerdictEngineEnabled bool `json:"verdict_engine_enabled" gorm:"not null;default:true"`
	// SemanticDedupEnabled: TypeSafe merges failure groups sharing a cause (also needs
	// AIFailureAnalysisSettings.DedupEnabled).
	SemanticDedupEnabled bool `json:"semantic_dedup_enabled" gorm:"not null;default:true"`
	// AllowAutoFailureAnalysis is the per-vendor data-transmission consent for jobs
	// triggered automatically on run completion. Manual jobs need only Enabled.
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
	VerdictEngineEnabled     bool      `json:"verdict_engine_enabled"`
	SemanticDedupEnabled     bool      `json:"semantic_dedup_enabled"`
	AllowAutoFailureAnalysis bool      `json:"allow_auto_failure_analysis"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}

// TypeSafeSettingsPatch is a partial update; nil fields are left alone. A nil or empty
// APIKey preserves the stored key; ClearAPIKey blanks it. Both together is an error.
type TypeSafeSettingsPatch struct {
	Enabled                  *bool   `json:"enabled"`
	APIKey                   *string `json:"api_key"`
	ClearAPIKey              bool    `json:"clear_api_key"`
	Model                    *string `json:"model"`
	TimeoutSeconds           *int    `json:"timeout_seconds"`
	VerdictEngineEnabled     *bool   `json:"verdict_engine_enabled"`
	SemanticDedupEnabled     *bool   `json:"semantic_dedup_enabled"`
	AllowAutoFailureAnalysis *bool   `json:"allow_auto_failure_analysis"`
}
