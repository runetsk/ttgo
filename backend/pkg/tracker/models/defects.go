package models

import (
	"errors"
	"time"
)

// ErrDuplicateDefectLink is returned when a defect is already linked to the
// same run result (or the same test case at case scope).
var ErrDuplicateDefectLink = errors.New("this defect is already linked")

// Defect is a native, tracker-owned defect. It works with no external config
// and may optionally carry one reference-only external link.
type Defect struct {
	ID          string `json:"id"          gorm:"primaryKey"`
	Title       string `json:"title"       gorm:"not null"`
	Description string `json:"description" gorm:"type:text"`
	Status      string `json:"status"      gorm:"not null;default:'open';index"`  // "open" | "closed"
	Severity    string `json:"severity"    gorm:"not null;default:'minor';index"` // critical|major|minor|trivial

	ExternalProvider string `json:"external_provider"`
	ExternalKey      string `json:"external_key"`
	ExternalURL      string `json:"external_url"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	LinkedTestCount int `json:"linked_test_count" gorm:"-"` // computed: distinct test cases
}

// DefectLink joins a Defect to a test case and/or a specific run result.
// Result-scoped: both RunResultID and TestCaseID set. Test-case-scoped:
// TestCaseID set, RunResultID nil.
type DefectLink struct {
	ID          string    `json:"id"            gorm:"primaryKey"`
	DefectID    string    `json:"defect_id"     gorm:"index;not null"`
	TestCaseID  *string   `json:"test_case_id"  gorm:"index"`
	RunResultID *string   `json:"run_result_id" gorm:"index"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreateDefectRequest struct {
	Title            string `json:"title"`
	Description      string `json:"description"`
	Severity         string `json:"severity"`
	Status           string `json:"status"`
	ExternalProvider string `json:"external_provider"`
	ExternalKey      string `json:"external_key"`
	ExternalURL      string `json:"external_url"`
}

// UpdateDefectRequest — nil fields are left unchanged.
type UpdateDefectRequest struct {
	Title            *string `json:"title"`
	Description      *string `json:"description"`
	Severity         *string `json:"severity"`
	Status           *string `json:"status"`
	ExternalProvider *string `json:"external_provider"`
	ExternalKey      *string `json:"external_key"`
	ExternalURL      *string `json:"external_url"`
}

type LinkDefectRequest struct {
	DefectID string `json:"defect_id"`
}

// RunDefectRow is a flat row for "all defects in a run".
type RunDefectRow struct {
	Defect
	TestCaseID       string `json:"test_case_id"`
	TestNameSnapshot string `json:"test_name_snapshot"`
	ResultStatus     string `json:"result_status"`
}
