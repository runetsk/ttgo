package store

// defects.go — DefectLink CRUD, Jira HTTP helpers, status refresh, and write-back.
// All Jira API calls use a 10-second timeout and HTTP Basic Auth (FR-012, Assumption).
// (008-jira-integration)

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"ttgo/pkg/tracker/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ────────────────────────────────────────────────────────────────────────────
// CRUD
// ────────────────────────────────────────────────────────────────────────────

// CreateDefectLink persists a new DefectLink and writes an audit log entry.
// Returns ErrDuplicateDefectLink if the (testCaseID, jiraKey) pair already exists.
func (s *Store) CreateDefectLink(testCaseID, jiraKey string, issue models.JiraIssueSummary) (*models.DefectLink, error) {
	now := time.Now()
	link := &models.DefectLink{
		ID:                uuid.New().String(),
		TestCaseID:        testCaseID,
		JiraIssueKey:      jiraKey,
		LastKnownSummary:  issue.Summary,
		LastKnownStatus:   issue.StatusName,
		LastKnownPriority: issue.PriorityName,
		LastKnownAssignee: issue.AssigneeName,
		LastKnownURL:      issue.BrowseURL,
		StatusCategory:    issue.StatusCategory,
		LastSyncedAt:      &now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := s.db.Create(link).Error; err != nil {
		if isUniqueConstraintError(err) {
			return nil, models.ErrDuplicateDefectLink
		}
		return nil, err
	}
	_ = s.logDefectEvent(link.ID, fmt.Sprintf("defect_link:created:%s", link.ID))
	return link, nil
}

// GetDefectLink returns the DefectLink for (testCaseID, jiraKey), or nil if not found.
func (s *Store) GetDefectLink(testCaseID, jiraKey string) (*models.DefectLink, error) {
	var link models.DefectLink
	err := s.db.Where("test_case_id = ? AND jira_issue_key = ? AND run_result_id IS NULL", testCaseID, jiraKey).First(&link).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &link, err
}

// ListDefectLinksByTestCase returns all DefectLinks for a test case ordered by
// creation date descending (most recently linked first, per FR-005).
func (s *Store) ListDefectLinksByTestCase(testCaseID string) ([]models.DefectLink, error) {
	var links []models.DefectLink
	err := s.db.Where("test_case_id = ? AND run_result_id IS NULL", testCaseID).
		Order("created_at DESC").
		Find(&links).Error
	return links, err
}

// ── Run-result-scoped CRUD ──

// CreateDefectLinkForResult persists a defect link scoped to a specific run result.
func (s *Store) CreateDefectLinkForResult(runResultID, testCaseID, jiraKey string, issue models.JiraIssueSummary) (*models.DefectLink, error) {
	now := time.Now()
	link := &models.DefectLink{
		ID:                uuid.New().String(),
		TestCaseID:        testCaseID,
		RunResultID:       &runResultID,
		JiraIssueKey:      jiraKey,
		LastKnownSummary:  issue.Summary,
		LastKnownStatus:   issue.StatusName,
		LastKnownPriority: issue.PriorityName,
		LastKnownAssignee: issue.AssigneeName,
		LastKnownURL:      issue.BrowseURL,
		StatusCategory:    issue.StatusCategory,
		LastSyncedAt:      &now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := s.db.Create(link).Error; err != nil {
		if isUniqueConstraintError(err) {
			return nil, models.ErrDuplicateDefectLink
		}
		return nil, err
	}
	_ = s.logDefectEvent(link.ID, fmt.Sprintf("defect_link:created:%s", link.ID))
	return link, nil
}

// ListDefectLinksByRunResult returns all DefectLinks scoped to a specific run result.
func (s *Store) ListDefectLinksByRunResult(runResultID string) ([]models.DefectLink, error) {
	var links []models.DefectLink
	err := s.db.Where("run_result_id = ?", runResultID).
		Order("created_at DESC").
		Find(&links).Error
	return links, err
}

// DeleteDefectLinkByResult removes a defect link scoped to a run result.
func (s *Store) DeleteDefectLinkByResult(runResultID, jiraKey string) error {
	result := s.db.Where("run_result_id = ? AND jira_issue_key = ?", runResultID, jiraKey).
		Delete(&models.DefectLink{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("defect link not found")
	}
	_ = s.logDefectEvent("", fmt.Sprintf("defect_link:deleted:%s@result:%s", jiraKey, runResultID))
	return nil
}

// ListDefectLinksByRun returns all defect links across every result in a run,
// enriched with the test name and result status.
func (s *Store) ListDefectLinksByRun(runID string) ([]models.RunDefectLinkRow, error) {
	var rows []models.RunDefectLinkRow
	err := s.db.Raw(`
		SELECT dl.*, rr.test_name_snapshot, rr.test_case_id, rr.status AS result_status
		FROM defect_links dl
		JOIN run_results rr ON rr.id = dl.run_result_id
		WHERE rr.test_run_id = ?
		ORDER BY rr.test_name_snapshot ASC, dl.created_at DESC
	`, runID).Scan(&rows).Error
	return rows, err
}

// CountDefectLinksByRunResults returns open/closed defect link counts per run result ID.
func (s *Store) CountDefectLinksByRunResults(runResultIDs []string) (open map[string]int, closed map[string]int, err error) {
	type result struct {
		RunResultID    string
		StatusCategory string
		Count          int
	}
	var results []result
	err = s.db.Model(&models.DefectLink{}).
		Select("run_result_id, status_category, count(*) as count").
		Where("run_result_id IN ?", runResultIDs).
		Group("run_result_id, status_category").
		Find(&results).Error
	if err != nil {
		return nil, nil, err
	}
	open = make(map[string]int, len(runResultIDs))
	closed = make(map[string]int, len(runResultIDs))
	for _, r := range results {
		if r.StatusCategory == "done" {
			closed[r.RunResultID] = r.Count
		} else {
			open[r.RunResultID] += r.Count
		}
	}
	return open, closed, nil
}

// ────────────────────────────────────────────────────────────────────────────
// Jira Issue Creation + Linking (US1)
// ────────────────────────────────────────────────────────────────────────────

// CreateJiraIssueAndLink creates a new Jira issue and links it to the test case
// in a single operation. If Jira creation fails, no link is persisted.
func (s *Store) CreateJiraIssueAndLink(testCaseID string, req models.CreateJiraIssueRequest) (*models.DefectLink, error) {
	cfg, err := s.GetJiraConfig()
	if err != nil {
		return nil, err
	}
	if cfg == nil || !cfg.Enabled {
		return nil, fmt.Errorf("Jira integration is not configured or disabled")
	}

	projectKey := req.ProjectKey
	if projectKey == "" {
		projectKey = cfg.DefaultProjectKey
	} else if cfg.DefaultProjectKey != "" && projectKey != cfg.DefaultProjectKey {
		// Pin to the configured project so a write-scoped caller cannot create
		// issues in arbitrary projects the service account can reach (F-009).
		return nil, fmt.Errorf("project_key must match the configured default project %q", cfg.DefaultProjectKey)
	}
	issueType := req.IssueType
	if issueType == "" {
		issueType = cfg.DefaultIssueType
	}
	if issueType == "" {
		issueType = "Bug"
	}

	issue, err := s.createJiraIssue(cfg, projectKey, issueType, req.Summary, req.Description)
	if err != nil {
		return nil, err
	}

	return s.CreateDefectLink(testCaseID, issue.Key, issue)
}

// ────────────────────────────────────────────────────────────────────────────
// Status Refresh (US3)
// ────────────────────────────────────────────────────────────────────────────

// RefreshDefectStatuses fetches the current Jira status for every DefectLink
// belonging to testCaseID, updates cached fields, and evaluates ReverificationFlagged.
// Per FR-007: partial failures retain last-known data and set a per-link ⚠ marker.
func (s *Store) RefreshDefectStatuses(testCaseID string) ([]models.DefectLink, error) {
	cfg, err := s.GetJiraConfig()
	if err != nil {
		return nil, err
	}
	if cfg == nil || !cfg.Enabled {
		return nil, fmt.Errorf("Jira integration is not configured or disabled")
	}

	links, err := s.ListDefectLinksByTestCase(testCaseID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	allDone := len(links) > 0
	for i, link := range links {
		issue, jiraErr := s.getJiraIssueSummary(cfg, link.JiraIssueKey)
		if jiraErr != nil {
			// Per FR-007: retain last-known data; mark status to signal staleness.
			links[i].LastKnownStatus = "⚠ " + links[i].LastKnownStatus
			allDone = false
			continue
		}
		links[i].LastKnownSummary = issue.Summary
		links[i].LastKnownStatus = issue.StatusName
		links[i].LastKnownPriority = issue.PriorityName
		links[i].LastKnownAssignee = issue.AssigneeName
		links[i].LastKnownURL = issue.BrowseURL
		links[i].StatusCategory = issue.StatusCategory
		links[i].LastSyncedAt = &now
		links[i].UpdatedAt = now

		if issue.StatusCategory != "done" {
			allDone = false
		}
	}

	// Batch all DB updates in a single transaction
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		for i := range links {
			if err := tx.Model(&links[i]).Updates(map[string]interface{}{
				"last_known_summary":  links[i].LastKnownSummary,
				"last_known_status":   links[i].LastKnownStatus,
				"last_known_priority": links[i].LastKnownPriority,
				"last_known_assignee": links[i].LastKnownAssignee,
				"last_known_url":      links[i].LastKnownURL,
				"status_category":     links[i].StatusCategory,
				"last_synced_at":      links[i].LastSyncedAt,
				"updated_at":          links[i].UpdatedAt,
			}).Error; err != nil {
				return err
			}
		}
		return tx.Model(&models.TestCase{}).Where("id = ?", testCaseID).
			Update("reverification_flagged", allDone).Error
	}); err != nil {
		return nil, fmt.Errorf("failed to persist defect status updates: %w", err)
	}

	return links, nil
}

// DismissReverification clears the ReverificationFlagged flag on a test case (FR-008).
func (s *Store) DismissReverification(testCaseID string) error {
	if err := s.db.Model(&models.TestCase{}).Where("id = ?", testCaseID).
		Update("reverification_flagged", false).Error; err != nil {
		return err
	}
	_ = s.logDefectEvent(testCaseID, fmt.Sprintf("defect_link:reverification_dismissed:%s", testCaseID))
	return nil
}

// ────────────────────────────────────────────────────────────────────────────
// Write-Back (US4)
// ────────────────────────────────────────────────────────────────────────────

// MarkCommentPending sets CommentPending=true and stores the pending comment text (FR-009).
func (s *Store) MarkCommentPending(testCaseID, jiraKey, commentText string) error {
	return s.db.Model(&models.DefectLink{}).
		Where("test_case_id = ? AND jira_issue_key = ?", testCaseID, jiraKey).
		Updates(map[string]interface{}{
			"comment_pending":      true,
			"pending_comment_text": commentText,
			"updated_at":           time.Now(),
		}).Error
}

// ClearCommentPending sets CommentPending=false and clears the pending comment text.
func (s *Store) ClearCommentPending(testCaseID, jiraKey string) error {
	return s.db.Model(&models.DefectLink{}).
		Where("test_case_id = ? AND jira_issue_key = ?", testCaseID, jiraKey).
		Updates(map[string]interface{}{
			"comment_pending":      false,
			"pending_comment_text": "",
			"updated_at":           time.Now(),
		}).Error
}

// PostAndClearComment posts a Jira comment for a CommentPending link and clears the flag.
// Returns the updated DefectLink or an error if the post fails.
func (s *Store) PostAndClearComment(testCaseID, jiraKey string) (*models.DefectLink, error) {
	link, err := s.GetDefectLink(testCaseID, jiraKey)
	if err != nil {
		return nil, err
	}
	if link == nil {
		return nil, fmt.Errorf("defect link not found")
	}
	if !link.CommentPending || link.PendingCommentText == "" {
		return nil, fmt.Errorf("no pending comment for this link")
	}

	cfg, err := s.GetJiraConfig()
	if err != nil {
		return nil, err
	}
	if cfg == nil || !cfg.Enabled {
		return nil, fmt.Errorf("Jira integration is not configured or disabled")
	}

	if err := s.postJiraComment(cfg, jiraKey, link.PendingCommentText); err != nil {
		return nil, err
	}

	_ = s.logDefectEvent(jiraKey, fmt.Sprintf("defect_link:retry_success:%s", jiraKey))
	if err := s.ClearCommentPending(testCaseID, jiraKey); err != nil {
		return nil, err
	}

	return s.GetDefectLink(testCaseID, jiraKey)
}

// WriteBackComment posts a Jira comment for a passed test case result.
// On failure it calls MarkCommentPending and returns the warning text.
// The returned warning string is non-empty only on failure.
func (s *Store) WriteBackComment(cfg *models.JiraConfig, testCaseID, jiraKey, testName, runURL string) string {
	commentText := fmt.Sprintf(`✅ Test "%s" passed in TTGO. Run: %s`, testName, runURL)
	if err := s.postJiraComment(cfg, jiraKey, commentText); err != nil {
		_ = s.MarkCommentPending(testCaseID, jiraKey, commentText)
		_ = s.logDefectEvent(jiraKey, fmt.Sprintf("defect_link:comment_failed:%s", jiraKey))
		return err.Error()
	}
	_ = s.logDefectEvent(jiraKey, fmt.Sprintf("defect_link:jira_writeback:%s", jiraKey))
	return ""
}

// ────────────────────────────────────────────────────────────────────────────
// Jira HTTP helpers (private)
// ────────────────────────────────────────────────────────────────────────────

// jiraRequest performs an authenticated HTTP request against the Jira Cloud REST API.
// Uses 10-second timeout and HTTP Basic Auth (base64(email:apiToken)).
func (s *Store) jiraRequest(cfg *models.JiraConfig, method, urlPath string, body io.Reader) (*http.Response, error) {
	base := strings.TrimRight(cfg.BaseURL, "/")
	fullURL := base + urlPath

	client := s.httpClient
	req, err := http.NewRequest(method, fullURL, body)
	if err != nil {
		return nil, fmt.Errorf("Jira is unreachable — check your connection and try again")
	}

	creds := base64.StdEncoding.EncodeToString([]byte(cfg.Email + ":" + cfg.APIToken))
	req.Header.Set("Authorization", "Basic "+creds)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Jira is unreachable — check your connection and try again")
	}
	return resp, nil
}

// GetJiraIssueSummary is the public wrapper used by API handlers to validate a key before linking.
func (s *Store) GetJiraIssueSummary(cfg *models.JiraConfig, key string) (models.JiraIssueSummary, error) {
	return s.getJiraIssueSummary(cfg, key)
}

// getJiraIssueSummary fetches summary, status, priority, and assignee for a Jira issue.
func (s *Store) getJiraIssueSummary(cfg *models.JiraConfig, key string) (models.JiraIssueSummary, error) {
	resp, err := s.jiraRequest(cfg, http.MethodGet,
		fmt.Sprintf("/rest/api/3/issue/%s?fields=summary,status,priority,assignee", key), nil)
	if err != nil {
		return models.JiraIssueSummary{}, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return models.JiraIssueSummary{}, fmt.Errorf("Jira credentials are invalid — check your API token in Settings")
	case http.StatusForbidden:
		return models.JiraIssueSummary{}, fmt.Errorf("You do not have permission to perform this action in Jira — contact your Jira administrator")
	case http.StatusNotFound:
		return models.JiraIssueSummary{}, fmt.Errorf("Jira issue %s could not be found — it may have been deleted or moved", key)
	case http.StatusTooManyRequests:
		return models.JiraIssueSummary{}, fmt.Errorf("Jira rate limit reached — wait a moment and try again")
	}
	if resp.StatusCode >= 500 {
		return models.JiraIssueSummary{}, fmt.Errorf("Jira is temporarily unavailable — try again shortly")
	}
	if resp.StatusCode != http.StatusOK {
		return models.JiraIssueSummary{}, fmt.Errorf("Jira returned HTTP %d", resp.StatusCode)
	}

	var issue struct {
		Fields struct {
			Summary string `json:"summary"`
			Status  struct {
				Name           string `json:"name"`
				StatusCategory struct {
					Key string `json:"key"`
				} `json:"statusCategory"`
			} `json:"status"`
			Priority struct {
				Name string `json:"name"`
			} `json:"priority"`
			Assignee *struct {
				DisplayName string `json:"displayName"`
			} `json:"assignee"`
		} `json:"fields"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return models.JiraIssueSummary{}, fmt.Errorf("failed to parse Jira response: %v", err)
	}

	assignee := ""
	if issue.Fields.Assignee != nil {
		assignee = issue.Fields.Assignee.DisplayName
	}

	return models.JiraIssueSummary{
		Key:            key,
		Summary:        issue.Fields.Summary,
		StatusName:     issue.Fields.Status.Name,
		StatusCategory: issue.Fields.Status.StatusCategory.Key,
		PriorityName:   issue.Fields.Priority.Name,
		AssigneeName:   assignee,
		BrowseURL:      strings.TrimRight(cfg.BaseURL, "/") + "/browse/" + key,
	}, nil
}

// createJiraIssue creates a new Jira issue using REST API v3 (ADF body format).
func (s *Store) createJiraIssue(cfg *models.JiraConfig, projectKey, issueType, summary, description string) (models.JiraIssueSummary, error) {
	if projectKey == "" {
		return models.JiraIssueSummary{}, fmt.Errorf("project_key is required to create a Jira issue")
	}

	descText := description
	if descText == "" {
		descText = "Actual result: (not recorded)"
	}

	payload := map[string]interface{}{
		"fields": map[string]interface{}{
			"project":   map[string]string{"key": projectKey},
			"summary":   summary,
			"issuetype": map[string]string{"name": issueType},
			"description": map[string]interface{}{
				"type":    "doc",
				"version": 1,
				"content": []interface{}{
					map[string]interface{}{
						"type": "paragraph",
						"content": []interface{}{
							map[string]interface{}{"type": "text", "text": descText},
						},
					},
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return models.JiraIssueSummary{}, fmt.Errorf("failed to build Jira request: %v", err)
	}

	resp, err := s.jiraRequest(cfg, http.MethodPost, "/rest/api/3/issue", bytes.NewReader(body))
	if err != nil {
		return models.JiraIssueSummary{}, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return models.JiraIssueSummary{}, fmt.Errorf("Jira credentials are invalid — check your API token in Settings")
	case http.StatusForbidden:
		return models.JiraIssueSummary{}, fmt.Errorf("You do not have permission to perform this action in Jira — contact your Jira administrator")
	case http.StatusTooManyRequests:
		return models.JiraIssueSummary{}, fmt.Errorf("Jira rate limit reached — wait a moment and try again")
	}
	if resp.StatusCode >= 500 {
		return models.JiraIssueSummary{}, fmt.Errorf("Jira is temporarily unavailable — try again shortly")
	}
	if resp.StatusCode != http.StatusCreated {
		// Do not echo the raw upstream body back to the caller — it can disclose
		// internal Jira detail. Drain it and return a generic message (F-009).
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		return models.JiraIssueSummary{}, fmt.Errorf("Jira rejected the issue (HTTP %d) — verify the project key and required fields", resp.StatusCode)
	}

	var created struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return models.JiraIssueSummary{}, fmt.Errorf("failed to parse Jira create response: %v", err)
	}

	// Fetch the full issue summary to populate cached fields.
	return s.getJiraIssueSummary(cfg, created.Key)
}

// postJiraComment posts an ADF-wrapped comment to a Jira issue.
func (s *Store) postJiraComment(cfg *models.JiraConfig, key, commentText string) error {
	payload := map[string]interface{}{
		"body": map[string]interface{}{
			"type":    "doc",
			"version": 1,
			"content": []interface{}{
				map[string]interface{}{
					"type": "paragraph",
					"content": []interface{}{
						map[string]interface{}{"type": "text", "text": commentText},
					},
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to build Jira comment request: %v", err)
	}

	resp, err := s.jiraRequest(cfg, http.MethodPost,
		fmt.Sprintf("/rest/api/3/issue/%s/comment", key), bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("Jira credentials are invalid — check your API token in Settings")
	case http.StatusForbidden:
		return fmt.Errorf("You do not have permission to perform this action in Jira — contact your Jira administrator")
	case http.StatusNotFound:
		return fmt.Errorf("Jira issue %s could not be found — it may have been deleted or moved", key)
	case http.StatusTooManyRequests:
		return fmt.Errorf("Jira rate limit reached — wait a moment and try again")
	}
	if resp.StatusCode >= 500 {
		return fmt.Errorf("Jira is temporarily unavailable — try again shortly")
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Jira returned HTTP %d when posting comment", resp.StatusCode)
	}
	return nil
}

// ────────────────────────────────────────────────────────────────────────────
// Internal helpers
// ────────────────────────────────────────────────────────────────────────────

func (s *Store) logDefectEvent(ref, action string) error {
	return s.CreateAuditLog(&models.AuditLog{
		ID:        uuid.New().String(),
		Action:    action,
		Timestamp: time.Now(),
	})
}
