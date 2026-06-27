package store

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"ttgo/pkg/tracker/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ────────────────────────────────────────────────────────────────────────────
// Requirements CRUD
// ────────────────────────────────────────────────────────────────────────────

// CreateRequirement persists a new requirement.
// Returns ErrDuplicateRequirementIdentifier if the identifier is already taken.
func (s *Store) CreateRequirement(r *models.Requirement) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	if err := s.db.Create(r).Error; err != nil {
		if isUniqueConstraintError(err) {
			return models.ErrDuplicateRequirementIdentifier
		}
		return err
	}
	_ = s.store_logRequirementEvent(r.ID, "created") //nolint:errcheck
	return nil
}

// GetRequirement fetches a single requirement by ID.
func (s *Store) GetRequirement(id string) (*models.Requirement, error) {
	var r models.Requirement
	if err := s.db.First(&r, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("requirement not found")
		}
		return nil, err
	}
	return &r, nil
}

// ListRequirements returns top-level requirements (no parent) ordered by identifier.
func (s *Store) ListRequirements() ([]*models.Requirement, error) {
	var reqs []*models.Requirement
	if err := s.db.Where("parent_id IS NULL").Order("identifier").Find(&reqs).Error; err != nil {
		return nil, err
	}
	return reqs, nil
}

// ListChildRequirements returns all child requirements for a given parent, ordered by identifier.
func (s *Store) ListChildRequirements(parentID string) ([]*models.Requirement, error) {
	var reqs []*models.Requirement
	if err := s.db.Where("parent_id = ?", parentID).Order("identifier").Find(&reqs).Error; err != nil {
		return nil, err
	}
	return reqs, nil
}

// GetRequirementChildCounts returns a map of requirement ID → child count for the given IDs.
func (s *Store) GetRequirementChildCounts(ids []string) (map[string]int, error) {
	if len(ids) == 0 {
		return map[string]int{}, nil
	}
	type result struct {
		ParentID string
		Count    int
	}
	var results []result
	if err := s.db.Model(&models.Requirement{}).
		Select("parent_id, count(*) as count").
		Where("parent_id IN ?", ids).
		Group("parent_id").
		Find(&results).Error; err != nil {
		return nil, err
	}
	counts := make(map[string]int, len(results))
	for _, r := range results {
		counts[r.ParentID] = r.Count
	}
	return counts, nil
}

// UpdateRequirement updates the identifier, title, and description of an
// existing requirement.
// Returns ErrDuplicateRequirementIdentifier if the new identifier conflicts.
func (s *Store) UpdateRequirement(r *models.Requirement) error {
	if err := s.db.Model(r).Updates(map[string]interface{}{
		"identifier":  r.Identifier,
		"title":       r.Title,
		"description": r.Description,
		"updated_at":  time.Now(),
	}).Error; err != nil {
		if isUniqueConstraintError(err) {
			return models.ErrDuplicateRequirementIdentifier
		}
		return err
	}
	_ = s.store_logRequirementEvent(r.ID, "updated") //nolint:errcheck
	return nil
}

// DeleteRequirement removes a requirement by ID, cascading to descendants and links.
func (s *Store) DeleteRequirement(id string) error {
	return s.DeleteRequirements([]string{id})
}

// DeleteRequirements removes requirements by IDs and the full descendant closure,
// atomically. Previously this ran several un-transactioned deletes (errors
// ignored) and only one level deep, leaving grandchildren and links orphaned on
// partial failure (F-036).
func (s *Store) DeleteRequirements(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// BFS the descendant closure so no grandchild is left orphaned.
		seen := make(map[string]bool, len(ids))
		frontier := make([]string, 0, len(ids))
		for _, id := range ids {
			if !seen[id] {
				seen[id] = true
				frontier = append(frontier, id)
			}
		}
		for len(frontier) > 0 {
			var kids []string
			if err := tx.Model(&models.Requirement{}).Where("parent_id IN ?", frontier).Pluck("id", &kids).Error; err != nil {
				return err
			}
			frontier = frontier[:0]
			for _, k := range kids {
				if !seen[k] {
					seen[k] = true
					frontier = append(frontier, k)
				}
			}
		}
		allIDs := make([]string, 0, len(seen))
		for id := range seen {
			allIDs = append(allIDs, id)
		}
		if err := tx.Delete(&models.RequirementTestCaseLink{}, "requirement_id IN ?", allIDs).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Requirement{}, "id IN ?", allIDs).Error
	})
	if err != nil {
		return err
	}
	for _, id := range ids {
		_ = s.store_logRequirementEvent(id, "deleted") //nolint:errcheck
	}
	return nil
}

// SetRequirementParent sets the parent_id for a child requirement.
func (s *Store) SetRequirementParent(childID, parentID string) error {
	if parentID == "" {
		return s.db.Model(&models.Requirement{}).Where("id = ?", childID).Update("parent_id", nil).Error
	}
	if childID == parentID {
		return fmt.Errorf("a requirement cannot be its own parent")
	}
	// Walk up from parentID; reaching childID means this assignment would create a
	// cycle, which would hide nodes from the matrix and loop tree traversals (F-055).
	cur := parentID
	for i := 0; i < 1000 && cur != ""; i++ {
		if cur == childID {
			return fmt.Errorf("setting this parent would create a cycle")
		}
		var p models.Requirement
		if err := s.db.Select("parent_id").First(&p, "id = ?", cur).Error; err != nil || p.ParentID == nil {
			break
		}
		cur = *p.ParentID
	}
	return s.db.Model(&models.Requirement{}).Where("id = ?", childID).Update("parent_id", parentID).Error
}

// SetRequirementJiraPostHash records the content hash of the last post-to-jira
// for idempotency (F-056).
func (s *Store) SetRequirementJiraPostHash(id, hash string) error {
	return s.db.Model(&models.Requirement{}).Where("id = ?", id).Update("last_jira_post_hash", hash).Error
}

// ────────────────────────────────────────────────────────────────────────────
// Traceability Links
// ────────────────────────────────────────────────────────────────────────────

// CreateLink links a test case to a requirement.
// Returns ErrDuplicateLink if the pair already exists.
// Returns an error if either the requirement or the test case does not exist.
func (s *Store) CreateLink(requirementID, testCaseID string) (*models.RequirementTestCaseLink, error) {
	// Verify requirement exists.
	var reqCount int64
	s.db.Model(&models.Requirement{}).Where("id = ?", requirementID).Count(&reqCount)
	if reqCount == 0 {
		return nil, fmt.Errorf("requirement not found")
	}
	// Verify test case exists.
	var tcCount int64
	s.db.Model(&models.TestCase{}).Where("id = ?", testCaseID).Count(&tcCount)
	if tcCount == 0 {
		return nil, fmt.Errorf("test case not found")
	}

	// Duplicate check.
	var existing int64
	s.db.Model(&models.RequirementTestCaseLink{}).
		Where("requirement_id = ? AND test_case_id = ?", requirementID, testCaseID).
		Count(&existing)
	if existing > 0 {
		return nil, models.ErrDuplicateLink
	}

	link := &models.RequirementTestCaseLink{
		ID:            uuid.New().String(),
		RequirementID: requirementID,
		TestCaseID:    testCaseID,
		CreatedAt:     time.Now(),
	}
	if err := s.db.Create(link).Error; err != nil {
		return nil, err
	}
	_ = s.store_logLinkEvent(link.ID, "created") //nolint:errcheck
	return link, nil
}

// DeleteLink removes the traceability link between a requirement and a test
// case. Returns an error if no such link exists.
func (s *Store) DeleteLink(requirementID, testCaseID string) error {
	result := s.db.Where("requirement_id = ? AND test_case_id = ?", requirementID, testCaseID).
		Delete(&models.RequirementTestCaseLink{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("link not found")
	}
	_ = s.store_logLinkEvent(requirementID+":"+testCaseID, "deleted") //nolint:errcheck
	return nil
}

// ListRequirementsByTestCase returns all requirements linked to a given test case.
func (s *Store) ListRequirementsByTestCase(testCaseID string) ([]*models.Requirement, error) {
	var reqs []*models.Requirement
	err := s.db.
		Joins("JOIN requirement_test_case_links ON requirement_test_case_links.requirement_id = requirements.id").
		Where("requirement_test_case_links.test_case_id = ?", testCaseID).
		Order("requirements.identifier").
		Find(&reqs).Error
	if err != nil {
		return nil, err
	}
	return reqs, nil
}

// ListTestCasesByRequirement returns all test cases linked to a given requirement.
func (s *Store) ListTestCasesByRequirement(requirementID string) ([]*models.TestCase, error) {
	var tcs []*models.TestCase
	err := s.db.
		Joins("JOIN requirement_test_case_links ON requirement_test_case_links.test_case_id = test_cases.id").
		Where("requirement_test_case_links.requirement_id = ?", requirementID).
		Order("test_cases.name").
		Find(&tcs).Error
	if err != nil {
		return nil, err
	}
	return tcs, nil
}

// ────────────────────────────────────────────────────────────────────────────
// Traceability Matrix
// ────────────────────────────────────────────────────────────────────────────

type matrixQueryRow struct {
	RequirementID string
	Identifier    string
	Title         string
	Description   string
	TestCaseID    *string
	TestCaseName  *string
}

// GetTraceabilityMatrix returns the full traceability matrix: every requirement
// with its linked test cases and a computed CoverageSummary.
func (s *Store) GetTraceabilityMatrix() (*models.MatrixResponse, error) {
	rows, err := s.db.Raw(`
		SELECT
			r.id          AS requirement_id,
			r.identifier  AS identifier,
			r.title       AS title,
			r.description AS description,
			rtl.test_case_id AS test_case_id,
			tc.name          AS test_case_name
		FROM requirements r
		LEFT JOIN requirement_test_case_links rtl ON rtl.requirement_id = r.id
		LEFT JOIN test_cases tc ON tc.id = rtl.test_case_id
		WHERE r.parent_id IS NULL
		ORDER BY r.identifier, rtl.created_at
	`).Rows()
	if err != nil {
		return nil, fmt.Errorf("matrix query failed: %w", err)
	}
	defer rows.Close()

	// Group results by requirement_id.
	orderedIDs := []string{}
	rowMap := map[string]*models.MatrixRow{}

	for rows.Next() {
		var q matrixQueryRow
		if err := s.db.ScanRows(rows, &q); err != nil {
			return nil, err
		}
		mr, exists := rowMap[q.RequirementID]
		if !exists {
			mr = &models.MatrixRow{
				RequirementID:   q.RequirementID,
				Identifier:      q.Identifier,
				Title:           q.Title,
				Description:     q.Description,
				LinkedTestCases: []models.LinkedTC{},
			}
			rowMap[q.RequirementID] = mr
			orderedIDs = append(orderedIDs, q.RequirementID)
		}
		if q.TestCaseID != nil && *q.TestCaseID != "" {
			mr.LinkedTestCases = append(mr.LinkedTestCases, models.LinkedTC{
				TestCaseID:   *q.TestCaseID,
				TestCaseName: derefStr(q.TestCaseName),
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Build ordered slice and compute summary.
	matrixRows := make([]*models.MatrixRow, 0, len(orderedIDs))
	covered := 0
	for _, id := range orderedIDs {
		mr := rowMap[id]
		mr.Covered = len(mr.LinkedTestCases) > 0
		if mr.Covered {
			covered++
		}
		matrixRows = append(matrixRows, mr)
	}

	total := len(matrixRows)
	uncovered := total - covered
	pct := 0.0
	if total > 0 {
		pct = math.Round(float64(covered)/float64(total)*1000) / 10
	}

	return &models.MatrixResponse{
		Rows: matrixRows,
		Summary: models.CoverageSummary{
			Total:      total,
			Covered:    covered,
			Uncovered:  uncovered,
			Percentage: pct,
		},
	}, nil
}

// ────────────────────────────────────────────────────────────────────────────
// Jira Config
// ────────────────────────────────────────────────────────────────────────────

const jiraConfigSingletonID = "singleton"

// GetJiraConfig returns the workspace Jira configuration.
// Returns nil (no error) if no configuration has been saved yet.
func (s *Store) GetJiraConfig() (*models.JiraConfig, error) {
	var cfg models.JiraConfig
	if err := s.db.First(&cfg, "id = ?", jiraConfigSingletonID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	cfg.APIToken = s.decryptSecret(cfg.APIToken) // at-rest decryption (F-016)
	return &cfg, nil
}

// UpsertJiraConfig creates or updates the singleton Jira configuration.
// If newToken is empty, the existing stored token is preserved.
// defaultProjectKey and defaultIssueType configure defect-creation defaults (008-jira-integration).
func (s *Store) UpsertJiraConfig(baseURL, email, newToken string, enabled bool, defaultProjectKey, defaultIssueType string) (*models.JiraConfig, error) {
	now := time.Now()

	existing, err := s.GetJiraConfig()
	if err != nil {
		return nil, err
	}

	token := newToken
	if token == "" && existing != nil {
		token = existing.APIToken // preserve existing token when not rotating
	}
	token = s.encryptSecret(token) // encrypt at rest before storage (F-016)

	cfg := models.JiraConfig{
		ID:                jiraConfigSingletonID,
		BaseURL:           baseURL,
		Email:             email,
		APIToken:          token,
		Enabled:           enabled,
		DefaultProjectKey: defaultProjectKey,
		DefaultIssueType:  defaultIssueType,
		UpdatedAt:         now,
	}
	if existing == nil {
		cfg.CreatedAt = now
		if err := s.db.Create(&cfg).Error; err != nil {
			return nil, err
		}
	} else {
		if err := s.db.Model(&cfg).Updates(map[string]interface{}{
			"base_url":            baseURL,
			"email":               email,
			"api_token":           token,
			"enabled":             enabled,
			"default_project_key": defaultProjectKey,
			"default_issue_type":  defaultIssueType,
			"updated_at":          now,
		}).Error; err != nil {
			return nil, err
		}
		cfg.CreatedAt = existing.CreatedAt
	}
	return s.GetJiraConfig() // re-read so the returned token is decrypted for masking
}

// ────────────────────────────────────────────────────────────────────────────
// Import-related methods (011-jira-confluence-import)
// ────────────────────────────────────────────────────────────────────────────

// FindRequirementBySource returns an existing requirement imported from the
// given source, or nil if none exists. Used for duplicate detection.
func (s *Store) FindRequirementBySource(sourceType, sourceKey string) (*models.Requirement, error) {
	if sourceType == "" || sourceKey == "" {
		return nil, nil
	}
	var req models.Requirement
	if err := s.db.Where("source_type = ? AND source_key = ?", sourceType, sourceKey).First(&req).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &req, nil
}

// CreateImportedRequirement creates a requirement with source metadata and
// sets imported_at to the current time.
func (s *Store) CreateImportedRequirement(r *models.Requirement) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	now := time.Now()
	r.ImportedAt = &now
	r.CreatedAt = now
	r.UpdatedAt = now
	if err := s.db.Create(r).Error; err != nil {
		if isUniqueConstraintError(err) {
			return models.ErrDuplicateRequirementIdentifier
		}
		return err
	}
	action := fmt.Sprintf("requirement:imported_%s:%s", r.SourceType, r.ID)
	_ = s.CreateAuditLog(&models.AuditLog{
		ID:        uuid.New().String(),
		Action:    action,
		Timestamp: now,
	})
	return nil
}

// GetRequirementForResync fetches a requirement by ID for resync.
// Returns error if not found or has no source.
func (s *Store) GetRequirementForResync(id string) (*models.Requirement, error) {
	req, err := s.GetRequirement(id)
	if err != nil {
		return nil, err
	}
	if req.SourceType == "" || req.SourceKey == "" {
		return nil, fmt.Errorf("this requirement has no import source to sync from")
	}
	return req, nil
}

// ApplyResyncUpdate updates title, description, and sets imported_at to now.
func (s *Store) ApplyResyncUpdate(id, title, description string) (*models.Requirement, error) {
	now := time.Now()
	if err := s.db.Model(&models.Requirement{}).Where("id = ?", id).Updates(map[string]interface{}{
		"title":       title,
		"description": description,
		"imported_at": now,
		"updated_at":  now,
	}).Error; err != nil {
		return nil, err
	}
	return s.GetRequirement(id)
}

// MarkSynced updates imported_at to now (used for keep_local acknowledgment).
func (s *Store) MarkSynced(id string) error {
	now := time.Now()
	return s.db.Model(&models.Requirement{}).Where("id = ?", id).Updates(map[string]interface{}{
		"imported_at": now,
	}).Error
}

// UnlinkRequirement clears source metadata, converting to standalone.
func (s *Store) UnlinkRequirement(id string) (*models.Requirement, error) {
	if err := s.db.Model(&models.Requirement{}).Where("id = ?", id).Updates(map[string]interface{}{
		"source_type": "",
		"source_key":  "",
		"source_url":  "",
		"imported_at": nil,
		"updated_at":  time.Now(),
	}).Error; err != nil {
		return nil, err
	}
	return s.GetRequirement(id)
}

// ────────────────────────────────────────────────────────────────────────────
// Confluence Config (011-jira-confluence-import)
// ────────────────────────────────────────────────────────────────────────────

const confluenceConfigSingletonID = "singleton"

// GetConfluenceConfig returns the workspace Confluence configuration.
// Returns nil (no error) if no configuration has been saved yet.
func (s *Store) GetConfluenceConfig() (*models.ConfluenceConfig, error) {
	var cfg models.ConfluenceConfig
	if err := s.db.First(&cfg, "id = ?", confluenceConfigSingletonID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	cfg.APIToken = s.decryptSecret(cfg.APIToken) // at-rest decryption (F-016)
	return &cfg, nil
}

// UpsertConfluenceConfig creates or updates the singleton Confluence configuration.
// If newToken is empty, the existing stored token is preserved.
func (s *Store) UpsertConfluenceConfig(baseURL, email, newToken string, enabled bool) (*models.ConfluenceConfig, error) {
	now := time.Now()

	existing, err := s.GetConfluenceConfig()
	if err != nil {
		return nil, err
	}

	token := newToken
	if token == "" && existing != nil {
		token = existing.APIToken
	}
	token = s.encryptSecret(token) // encrypt at rest before storage (F-016)

	cfg := models.ConfluenceConfig{
		ID:        confluenceConfigSingletonID,
		BaseURL:   baseURL,
		Email:     email,
		APIToken:  token,
		Enabled:   enabled,
		UpdatedAt: now,
	}
	if existing == nil {
		cfg.CreatedAt = now
		if err := s.db.Create(&cfg).Error; err != nil {
			return nil, err
		}
	} else {
		if err := s.db.Model(&cfg).Updates(map[string]interface{}{
			"base_url":   baseURL,
			"email":      email,
			"api_token":  token,
			"enabled":    enabled,
			"updated_at": now,
		}).Error; err != nil {
			return nil, err
		}
		cfg.CreatedAt = existing.CreatedAt
	}
	return s.GetConfluenceConfig() // re-read so the returned token is decrypted for masking
}

// ────────────────────────────────────────────────────────────────────────────
// Internal helpers
// ────────────────────────────────────────────────────────────────────────────

func (s *Store) store_logRequirementEvent(requirementID, action string) error {
	return s.CreateAuditLog(&models.AuditLog{
		ID:         uuid.New().String(),
		TestCaseID: "",
		Action:     fmt.Sprintf("requirement:%s:%s", action, requirementID),
		Timestamp:  time.Now(),
	})
}

func (s *Store) store_logLinkEvent(ref, action string) error {
	return s.CreateAuditLog(&models.AuditLog{
		ID:         uuid.New().String(),
		TestCaseID: "",
		Action:     fmt.Sprintf("traceability_link:%s:%s", action, ref),
		Timestamp:  time.Now(),
	})
}

// isUniqueConstraintError detects SQLite UNIQUE constraint violations.
func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint") || strings.Contains(msg, "unique_idx") || strings.Contains(msg, "UNIQUE constraint failed")
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
