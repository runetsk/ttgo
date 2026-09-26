package models

import "time"

// Failure-analysis cost ledger: what a billable call was for, and which engine billed it.
const (
	AnalysisCostKindAnalysis = "analysis" // a representative's decision and explanation
	AnalysisCostKindExplain  = "explain"  // an explanation written later for a stored decision
	AnalysisCostKindSemantic = "semantic" // one job's semantic grouping pass

	AnalysisCostEngineTypeSafe = "typesafe"
	AnalysisCostEngineLLM      = "llm"
)

// AIAnalysisCostEvent is one billable failure-analysis call, appended when the call returns,
// priced with the prices then in force (the failure-analysis twin of AIGenerationAttempt).
// Explain adds tokens to an analysis long after it was stored, so spend is dated here, never
// on the analysis row. EstimatedCost is nil when the LLM provider has no prices configured.
// Clones never have events; analyses from before the ledger have none (unpriced).
type AIAnalysisCostEvent struct {
	ID                  string    `json:"id"                    gorm:"primaryKey"`
	CreatedAt           time.Time `json:"created_at"            gorm:"index"`
	Kind                string    `json:"kind"                  gorm:"not null"`
	Engine              string    `json:"engine"                gorm:"not null"`
	RunID               string    `json:"run_id"                gorm:"index;not null"` // test run
	JobID               *string   `json:"job_id,omitempty"      gorm:"index"`
	AnalysisID          *string   `json:"analysis_id,omitempty"`
	ProviderID          *string   `json:"provider_id,omitempty"`
	Model               string    `json:"model"`
	PromptTokens        int       `json:"prompt_tokens"`
	CompletionTokens    int       `json:"completion_tokens"`
	TypeSafeInputTokens int       `json:"typesafe_input_tokens"`
	EstimatedCost       *float64  `json:"estimated_cost,omitempty"`
}

func (AIAnalysisCostEvent) TableName() string { return "ai_analysis_cost_events" }
