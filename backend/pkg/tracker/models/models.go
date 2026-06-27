package models

import (
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrCircularReference = errors.New("circular reference detected: cannot move a folder into itself or its descendants")
)

// Folder represents a hierarchical container for test cases.
type Folder struct {
	ID        string    `json:"id" db:"id" gorm:"primaryKey"`
	Name      string    `json:"name" db:"name"`
	ParentID  *string   `json:"parent_id,omitempty" db:"parent_id" gorm:"index"` // Nullable for root folders
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`

	// Children fields for API responses (not stored in DB directly)
	SubFolders []*Folder   `json:"sub_folders,omitempty" gorm:"-"`
	TestCases  []*TestCase `json:"test_cases,omitempty" gorm:"-"`
}

// Category represents a tag or group for test cases (e.g., "Smoke", "Regression").
type Category struct {
	ID          string    `json:"id" db:"id" gorm:"primaryKey"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description,omitempty" db:"description"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`

	// Associations
	TestCases []*TestCase `json:"test_cases,omitempty" gorm:"many2many:suite_test_cases;joinForeignKey:SuiteID;joinReferences:TestCaseID"`
}

// TableName keeps the existing DB table name for backward compatibility.
func (Category) TableName() string { return "suites" }

// TestCase represents a specific test scenario.
type TestCase struct {
	ID          string    `json:"id" db:"id" gorm:"primaryKey"`
	FolderID    string    `json:"folder_id" db:"folder_id" gorm:"index"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description,omitempty" db:"description"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`

	// 008-jira-integration: set true when all linked defects reach statusCategory=="done".
	// Persists until manually dismissed via DELETE /api/tests/{id}/reverification-flag.
	ReverificationFlagged bool `json:"reverification_flagged" gorm:"default:false"`

	// Associations
	Categories   []*Category         `json:"categories,omitempty" gorm:"many2many:suite_test_cases;joinForeignKey:TestCaseID;joinReferences:SuiteID"`
	Steps        []*TestStep         `json:"steps,omitempty" gorm:"foreignKey:TestCaseID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	CustomValues []*CustomFieldValue `json:"custom_values,omitempty" gorm:"foreignKey:TestCaseID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	// Populated on demand by store.ListRequirementsByTestCase — not persisted.
	LinkedRequirements []*Requirement `json:"linked_requirements,omitempty" gorm:"-"`

	// 008-jira-integration: populated on demand by GetFolderTree — not persisted.
	OpenDefectCount   int `json:"open_defect_count,omitempty" gorm:"-"`
	ClosedDefectCount int `json:"closed_defect_count,omitempty" gorm:"-"`

	// Populated on demand by ListTestCases when called with TestCaseFilter.ListView=true.
	// Not persisted — avoids the cost of Preload("Steps") when the caller only needs the count.
	StepsCount int `json:"steps_count,omitempty" gorm:"-"`
}

// CategoryTestCase represents the many-to-many relationship between Categories and TestCases.
type CategoryTestCase struct {
	CategoryID string `json:"category_id" gorm:"column:suite_id;primaryKey"`
	TestCaseID string `json:"test_case_id" gorm:"primaryKey;index"`
}

// TableName keeps the existing DB table name for backward compatibility.
func (CategoryTestCase) TableName() string { return "suite_test_cases" }

// ExecutionStatus defines the possible outcomes of a test run.
type ExecutionStatus string

const (
	StatusPass    ExecutionStatus = "PASS"
	StatusFail    ExecutionStatus = "FAIL"
	StatusSkip    ExecutionStatus = "SKIP"
	StatusPending ExecutionStatus = "PENDING"
	StatusRunning ExecutionStatus = "RUNNING"
	StatusError   ExecutionStatus = "ERROR"
)

// ValidExecutionStatuses is the set of allowed execution status values.
var ValidExecutionStatuses = map[ExecutionStatus]bool{
	StatusPass:    true,
	StatusFail:    true,
	StatusSkip:    true,
	StatusPending: true,
	StatusRunning: true,
	StatusError:   true,
}

// IsValidExecutionStatus checks whether a string is a valid execution status.
func IsValidExecutionStatus(s string) bool {
	return ValidExecutionStatuses[ExecutionStatus(s)]
}

// ValidRunStatuses is the set of allowed test run statuses.
var ValidRunStatuses = map[ExecutionStatus]bool{
	StatusRunning: true,
	StatusPass:    true,
	StatusFail:    true,
}

// IsValidRunStatus checks whether a string is a valid test run status.
func IsValidRunStatus(s string) bool {
	return ValidRunStatuses[ExecutionStatus(s)]
}

// TestStep represents a single step in a test case.
type TestStep struct {
	ID             string `json:"id" gorm:"primaryKey"`
	TestCaseID     string `json:"test_case_id" gorm:"index"`
	Action         string `json:"action"`
	ExpectedResult string `json:"expected_result"`
	OrderIndex     int    `json:"order_index"`
}

// CustomFieldType defines the supported types for custom fields.
type CustomFieldType string

const (
	FieldTypeText     CustomFieldType = "TEXT"
	FieldTypeSelect   CustomFieldType = "SELECT"
	FieldTypeNumber   CustomFieldType = "NUMBER"
	FieldTypeDate     CustomFieldType = "DATE"
	FieldTypeCheckbox CustomFieldType = "CHECKBOX"
)

// CustomFieldDefinition represents a global custom field definition.
type CustomFieldDefinition struct {
	ID          string          `json:"id" gorm:"primaryKey"`
	Name        string          `json:"name"`
	Type        CustomFieldType `json:"type"`
	IsMandatory bool            `json:"is_mandatory"`
	Options     json.RawMessage `json:"options,omitempty" gorm:"type:json"` // Array of strings for SELECT type
	CreatedAt   time.Time       `json:"created_at"`
}

// CustomFieldValue represents a value for a specific custom field on a test case.
type CustomFieldValue struct {
	ID             string                 `json:"id" gorm:"primaryKey"`
	TestCaseID     string                 `json:"test_case_id" gorm:"index"`
	CustomFieldID  string                 `json:"custom_field_id" gorm:"index"`
	Value          json.RawMessage        `json:"value" gorm:"type:json"` // Stored as JSON to handle basic types
	CustomFieldDef *CustomFieldDefinition `json:"definition,omitempty" gorm:"foreignKey:CustomFieldID"`
}

// AuditLog represents a high-level change record for a test case.
type AuditLog struct {
	ID         string    `json:"id" gorm:"primaryKey"`
	TestCaseID string    `json:"test_case_id" gorm:"index"`
	Action     string    `json:"action"`            // e.g., "Steps updated", "Title changed"
	Diff       string    `json:"diff,omitempty"`    // simplified human-readable diff or JSON
	UserID     string    `json:"user_id,omitempty"` // For future auth integration
	Timestamp  time.Time `json:"timestamp"`
}

// Comment represents a user comment on a test run or run result.
type Comment struct {
	ID         string    `json:"id" gorm:"primaryKey"`
	TargetType string    `json:"target_type" gorm:"index:idx_comment_target;not null"` // "run" or "result"
	TargetID   string    `json:"target_id" gorm:"index:idx_comment_target;not null"`   // UUID of TestRun or RunResult
	UserID     string    `json:"user_id" gorm:"index"`
	Content    string    `json:"content" gorm:"type:text;not null"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`

	// Association — populated via Preload
	User *User `json:"user,omitempty" gorm:"foreignKey:UserID;references:ID"`
}

// RunFolder is a named container for test runs with unlimited nesting.
type RunFolder struct {
	ID           string    `json:"id"            gorm:"primaryKey"`
	ParentID     *string   `json:"parent_id,omitempty" gorm:"index"` // Nullable for root folders
	Name         string    `json:"name"`
	DisplayOrder int       `json:"display_order" gorm:"index"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// Children fields for API responses (not stored in DB directly)
	SubFolders []*RunFolder `json:"sub_folders,omitempty" gorm:"-"`
	TestRuns   []*TestRun   `json:"test_runs,omitempty" gorm:"-"`
}

// TestRun represents a snapshot execution of a Test Category (or an empty run).
type TestRun struct {
	ID          string          `json:"id"            gorm:"primaryKey"`
	Name        string          `json:"name"`
	CategoryID  *string         `json:"category_id"      gorm:"index"` // nullable — empty runs have no category
	RunFolderID *string         `json:"run_folder_id" gorm:"index"`    // nullable FK to RunFolder
	Status      ExecutionStatus `json:"status"`                        // PENDING, RUNNING, PASSED, FAILED, etc.
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`

	// Computed aggregate fields (not persisted, populated by GetTestRuns list query)
	TotalResults   int `json:"total_results" gorm:"-"`
	PassedResults  int `json:"passed_results" gorm:"-"`
	FailedResults  int `json:"failed_results" gorm:"-"`
	SkippedResults int `json:"skipped_results" gorm:"-"`
	PendingResults int `json:"pending_results" gorm:"-"`

	// Defect type breakdowns (not persisted, populated by GetTestRuns list query)
	ToInvestigate int `json:"to_investigate_count" gorm:"-"`
	ProductBug    int `json:"product_bug_count" gorm:"-"`
	AutomationBug int `json:"automation_bug_count" gorm:"-"`
	SystemIssue   int `json:"system_issue_count" gorm:"-"`

	// Comment count (not persisted, populated by list handler)
	CommentCount int64 `json:"comment_count" gorm:"-"`

	// Defect link counts (not persisted, populated by list handler)
	OpenDefectLinkCount   int `json:"open_defect_link_count" gorm:"-"`
	ClosedDefectLinkCount int `json:"closed_defect_link_count" gorm:"-"`

	// Retry stats (not persisted, populated by GetTestRun/GetTestRuns)
	RetriedCount  int `json:"retried_count" gorm:"-"`
	TotalAttempts int `json:"total_attempts" gorm:"-"`

	// Associations
	RunResults []*RunResult `json:"run_results,omitempty" gorm:"foreignKey:TestRunID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

// RunResult represents the result of a single test case within a Test Run.
type RunResult struct {
	ID               string          `json:"id" gorm:"primaryKey"`
	TestRunID        string          `json:"test_run_id" gorm:"index"`
	TestCaseID       *string         `json:"test_case_id" gorm:"index"` // Nullable if test case is deleted
	AttemptNumber    int             `json:"attempt_number" gorm:"default:1;not null"`
	TestNameSnapshot string          `json:"test_name_snapshot"` // Preserved name
	Status           ExecutionStatus `json:"status"`

	// Timing
	DurationMs int64     `json:"duration_ms"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`

	// Failure Details
	ErrorMessage string `json:"error_message"`
	StackTrace   string `json:"stack_trace"`
	FailureType  string `json:"failure_type"`

	// Artifacts
	Screenshots string `json:"screenshots" gorm:"type:text"` // JSON array of URL strings
	Video       string `json:"video"`
	TraceURL    string `json:"trace_url"`
	LogText     string `json:"log_text"`

	// Context
	Browser     string `json:"browser" gorm:"index"`
	OS          string `json:"os" gorm:"index"`
	Environment string `json:"environment" gorm:"index"`
	AppVersion  string `json:"app_version"`

	// DefectType classifies the failure reason for FAIL results.
	// Values: "to_investigate" | "product_bug" | "automation_bug" | "system_issue" | "" (not applicable)
	DefectType string `json:"defect_type" gorm:"default:''"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Detailed Steps & Metadata
	Steps    json.RawMessage `json:"steps" gorm:"type:json"`
	Metadata json.RawMessage `json:"metadata,omitempty" gorm:"type:json"`

	// Defect link counts (not persisted, populated by GetTestRun)
	OpenDefectLinkCount   int `json:"open_defect_link_count" gorm:"-"`
	ClosedDefectLinkCount int `json:"closed_defect_link_count" gorm:"-"`

	// Association (not stored, loaded via preload)
	TestCase *TestCase `json:"test_case,omitempty" gorm:"foreignKey:TestCaseID;references:ID"`
}

// CreateRunResultRequest is the request body for adding a new result to a test run.
type CreateRunResultRequest struct {
	TestCaseID       *string         `json:"test_case_id"`
	AttemptNumber    int             `json:"attempt_number,omitempty"`
	TestNameSnapshot string          `json:"test_name_snapshot,omitempty"`
	Status           ExecutionStatus `json:"status,omitempty"`
	DurationMs       int64           `json:"duration_ms,omitempty"`
	StartTime        time.Time       `json:"start_time,omitempty"`
	EndTime          time.Time       `json:"end_time,omitempty"`
	ErrorMessage     string          `json:"error_message,omitempty"`
	StackTrace       string          `json:"stack_trace,omitempty"`
	FailureType      string          `json:"failure_type,omitempty"`
	Screenshots      string          `json:"screenshots,omitempty"`
	Video            string          `json:"video,omitempty"`
	TraceURL         string          `json:"trace_url,omitempty"`
	LogText          string          `json:"log_text,omitempty"`
	Browser          string          `json:"browser,omitempty"`
	OS               string          `json:"os,omitempty"`
	Environment      string          `json:"environment,omitempty"`
	AppVersion       string          `json:"app_version,omitempty"`
	DefectType       string          `json:"defect_type,omitempty"`
	Steps            json.RawMessage `json:"steps,omitempty"`
	Metadata         json.RawMessage `json:"metadata,omitempty"`
}

// ToRunResult converts a create request into a RunResult model.
func (r *CreateRunResultRequest) ToRunResult(testRunID string) *RunResult {
	return &RunResult{
		TestRunID:        testRunID,
		TestCaseID:       r.TestCaseID,
		AttemptNumber:    r.AttemptNumber,
		TestNameSnapshot: r.TestNameSnapshot,
		Status:           r.Status,
		DurationMs:       r.DurationMs,
		StartTime:        r.StartTime,
		EndTime:          r.EndTime,
		ErrorMessage:     r.ErrorMessage,
		StackTrace:       r.StackTrace,
		FailureType:      r.FailureType,
		Screenshots:      r.Screenshots,
		Video:            r.Video,
		TraceURL:         r.TraceURL,
		LogText:          r.LogText,
		Browser:          r.Browser,
		OS:               r.OS,
		Environment:      r.Environment,
		AppVersion:       r.AppVersion,
		DefectType:       r.DefectType,
		Steps:            r.Steps,
		Metadata:         r.Metadata,
	}
}

// CreateCustomFieldRequest is the request body for creating a custom field definition.
type CreateCustomFieldRequest struct {
	Name        string          `json:"name"`
	Type        CustomFieldType `json:"type"`
	IsMandatory bool            `json:"is_mandatory"`
	Options     json.RawMessage `json:"options,omitempty"`
}

// UpdateRunResultRequest is the request body for updating a single run result.
// Pointer fields distinguish "not sent" (nil) from "explicitly set to zero/empty" (non-nil).
type UpdateRunResultRequest struct {
	Status       *string `json:"status,omitempty"`
	DefectType   *string `json:"defect_type,omitempty"`
	ErrorMessage *string `json:"error_message,omitempty"`
	StackTrace   *string `json:"stack_trace,omitempty"`
	FailureType  *string `json:"failure_type,omitempty"`
	DurationMs   *int64  `json:"duration_ms,omitempty"`
	Screenshots  *string `json:"screenshots,omitempty"`
	Video        *string `json:"video,omitempty"`
	TraceURL     *string `json:"trace_url,omitempty"`
	LogText      *string `json:"log_text,omitempty"`
	Browser      *string `json:"browser,omitempty"`
	OS           *string `json:"os,omitempty"`
	Environment  *string `json:"environment,omitempty"`
	AppVersion   *string `json:"app_version,omitempty"`
}

// TestCaseExecution is the flat row returned by "latest executions for a test case".
type TestCaseExecution struct {
	ID            string    `json:"id"`
	Status        string    `json:"status"`
	DefectType    string    `json:"defect_type"`
	DurationMs    int64     `json:"duration_ms"`
	ErrorMessage  string    `json:"error_message"`
	Environment   string    `json:"environment"`
	Browser       string    `json:"browser"`
	CreatedAt     time.Time `json:"created_at"`
	AttemptNumber int       `json:"attempt_number"`
	RunID         string    `json:"run_id"`
	RunName       string    `json:"run_name"`
	RunStatus     string    `json:"run_status"`
}
