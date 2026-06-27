package models

import (
	"errors"
	"time"
)

// ErrDuplicateDefectLink is returned when the same Jira key is linked to the
// same test case more than once (FR-013).
var ErrDuplicateDefectLink = errors.New("this Jira issue is already linked to the test case")

// DefectLink persists the many-to-many relationship between a TTGO test case
// and a Jira Cloud issue, including cached issue metadata and write-back state.
// Table: defect_links (008-jira-integration)
type DefectLink struct {
	ID                 string     `json:"id"                   gorm:"primaryKey"`
	TestCaseID         string     `json:"test_case_id"         gorm:"index;not null"`
	RunResultID        *string    `json:"run_result_id,omitempty" gorm:"index"` // nullable — when set, link is scoped to a specific run result
	JiraIssueKey       string     `json:"jira_issue_key"       gorm:"not null"` // e.g. "PROJ-123"
	LastKnownSummary   string     `json:"last_known_summary"`
	LastKnownStatus    string     `json:"last_known_status"`   // e.g. "In Progress"
	LastKnownPriority  string     `json:"last_known_priority"` // e.g. "High"
	LastKnownAssignee  string     `json:"last_known_assignee"`
	LastKnownURL       string     `json:"last_known_url"`                            // Jira issue browse URL
	StatusCategory     string     `json:"status_category"`                           // "todo"|"indeterminate"|"done"
	CommentPending     bool       `json:"comment_pending"      gorm:"default:false"` // true = write-back failed
	PendingCommentText string     `json:"pending_comment_text,omitempty"`            // stored for retry
	LastSyncedAt       *time.Time `json:"last_synced_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// JiraIssueSummary carries Jira Cloud issue data between the store Jira HTTP
// helpers and the CRUD methods. Not persisted.
type JiraIssueSummary struct {
	Key            string // "PROJ-123"
	Summary        string
	StatusName     string // e.g. "In Progress"
	StatusCategory string // "todo" | "indeterminate" | "done"
	PriorityName   string
	AssigneeName   string
	BrowseURL      string // base_url + "/browse/" + key
}

// LinkDefectRequest is the payload for POST /api/tests/{id}/defect-links.
type LinkDefectRequest struct {
	JiraIssueKey string `json:"jira_issue_key"` // required, e.g. "PROJ-123"
}

// CreateJiraIssueRequest is the payload for POST /api/tests/{id}/defect-links/create-issue.
type CreateJiraIssueRequest struct {
	Summary     string `json:"summary"`     // required
	Description string `json:"description"` // optional, plain text
	ProjectKey  string `json:"project_key"` // optional, falls back to JiraConfig default
	IssueType   string `json:"issue_type"`  // optional, falls back to JiraConfig default ("Bug")
}

// DefectLinkListResponse is the envelope returned by GET /api/tests/{id}/defect-links
// and POST /api/tests/{id}/defect-links/refresh.
type DefectLinkListResponse struct {
	Links                 []DefectLink `json:"links"`
	ReverificationFlagged bool         `json:"reverification_flagged"`
}

// RunDefectLinkRow is the flat result returned by the "list all defect links in a run" query.
// It embeds DefectLink fields and adds context from the run result row.
type RunDefectLinkRow struct {
	DefectLink
	TestNameSnapshot string `json:"test_name_snapshot"`
	TestCaseID       string `json:"test_case_id"`
	ResultStatus     string `json:"result_status"`
}

// RunResultUpdateWarning is appended to the run-result update response body when
// a Jira write-back partially fails (FR-009).
type RunResultUpdateWarning struct {
	JiraIssueKey string `json:"jira_issue_key"`
	Message      string `json:"message"`
}
