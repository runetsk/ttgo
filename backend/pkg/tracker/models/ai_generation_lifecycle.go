package models

import (
	"encoding/json"
	"time"
)

// AIGenerationRun status values.
const (
	AIGenerationRunStatusPending   = "pending"
	AIGenerationRunStatusRunning   = "running"
	AIGenerationRunStatusCompleted = "completed"
	AIGenerationRunStatusFailed    = "failed"
	AIGenerationRunStatusCancelled = "cancelled"
)

// AIGeneratedDraft status values.
const (
	AIDraftStatusPending    = "pending"
	AIDraftStatusAccepted   = "accepted"
	AIDraftStatusRejected   = "rejected"
	AIDraftStatusSuperseded = "superseded"
)

// AIGenerationEvent types (append-only lifecycle audit trail).
const (
	AIGenEventGenerated   = "generated"
	AIGenEventValidated   = "validated"
	AIGenEventEdited      = "edited"
	AIGenEventRegenerated = "regenerated"
	AIGenEventRejected    = "rejected"
	AIGenEventAccepted    = "accepted"
	AIGenEventRestored    = "restored"
)

// AIRejectionReasons is the structured rejection taxonomy.
var AIRejectionReasons = map[string]bool{
	"duplicate": true, "irrelevant": true, "incorrect": true, "too_vague": true,
	"incomplete_coverage": true, "poor_steps": true, "other": true,
}

// AIGenerationRun stores one durable generation attempt. Credentials are never
// stored here; RequestContext holds the exact rendered prompt (already exposed
// by the prompt-preview UI). Raw provider responses are NOT persisted.
type AIGenerationRun struct {
	ID             string  `json:"id"              gorm:"primaryKey"`
	IdempotencyKey string  `json:"idempotency_key" gorm:"uniqueIndex;not null"`
	RequirementID  string  `json:"requirement_id"  gorm:"index;not null"`
	UserID         *string `json:"user_id,omitempty"`
	ProviderID     *string `json:"provider_id,omitempty"`
	ProviderLabel  string  `json:"provider_label"`
	ProviderType   string  `json:"provider_type"`
	ModelName      string  `json:"model_name"`

	TemplateType    string `json:"template_type"`                     // "standard" | "parent"
	TemplateVersion int    `json:"template_version" gorm:"default:1"` // reserved; templates are unversioned today
	TemplateHash    string `json:"template_hash"`                     // sha256 hex of the template content used

	CoverageLevel          string `json:"coverage_level"`
	DetailLevel            string `json:"detail_level"`
	AdditionalInstructions string `json:"additional_instructions" gorm:"type:text"`
	MaxTokens              int    `json:"max_tokens_budget"`
	RequestContext         string `json:"request_context" gorm:"type:text"`

	Status      string     `json:"status" gorm:"index;not null;default:'pending'"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	DurationMs  int64      `json:"duration_ms"`

	PromptTokens     int      `json:"prompt_tokens"`
	CompletionTokens int      `json:"completion_tokens"`
	TotalTokens      int      `json:"total_tokens"`
	EstimatedCost    *float64 `json:"estimated_cost,omitempty"` // populated once provider pricing config ships

	RetryCount    int    `json:"retry_count"`
	FinishReason  string `json:"finish_reason"`
	ErrorCategory string `json:"error_category,omitempty"`
	ErrorMessage  string `json:"error_message,omitempty" gorm:"type:text"`

	ParentRunID *string `json:"parent_run_id,omitempty" gorm:"index"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Populated on demand by GetGenerationRunWithDrafts — not persisted.
	Drafts []*AIGeneratedDraft `json:"drafts,omitempty" gorm:"-"`
}

// DraftContent is the serialize-only shape of a draft's editable content
// (VersionSnapshot pattern — see store/versions.go).
type DraftContent struct {
	Name        string          `json:"name"`
	Category    string          `json:"category"`
	Description string          `json:"description"`
	SourceRefs  []string        `json:"source_refs,omitempty"`
	Steps       []GeneratedStep `json:"steps"`
}

// AIGeneratedDraft stores one draft within a run. Current content lives in the
// columns below; OriginalJSON preserves the as-generated DraftContent snapshot.
type AIGeneratedDraft struct {
	ID       string `json:"id"       gorm:"primaryKey"`
	RunID    string `json:"run_id"   gorm:"index;not null"`
	Position int    `json:"position" gorm:"not null"`
	Version  int    `json:"version"  gorm:"not null;default:1"`

	Name           string `json:"name" gorm:"not null"`
	Category       string `json:"category"`
	Description    string `json:"description" gorm:"type:text"`
	StepsJSON      string `json:"-" gorm:"type:text"` // json.Marshal-ed []GeneratedStep
	SourceRefsJSON string `json:"-" gorm:"type:text"` // json.Marshal-ed []string
	OriginalJSON   string `json:"-" gorm:"type:text"` // json.Marshal-ed DraftContent (as generated)

	Status         string `json:"status" gorm:"index;not null;default:'pending'"`
	ValidationJSON string `json:"-" gorm:"type:text"` // json.Marshal-ed []aigen.Finding
	Edited         bool   `json:"edited" gorm:"default:false"`

	AcceptedTestCaseID *string `json:"accepted_test_case_id,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ApplyContent writes c into the draft's editable columns.
func (d *AIGeneratedDraft) ApplyContent(c DraftContent) error {
	steps := c.Steps
	if steps == nil {
		steps = []GeneratedStep{}
	}
	stepsJSON, err := json.Marshal(steps)
	if err != nil {
		return err
	}
	refs := c.SourceRefs
	if refs == nil {
		refs = []string{}
	}
	refsJSON, err := json.Marshal(refs)
	if err != nil {
		return err
	}
	d.Name = c.Name
	d.Category = c.Category
	d.Description = c.Description
	d.StepsJSON = string(stepsJSON)
	d.SourceRefsJSON = string(refsJSON)
	return nil
}

// Content decodes the editable columns back into a DraftContent.
func (d *AIGeneratedDraft) Content() (DraftContent, error) {
	c := DraftContent{
		Name: d.Name, Category: d.Category, Description: d.Description,
		SourceRefs: []string{}, Steps: []GeneratedStep{},
	}
	if d.StepsJSON != "" {
		if err := json.Unmarshal([]byte(d.StepsJSON), &c.Steps); err != nil {
			return c, err
		}
	}
	if d.SourceRefsJSON != "" {
		if err := json.Unmarshal([]byte(d.SourceRefsJSON), &c.SourceRefs); err != nil {
			return c, err
		}
	}
	return c, nil
}

// AIGeneratedDraftResponse is the API shape of a draft with decoded content.
type AIGeneratedDraftResponse struct {
	ID                 string          `json:"id"`
	RunID              string          `json:"run_id"`
	Position           int             `json:"position"`
	Version            int             `json:"version"`
	Name               string          `json:"name"`
	Category           string          `json:"category"`
	Description        string          `json:"description"`
	SourceRefs         []string        `json:"source_refs"`
	Steps              []GeneratedStep `json:"steps"`
	Status             string          `json:"status"`
	Findings           json.RawMessage `json:"findings,omitempty"`
	Edited             bool            `json:"edited"`
	AcceptedTestCaseID *string         `json:"accepted_test_case_id,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// ToResponse decodes the draft into its API shape.
func (d *AIGeneratedDraft) ToResponse() (*AIGeneratedDraftResponse, error) {
	c, err := d.Content()
	if err != nil {
		return nil, err
	}
	resp := &AIGeneratedDraftResponse{
		ID: d.ID, RunID: d.RunID, Position: d.Position, Version: d.Version,
		Name: c.Name, Category: c.Category, Description: c.Description,
		SourceRefs: c.SourceRefs, Steps: c.Steps,
		Status: d.Status, Edited: d.Edited, AcceptedTestCaseID: d.AcceptedTestCaseID,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
	if d.ValidationJSON != "" && d.ValidationJSON != "null" && d.ValidationJSON != "[]" {
		resp.Findings = json.RawMessage(d.ValidationJSON)
	}
	return resp, nil
}

// AIGenerationEvent is one append-only lifecycle audit record. Reason holds a
// structured code (e.g. rejection reason); prompt text is never copied here.
type AIGenerationEvent struct {
	ID           string    `json:"id"         gorm:"primaryKey"`
	RunID        string    `json:"run_id"     gorm:"index;not null"`
	DraftID      *string   `json:"draft_id,omitempty" gorm:"index"`
	EventType    string    `json:"event_type" gorm:"index;not null"`
	ActorID      *string   `json:"actor_id,omitempty"`
	Reason       string    `json:"reason,omitempty"`
	Note         string    `json:"note,omitempty" gorm:"type:text"`
	MetadataJSON string    `json:"-" gorm:"type:text"`
	CreatedAt    time.Time `json:"created_at"`
}
