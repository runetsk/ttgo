package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"ttgo/internal/safehttp"
	"ttgo/pkg/tracker/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const qtestConfigSingletonID = "singleton-qtest"
const qtestPageSize = 100
const qtestMaxResponseBodyBytes = 10 << 20

// ────────────────────────────────────────────────────────────────────────────
// T005: QTest HTTP helper
// ────────────────────────────────────────────────────────────────────────────

// qtestRequest is the base HTTP helper for QTest API calls.
// Uses Bearer token auth, 10s timeout, user-friendly error messages.
func (s *Store) qtestRequest(cfg *models.QTestConfig, method, urlPath string, body io.Reader) (*http.Response, error) {
	base := strings.TrimRight(cfg.BaseURL, "/")
	// Normalize: strip trailing /api/v3 or /api/v if user included it in base URL
	base = strings.TrimRight(base, "/")
	for _, suffix := range []string{"/api/v3", "/api/v", "/api"} {
		if strings.HasSuffix(base, suffix) {
			base = strings.TrimSuffix(base, suffix)
			break
		}
	}
	fullURL := base + urlPath

	client := safehttp.IntegrationClient(10 * time.Second) // SSRF guard, allows self-hosted private hosts (F-003)
	req, err := http.NewRequest(method, fullURL, body)
	if err != nil {
		return nil, fmt.Errorf("QTest is unreachable — check your connection and try again")
	}

	req.Header.Set("Authorization", "bearer "+cfg.APIToken)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	log.Printf("[DEBUG] QTest %s %s", method, fullURL)
	start := time.Now()
	resp, err := client.Do(req)
	dur := time.Since(start)
	if err != nil {
		log.Printf("[WARN] QTest %s %s failed after %s: %v", method, fullURL, dur, err)
		return nil, fmt.Errorf("QTest is unreachable — check your connection and try again")
	}
	log.Printf("[INFO] QTest %s %s → %d (%s)", method, fullURL, resp.StatusCode, dur)

	return resp, nil
}

// qtestCheckStatus inspects the HTTP status code and returns a user-friendly error.
// Returns nil if the status is 2xx.
func qtestCheckStatus(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("QTest credentials are invalid — check your API token in Settings")
	case http.StatusForbidden:
		return fmt.Errorf("You do not have permission to access this resource in QTest")
	case http.StatusNotFound:
		return fmt.Errorf("QTest resource could not be found")
	case http.StatusTooManyRequests:
		return fmt.Errorf("QTest rate limit reached — wait a moment and try again")
	}
	if resp.StatusCode >= 500 {
		return fmt.Errorf("QTest is temporarily unavailable — try again shortly")
	}
	return fmt.Errorf("QTest returned HTTP %d", resp.StatusCode)
}

func decodeQTestResponse(body io.Reader, target any) error {
	return json.NewDecoder(io.LimitReader(body, qtestMaxResponseBodyBytes)).Decode(target)
}

// isQTestRateLimited returns true if the error is a rate-limit error.
func isQTestRateLimited(err error) bool {
	return err != nil && strings.Contains(err.Error(), "rate limit")
}

// isQTestNotFound returns true if the error is a 404 not-found error.
func isQTestNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "could not be found")
}

// ────────────────────────────────────────────────────────────────────────────
// T006: Config CRUD
// ────────────────────────────────────────────────────────────────────────────

// GetQTestConfig returns the singleton QTest configuration, or nil if not yet configured.
func (s *Store) GetQTestConfig() (*models.QTestConfig, error) {
	var cfg models.QTestConfig
	if err := s.db.First(&cfg, "id = ?", qtestConfigSingletonID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	cfg.APIToken = s.decryptSecret(cfg.APIToken) // at-rest decryption (F-016)
	return &cfg, nil
}

// UpsertQTestConfig creates or updates the singleton QTest configuration.
// If apiToken is empty, the existing token is preserved.
func (s *Store) UpsertQTestConfig(baseURL, email, apiToken string, projectID int64, projectName string, enabled bool) (*models.QTestConfig, error) {
	now := time.Now()
	existing, err := s.GetQTestConfig()
	if err != nil {
		return nil, err
	}

	finalToken := apiToken
	if apiToken == "" && existing != nil {
		finalToken = existing.APIToken
	}
	finalToken = s.encryptSecret(finalToken) // encrypt at rest before storage (F-016)

	cfg := &models.QTestConfig{
		ID:          qtestConfigSingletonID,
		BaseURL:     strings.TrimRight(baseURL, "/"),
		Email:       email,
		APIToken:    finalToken,
		ProjectID:   projectID,
		ProjectName: projectName,
		Enabled:     enabled,
		UpdatedAt:   now,
	}

	if existing == nil {
		cfg.CreatedAt = now
		if err := s.db.Create(cfg).Error; err != nil {
			return nil, err
		}
	} else {
		if err := s.db.Model(cfg).Updates(map[string]interface{}{
			"base_url":     cfg.BaseURL,
			"email":        cfg.Email,
			"api_token":    finalToken,
			"project_id":   cfg.ProjectID,
			"project_name": cfg.ProjectName,
			"enabled":      cfg.Enabled,
			"updated_at":   now,
		}).Error; err != nil {
			return nil, err
		}
		cfg.CreatedAt = existing.CreatedAt
	}

	_ = s.logQTestEvent("config_updated")
	return s.GetQTestConfig() // re-read so the returned token is decrypted for masking
}

// ────────────────────────────────────────────────────────────────────────────
// Enabled Projects CRUD (multi-project support)
// ────────────────────────────────────────────────────────────────────────────

// ListEnabledQTestProjects returns all locally enabled QTest projects.
func (s *Store) ListEnabledQTestProjects() ([]models.QTestEnabledProject, error) {
	var projects []models.QTestEnabledProject
	if err := s.db.Order("project_name").Find(&projects).Error; err != nil {
		return nil, err
	}
	return projects, nil
}

// AddEnabledQTestProject adds a project to the enabled list.
func (s *Store) AddEnabledQTestProject(projectID int64, projectName string) (*models.QTestEnabledProject, error) {
	p := &models.QTestEnabledProject{
		ID:          uuid.New().String(),
		ProjectID:   projectID,
		ProjectName: projectName,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	// If this is the first project, make it default
	var count int64
	s.db.Model(&models.QTestEnabledProject{}).Count(&count)
	if count == 0 {
		p.IsDefault = true
	}
	if err := s.db.Create(p).Error; err != nil {
		return nil, err
	}
	return p, nil
}

// RemoveEnabledQTestProject removes a project from the enabled list.
func (s *Store) RemoveEnabledQTestProject(projectID int64) error {
	result := s.db.Where("project_id = ?", projectID).Delete(&models.QTestEnabledProject{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("project not found in enabled list")
	}
	return nil
}

// SetDefaultQTestProject marks one project as default and clears the flag on others.
func (s *Store) SetDefaultQTestProject(projectID int64) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		tx.Model(&models.QTestEnabledProject{}).Where("1=1").Update("is_default", false)
		result := tx.Model(&models.QTestEnabledProject{}).Where("project_id = ?", projectID).Update("is_default", true)
		if result.RowsAffected == 0 {
			return fmt.Errorf("project not found")
		}
		return nil
	})
}

// ────────────────────────────────────────────────────────────────────────────
// T007: Mapping CRUD
// ────────────────────────────────────────────────────────────────────────────

// CreateQTestMapping creates a new mapping between a TTGO test case and a QTest test case.
func (s *Store) CreateQTestMapping(testCaseID string, qtestTCID int64, qtestPID, modulePath string, moduleID int64, qtestURL, contentHash string, projectID int64) (*models.QTestMapping, error) {
	return s.createQTestMappingTx(s.db, testCaseID, qtestTCID, qtestPID, modulePath, moduleID, qtestURL, contentHash, projectID)
}

func (s *Store) createQTestMappingTx(tx *gorm.DB, testCaseID string, qtestTCID int64, qtestPID, modulePath string, moduleID int64, qtestURL, contentHash string, projectID int64) (*models.QTestMapping, error) {
	now := time.Now()
	m := &models.QTestMapping{
		ID:               uuid.New().String(),
		TestCaseID:       testCaseID,
		QTestTestCaseID:  qtestTCID,
		QTestTestCasePID: qtestPID,
		QTestModuleID:    moduleID,
		QTestModulePath:  modulePath,
		QTestProjectID:   projectID,
		QTestURL:         qtestURL,
		ContentHash:      contentHash,
		SyncStatus:       "synced",
		LastSyncedAt:     &now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := tx.Create(m).Error; err != nil {
		return nil, err
	}
	return m, nil
}

// GetQTestMappingByTestCase returns the QTest mapping for a given TTGO test case, or nil.
func (s *Store) GetQTestMappingByTestCase(testCaseID string) (*models.QTestMapping, error) {
	return s.getQTestMappingByTestCaseTx(s.db, testCaseID)
}

func (s *Store) getQTestMappingByTestCaseTx(tx *gorm.DB, testCaseID string) (*models.QTestMapping, error) {
	var m models.QTestMapping
	query := tx.Where("test_case_id = ?", testCaseID).Limit(1).Find(&m)
	if query.Error != nil {
		return nil, query.Error
	}
	if query.RowsAffected == 0 {
		return nil, nil
	}
	return &m, nil
}

// GetQTestMappingsByTestCases returns QTest mappings for multiple test cases in a single query.
// Only returns entries that exist (linked cases).
func (s *Store) GetQTestMappingsByTestCases(testCaseIDs []string) (map[string]*models.QTestMapping, error) {
	if len(testCaseIDs) == 0 {
		return map[string]*models.QTestMapping{}, nil
	}
	var mappings []models.QTestMapping
	if err := s.db.Where("test_case_id IN ?", testCaseIDs).Find(&mappings).Error; err != nil {
		return nil, err
	}
	result := make(map[string]*models.QTestMapping, len(mappings))
	for i := range mappings {
		result[mappings[i].TestCaseID] = &mappings[i]
	}
	return result, nil
}

// DeleteQTestMapping removes the QTest mapping for a test case.
func (s *Store) DeleteQTestMapping(testCaseID string) error {
	result := s.db.Where("test_case_id = ?", testCaseID).Delete(&models.QTestMapping{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("no QTest mapping found for this test case")
	}
	_ = s.logQTestEvent(fmt.Sprintf("mapping_unlinked:%s", testCaseID))
	return nil
}

// BulkDeleteQTestMappings removes mappings for the given test case IDs and
// returns the number of rows deleted.
func (s *Store) BulkDeleteQTestMappings(testCaseIDs []string) (int, error) {
	if len(testCaseIDs) == 0 {
		return 0, nil
	}
	result := s.db.Where("test_case_id IN ?", testCaseIDs).Delete(&models.QTestMapping{})
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected > 0 {
		_ = s.logQTestEvent(fmt.Sprintf("mappings_bulk_unlinked:%d", result.RowsAffected))
	}
	return int(result.RowsAffected), nil
}

// UnlinkQTestMappingsByFolder removes mappings for every test case under
// folderID. When recursive is true, descendant folders are included as well.
// Returns the number of rows deleted.
func (s *Store) UnlinkQTestMappingsByFolder(folderID string, recursive bool) (int, error) {
	if folderID == "" {
		return 0, fmt.Errorf("folder_id is required")
	}

	var folderIDs []string
	if recursive {
		ids, err := s.GetFolderDescendants(folderID)
		if err != nil {
			return 0, fmt.Errorf("failed to walk folder descendants: %v", err)
		}
		folderIDs = ids
	} else {
		folderIDs = []string{folderID}
	}
	if len(folderIDs) == 0 {
		return 0, nil
	}

	var testCaseIDs []string
	if err := s.db.Model(&models.TestCase{}).Where("folder_id IN ?", folderIDs).Pluck("id", &testCaseIDs).Error; err != nil {
		return 0, fmt.Errorf("failed to list test cases: %v", err)
	}
	return s.BulkDeleteQTestMappings(testCaseIDs)
}

// ListQTestMappings returns all QTest mappings, optionally filtered by sync status.
func (s *Store) ListQTestMappings(statusFilter string) ([]models.QTestMapping, error) {
	var mappings []models.QTestMapping
	q := s.db.Model(&models.QTestMapping{})
	if statusFilter != "" {
		q = q.Where("sync_status = ?", statusFilter)
	}
	if err := q.Find(&mappings).Error; err != nil {
		return nil, err
	}
	return mappings, nil
}

// ────────────────────────────────────────────────────────────────────────────
// T008: Mapping update helpers
// ────────────────────────────────────────────────────────────────────────────

// UpdateQTestMappingSyncStatus updates the sync status and optional error message.
func (s *Store) UpdateQTestMappingSyncStatus(id, status, errorMsg string) error {
	return s.db.Model(&models.QTestMapping{}).Where("id = ?", id).Updates(map[string]interface{}{
		"sync_status":   status,
		"error_message": errorMsg,
		"updated_at":    time.Now(),
	}).Error
}

// UpdateQTestMappingAfterSync updates the content hash and timestamp after a successful sync.
func (s *Store) UpdateQTestMappingAfterSync(id, contentHash string) error {
	now := time.Now()
	return s.db.Model(&models.QTestMapping{}).Where("id = ?", id).Updates(map[string]interface{}{
		"content_hash":   contentHash,
		"sync_status":    "synced",
		"error_message":  "",
		"last_synced_at": now,
		"updated_at":     now,
	}).Error
}

// ────────────────────────────────────────────────────────────────────────────
// T010 + T012: QTest API calls for connection test and project listing
// ────────────────────────────────────────────────────────────────────────────

// TestQTestConnection validates the stored QTest credentials by listing projects.
func (s *Store) TestQTestConnection(cfg *models.QTestConfig) (bool, string, error) {
	resp, err := s.qtestRequest(cfg, http.MethodGet, "/api/v3/projects", nil)
	if err != nil {
		return false, err.Error(), nil
	}
	defer resp.Body.Close()

	if checkErr := qtestCheckStatus(resp); checkErr != nil {
		return false, checkErr.Error(), nil
	}
	return true, "Connected successfully", nil
}

// FetchQTestProjectsFromAPI returns available projects from the configured QTest instance.
func (s *Store) FetchQTestProjectsFromAPI(cfg *models.QTestConfig) ([]models.QTestProject, error) {
	resp, err := s.qtestRequest(cfg, http.MethodGet, "/api/v3/projects", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if checkErr := qtestCheckStatus(resp); checkErr != nil {
		return nil, checkErr
	}

	var projects []models.QTestProject
	if err := decodeQTestResponse(resp.Body, &projects); err != nil {
		return nil, fmt.Errorf("failed to parse QTest projects response: %v", err)
	}
	return projects, nil
}

// ────────────────────────────────────────────────────────────────────────────
// T017 + T026: QTest test case CRUD
// ────────────────────────────────────────────────────────────────────────────

// createQTestTestCase creates a test case in QTest via POST /api/v3/projects/{id}/test-cases.
// Returns the QTest test case ID and PID (display ID like "TC-123").
func (s *Store) createQTestTestCase(cfg *models.QTestConfig, projectID int64, moduleID int64, tc *models.TestCase, steps []*models.TestStep) (int64, string, error) {
	// Build test_steps array mapping TTGO fields → QTest fields
	qtestSteps := make([]map[string]interface{}, len(steps))
	for i, step := range steps {
		qtestSteps[i] = map[string]interface{}{
			"description": step.Action,         // TTGO Action → QTest description
			"expected":    step.ExpectedResult, // TTGO ExpectedResult → QTest expected
		}
	}

	// Strip HTML tags from description for QTest compatibility
	desc := stripHTMLTags(tc.Description)

	payload := map[string]interface{}{
		"name":        tc.Name,
		"description": desc,
		"parent_id":   moduleID,
		"test_steps":  qtestSteps,
		"properties":  []interface{}{},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return 0, "", fmt.Errorf("failed to build QTest request: %v", err)
	}

	resp, err := s.qtestRequest(cfg, http.MethodPost,
		fmt.Sprintf("/api/v3/projects/%d/test-cases", projectID),
		strings.NewReader(string(body)))
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	if checkErr := qtestCheckStatus(resp); checkErr != nil {
		return 0, "", checkErr
	}

	var created struct {
		ID  int64  `json:"id"`
		PID string `json:"pid"`
	}
	if err := decodeQTestResponse(resp.Body, &created); err != nil {
		return 0, "", fmt.Errorf("failed to parse QTest create response: %v", err)
	}

	return created.ID, created.PID, nil
}

// updateQTestTestCase updates an existing test case in QTest via PUT.
func (s *Store) updateQTestTestCase(cfg *models.QTestConfig, projectID int64, qtestTCID int64, tc *models.TestCase, steps []*models.TestStep) error {
	qtestSteps := make([]map[string]interface{}, len(steps))
	for i, step := range steps {
		qtestSteps[i] = map[string]interface{}{
			"description": step.Action,
			"expected":    step.ExpectedResult,
		}
	}

	desc := stripHTMLTags(tc.Description)

	payload := map[string]interface{}{
		"name":        tc.Name,
		"description": desc,
		"test_steps":  qtestSteps,
		"properties":  []interface{}{},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to build QTest update request: %v", err)
	}

	resp, err := s.qtestRequest(cfg, http.MethodPut,
		fmt.Sprintf("/api/v3/projects/%d/test-cases/%d", projectID, qtestTCID),
		strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if checkErr := qtestCheckStatus(resp); checkErr != nil {
		return checkErr
	}
	return nil
}

// ────────────────────────────────────────────────────────────────────────────
// T018: Upload (bulk)
// ────────────────────────────────────────────────────────────────────────────

// UploadTestCasesToQTest uploads one or more TTGO test cases to QTest.
func (s *Store) UploadTestCasesToQTest(testCaseIDs []string, moduleID int64, modulePath string, onConflict string, projectID int64) (*models.QTestBulkResult, error) {
	cfg, err := s.GetQTestConfig()
	if err != nil {
		return nil, err
	}
	if cfg == nil || !cfg.Enabled {
		return nil, fmt.Errorf("QTest integration is not configured or disabled")
	}

	result := &models.QTestBulkResult{Total: len(testCaseIDs)}

	for _, tcID := range testCaseIDs {
		tc, steps, fetchErr := s.getTestCaseWithSteps(tcID)
		if fetchErr != nil {
			result.Failed++
			result.Items = append(result.Items, models.QTestBulkResultItem{
				TestCaseID: tcID, Status: "failed", Error: fetchErr.Error(),
			})
			continue
		}

		// Check existing mapping
		existing, _ := s.GetQTestMappingByTestCase(tcID)
		if existing != nil {
			if onConflict == "skip" {
				result.Skipped++
				result.Items = append(result.Items, models.QTestBulkResultItem{
					TestCaseID: tcID, TestCaseName: tc.Name, Status: "skipped",
					QTestTestCaseID: existing.QTestTestCaseID, QTestURL: existing.QTestURL,
				})
				continue
			}
			// onConflict == "update": update existing QTest test case
			// Use the mapping's project ID for updates (it was uploaded to that project)
			updateProjectID := existing.QTestProjectID
			if updateProjectID == 0 {
				updateProjectID = projectID
			}
			updateErr := s.updateQTestTestCase(cfg, updateProjectID, existing.QTestTestCaseID, tc, steps)
			if updateErr != nil {
				if isQTestRateLimited(updateErr) {
					result.RateLimited = true
					result.Items = append(result.Items, models.QTestBulkResultItem{
						TestCaseID: tcID, TestCaseName: tc.Name, Status: "rate_limited",
					})
					break
				}
				result.Failed++
				result.Items = append(result.Items, models.QTestBulkResultItem{
					TestCaseID: tcID, TestCaseName: tc.Name, Status: "failed", Error: updateErr.Error(),
				})
				continue
			}
			hash := models.ComputeTestCaseContentHash(tc.Name, tc.Description, steps)
			_ = s.UpdateQTestMappingAfterSync(existing.ID, hash)
			result.Succeeded++
			result.Items = append(result.Items, models.QTestBulkResultItem{
				TestCaseID: tcID, TestCaseName: tc.Name, Status: "success",
				QTestTestCaseID: existing.QTestTestCaseID, QTestURL: existing.QTestURL,
			})
			continue
		}

		// Create new test case in QTest
		qtestID, qtestPID, createErr := s.createQTestTestCase(cfg, projectID, moduleID, tc, steps)
		if createErr != nil {
			if isQTestRateLimited(createErr) {
				result.RateLimited = true
				result.Items = append(result.Items, models.QTestBulkResultItem{
					TestCaseID: tcID, TestCaseName: tc.Name, Status: "rate_limited",
				})
				break
			}
			result.Failed++
			result.Items = append(result.Items, models.QTestBulkResultItem{
				TestCaseID: tcID, TestCaseName: tc.Name, Status: "failed", Error: createErr.Error(),
			})
			continue
		}

		// Build QTest URL
		qtestURL := fmt.Sprintf("%s/p/%d/portal/project#id=%d", cfg.BaseURL, projectID, qtestID)

		hash := models.ComputeTestCaseContentHash(tc.Name, tc.Description, steps)
		_, mappingErr := s.CreateQTestMapping(tcID, qtestID, qtestPID, modulePath, moduleID, qtestURL, hash, projectID)
		if mappingErr != nil {
			result.Failed++
			result.Items = append(result.Items, models.QTestBulkResultItem{
				TestCaseID: tcID, TestCaseName: tc.Name, Status: "failed",
				Error: "uploaded to QTest but failed to save mapping: " + mappingErr.Error(),
			})
			continue
		}

		_ = s.logQTestEvent(fmt.Sprintf("uploaded:%s→%d", tcID, qtestID))

		result.Succeeded++
		result.Items = append(result.Items, models.QTestBulkResultItem{
			TestCaseID: tcID, TestCaseName: tc.Name, Status: "success",
			QTestTestCaseID: qtestID, QTestURL: qtestURL,
		})
	}

	return result, nil
}

// ────────────────────────────────────────────────────────────────────────────
// T027: Sync (bulk)
// ────────────────────────────────────────────────────────────────────────────

// SyncTestCasesToQTest pushes local changes to QTest for mapped test cases.
func (s *Store) SyncTestCasesToQTest(testCaseIDs []string) (*models.QTestBulkResult, error) {
	cfg, err := s.GetQTestConfig()
	if err != nil {
		return nil, err
	}
	if cfg == nil || !cfg.Enabled {
		return nil, fmt.Errorf("QTest integration is not configured or disabled")
	}

	// If no specific IDs given, sync all mappings with pending changes
	var mappings []models.QTestMapping
	if len(testCaseIDs) == 0 {
		mappings, err = s.ListQTestMappings("changes_pending")
		if err != nil {
			return nil, err
		}
	} else {
		for _, tcID := range testCaseIDs {
			m, _ := s.GetQTestMappingByTestCase(tcID)
			if m != nil {
				mappings = append(mappings, *m)
			}
		}
	}

	result := &models.QTestBulkResult{Total: len(mappings)}

	for _, mapping := range mappings {
		tc, steps, fetchErr := s.getTestCaseWithSteps(mapping.TestCaseID)
		if fetchErr != nil {
			result.Failed++
			result.Items = append(result.Items, models.QTestBulkResultItem{
				TestCaseID: mapping.TestCaseID, Status: "failed", Error: fetchErr.Error(),
			})
			continue
		}

		// Check if content actually changed
		currentHash := models.ComputeTestCaseContentHash(tc.Name, tc.Description, steps)
		if currentHash == mapping.ContentHash {
			result.Skipped++
			result.Items = append(result.Items, models.QTestBulkResultItem{
				TestCaseID: mapping.TestCaseID, TestCaseName: tc.Name, Status: "skipped",
				QTestTestCaseID: mapping.QTestTestCaseID, QTestURL: mapping.QTestURL,
			})
			continue
		}

		// Use the mapping's stored project ID for the update call
		projectID := mapping.QTestProjectID
		if projectID == 0 {
			projectID = cfg.ProjectID // fallback to legacy config
		}

		// Push update to QTest
		updateErr := s.updateQTestTestCase(cfg, projectID, mapping.QTestTestCaseID, tc, steps)
		if updateErr != nil {
			if isQTestRateLimited(updateErr) {
				result.RateLimited = true
				result.Items = append(result.Items, models.QTestBulkResultItem{
					TestCaseID: mapping.TestCaseID, TestCaseName: tc.Name, Status: "rate_limited",
				})
				break
			}
			if isQTestNotFound(updateErr) {
				_ = s.UpdateQTestMappingSyncStatus(mapping.ID, "broken", "QTest test case no longer exists")
				result.Failed++
				result.Items = append(result.Items, models.QTestBulkResultItem{
					TestCaseID: mapping.TestCaseID, TestCaseName: tc.Name, Status: "failed",
					Error: "QTest test case has been deleted — mapping marked as broken",
				})
				continue
			}
			result.Failed++
			result.Items = append(result.Items, models.QTestBulkResultItem{
				TestCaseID: mapping.TestCaseID, TestCaseName: tc.Name, Status: "failed",
				Error: updateErr.Error(),
			})
			continue
		}

		_ = s.UpdateQTestMappingAfterSync(mapping.ID, currentHash)
		_ = s.logQTestEvent(fmt.Sprintf("synced:%s", mapping.TestCaseID))

		result.Succeeded++
		result.Items = append(result.Items, models.QTestBulkResultItem{
			TestCaseID: mapping.TestCaseID, TestCaseName: tc.Name, Status: "success",
			QTestTestCaseID: mapping.QTestTestCaseID, QTestURL: mapping.QTestURL,
		})
	}

	return result, nil
}

// ────────────────────────────────────────────────────────────────────────────
// T036: Module tree
// ────────────────────────────────────────────────────────────────────────────

// ListQTestModules returns the module (folder) hierarchy for a given project.
func (s *Store) ListQTestModules(cfg *models.QTestConfig, projectID int64) ([]*models.QTestModule, error) {
	resp, err := s.qtestRequest(cfg, http.MethodGet,
		fmt.Sprintf("/api/v3/projects/%d/modules?expand=descendants", projectID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if checkErr := qtestCheckStatus(resp); checkErr != nil {
		return nil, checkErr
	}

	var modules []*models.QTestModule
	if err := decodeQTestResponse(resp.Body, &modules); err != nil {
		return nil, fmt.Errorf("failed to parse QTest modules response: %v", err)
	}
	return normalizeQTestModules(modules), nil
}

// FetchQTestTestCases returns test cases from a QTest module.
func (s *Store) FetchQTestTestCases(cfg *models.QTestConfig, projectID int64, moduleID int64) ([]models.QTestRemoteTestCase, error) {
	var result []models.QTestRemoteTestCase
	for page := 1; ; page++ {
		resp, err := s.qtestRequest(cfg, http.MethodGet,
			fmt.Sprintf("/api/v3/projects/%d/test-cases?parentId=%d&expandSteps=true&page=%d&size=%d", projectID, moduleID, page, qtestPageSize), nil)
		if err != nil {
			return nil, err
		}

		if checkErr := qtestCheckStatus(resp); checkErr != nil {
			resp.Body.Close()
			return nil, checkErr
		}

		var raw []struct {
			ID          int64             `json:"id"`
			PID         string            `json:"pid"`
			Name        string            `json:"name"`
			Description string            `json:"description"`
			ParentID    int64             `json:"parent_id"`
			Properties  []json.RawMessage `json:"properties"`
			TestSteps   []struct {
				Description string `json:"description"`
				Expected    string `json:"expected"`
			} `json:"test_steps"`
		}
		if err := decodeQTestResponse(resp.Body, &raw); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to parse QTest test cases response: %v", err)
		}
		resp.Body.Close()

		for _, r := range raw {
			steps := make([]models.QTestRemoteStep, len(r.TestSteps))
			for j, step := range r.TestSteps {
				steps[j] = models.QTestRemoteStep{
					Description: step.Description,
					Expected:    step.Expected,
				}
			}
			properties := make([]models.QTestProperty, 0, len(r.Properties))
			for _, property := range r.Properties {
				properties = append(properties, parseQTestProperty(property))
			}
			result = append(result, models.QTestRemoteTestCase{
				ID:          r.ID,
				PID:         r.PID,
				Name:        r.Name,
				Description: r.Description,
				ParentID:    r.ParentID,
				ModuleID:    r.ParentID,
				Properties:  properties,
				Steps:       steps,
			})
		}

		if len(raw) < qtestPageSize {
			break
		}
	}

	return result, nil
}

func normalizeQTestModules(modules []*models.QTestModule) []*models.QTestModule {
	moduleMap := make(map[int64]*models.QTestModule)
	order := make([]int64, 0)

	var visit func(list []*models.QTestModule, parentID *int64)
	visit = func(list []*models.QTestModule, parentID *int64) {
		for _, module := range list {
			if module == nil {
				continue
			}

			normalized, ok := moduleMap[module.ID]
			if !ok {
				normalized = &models.QTestModule{
					ID:   module.ID,
					Name: module.Name,
					Path: module.Path,
				}
				moduleMap[module.ID] = normalized
				order = append(order, module.ID)
			}
			if strings.TrimSpace(normalized.Name) == "" {
				normalized.Name = module.Name
			}
			if strings.TrimSpace(normalized.Path) == "" {
				normalized.Path = module.Path
			}
			if normalized.ParentID == nil {
				switch {
				case module.ParentID != nil:
					parent := *module.ParentID
					normalized.ParentID = &parent
				case parentID != nil:
					parent := *parentID
					normalized.ParentID = &parent
				}
			}

			visit(module.Children, &normalized.ID)
		}
	}

	visit(modules, nil)

	if len(order) == 0 {
		return nil
	}

	var roots []*models.QTestModule
	for _, id := range order {
		module := moduleMap[id]
		module.Children = nil
	}
	for _, id := range order {
		module := moduleMap[id]
		if module.ParentID == nil {
			roots = append(roots, module)
			continue
		}
		parent := moduleMap[*module.ParentID]
		if parent == nil || parent.ID == module.ID {
			roots = append(roots, module)
			continue
		}
		parent.Children = append(parent.Children, module)
	}

	if len(roots) == 0 {
		roots = make([]*models.QTestModule, 0, len(order))
		for _, id := range order {
			roots = append(roots, moduleMap[id])
		}
	}

	ensureQTestModulePaths(roots, "")
	return roots
}

func findQTestModuleByID(modules []*models.QTestModule, moduleID int64) *models.QTestModule {
	for _, module := range modules {
		if module == nil {
			continue
		}
		if module.ID == moduleID {
			return module
		}
		if found := findQTestModuleByID(module.Children, moduleID); found != nil {
			return found
		}
	}
	return nil
}

func collectQTestModuleNodes(root *models.QTestModule) []*models.QTestModule {
	if root == nil {
		return nil
	}
	result := []*models.QTestModule{root}
	for _, child := range root.Children {
		result = append(result, collectQTestModuleNodes(child)...)
	}
	return result
}

func ensureQTestModulePaths(modules []*models.QTestModule, parentPath string) {
	for _, module := range modules {
		if module == nil {
			continue
		}
		if strings.TrimSpace(module.Path) == "" {
			if parentPath == "" {
				module.Path = module.Name
			} else {
				module.Path = parentPath + " / " + module.Name
			}
		}
		ensureQTestModulePaths(module.Children, module.Path)
	}
}

func qtestPropertyStringFromMap(raw map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := raw[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return strings.TrimSpace(typed)
			}
		case map[string]any:
			for _, nestedKey := range []string{"name", "label", "value", "activeValue"} {
				if nestedValue, ok := typed[nestedKey].(string); ok && strings.TrimSpace(nestedValue) != "" {
					return strings.TrimSpace(nestedValue)
				}
			}
		}
	}
	return ""
}

func parseQTestProperty(rawJSON json.RawMessage) models.QTestProperty {
	property := models.QTestProperty{Raw: rawJSON}

	var raw map[string]any
	if err := json.Unmarshal(rawJSON, &raw); err != nil {
		return property
	}

	property.Name = qtestPropertyStringFromMap(raw, "name", "field_name", "fieldName", "label", "display_name", "displayName", "field")
	property.Type = qtestPropertyStringFromMap(raw, "type")
	property.FieldType = qtestPropertyStringFromMap(raw, "field_type", "fieldType")
	property.ValueText = qtestPropertyStringFromMap(raw, "value_text", "valueText", "active_value", "activeValue", "value_name", "valueName", "field_value_name", "fieldValueName")

	if value, ok := raw["value"]; ok && value != nil {
		if encoded, err := json.Marshal(value); err == nil {
			property.Value = encoded
		}
	}

	if property.ValueText == "" && len(property.Value) > 0 {
		var valueMap map[string]any
		if err := json.Unmarshal(property.Value, &valueMap); err == nil {
			property.ValueText = qtestPropertyStringFromMap(valueMap, "activeValue", "value", "label", "name", "valueName", "fieldValueName")
		} else {
			var valueList []any
			if err := json.Unmarshal(property.Value, &valueList); err == nil {
				for _, item := range valueList {
					if itemMap, ok := item.(map[string]any); ok {
						property.ValueText = qtestPropertyStringFromMap(itemMap, "activeValue", "value", "label", "name", "valueName", "fieldValueName")
						if property.ValueText != "" {
							break
						}
					}
					if itemString, ok := item.(string); ok && strings.TrimSpace(itemString) != "" {
						property.ValueText = strings.TrimSpace(itemString)
						break
					}
				}
			}
		}
	}

	if property.ValueText == "" {
		property.ValueText = qtestPropertyStringFromMap(raw, "selected_value", "selectedValue", "display_value", "displayValue")
	}

	return property
}

// FetchQTestTestCasesRecursive returns test cases from the selected QTest module and all descendants.
func (s *Store) FetchQTestTestCasesRecursive(cfg *models.QTestConfig, projectID int64, rootModuleID int64) ([]models.QTestRemoteTestCase, error) {
	modules, err := s.ListQTestModules(cfg, projectID)
	if err != nil {
		return nil, err
	}
	ensureQTestModulePaths(modules, "")

	root := findQTestModuleByID(modules, rootModuleID)
	if root == nil {
		return nil, fmt.Errorf("QTest module could not be found")
	}

	var result []models.QTestRemoteTestCase
	for _, module := range collectQTestModuleNodes(root) {
		testCases, err := s.FetchQTestTestCases(cfg, projectID, module.ID)
		if err != nil {
			return nil, err
		}
		for i := range testCases {
			testCases[i].ModuleID = module.ID
			testCases[i].ModulePath = module.Path
		}
		result = append(result, testCases...)
	}

	return result, nil
}

func qtestPropertyValueText(property models.QTestProperty) string {
	if strings.TrimSpace(property.ValueText) != "" {
		return strings.TrimSpace(property.ValueText)
	}
	if len(property.Value) == 0 || string(property.Value) == "null" {
		return ""
	}

	var directString string
	if err := json.Unmarshal(property.Value, &directString); err == nil {
		return strings.TrimSpace(directString)
	}

	var object map[string]any
	if err := json.Unmarshal(property.Value, &object); err == nil {
		for _, key := range []string{"activeValue", "value", "label", "name", "valueName", "fieldValueName"} {
			if value, ok := object[key].(string); ok && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}

	var array []any
	if err := json.Unmarshal(property.Value, &array); err == nil {
		for _, item := range array {
			if itemString, ok := item.(string); ok && strings.TrimSpace(itemString) != "" {
				return strings.TrimSpace(itemString)
			}
			if itemMap, ok := item.(map[string]any); ok {
				if value := qtestPropertyStringFromMap(itemMap, "activeValue", "value", "label", "name", "valueName", "fieldValueName"); value != "" {
					return value
				}
			}
		}
	}

	if len(property.Raw) > 0 {
		var rawMap map[string]any
		if err := json.Unmarshal(property.Raw, &rawMap); err == nil {
			if value := qtestPropertyStringFromMap(rawMap, "value_text", "valueText", "value_name", "valueName", "field_value_name", "fieldValueName", "selected_value", "selectedValue", "display_value", "displayValue"); value != "" {
				return value
			}
		}
	}

	return ""
}

func matchSelectOption(rawValue string, options []string) string {
	trimmed := strings.TrimSpace(rawValue)
	if trimmed == "" {
		return ""
	}

	for _, option := range options {
		if strings.EqualFold(strings.TrimSpace(option), trimmed) {
			return option
		}
	}

	if idx := strings.LastIndex(trimmed, " - "); idx >= 0 && idx+3 < len(trimmed) {
		suffix := strings.TrimSpace(trimmed[idx+3:])
		for _, option := range options {
			if strings.EqualFold(strings.TrimSpace(option), suffix) {
				return option
			}
		}
	}

	return ""
}

func parseCustomFieldOptions(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}

	var options []string
	if err := json.Unmarshal(raw, &options); err == nil {
		return options
	}

	var encoded string
	if err := json.Unmarshal(raw, &encoded); err == nil && strings.TrimSpace(encoded) != "" {
		if err := json.Unmarshal([]byte(encoded), &options); err == nil {
			return options
		}
	}

	return nil
}

func buildQTestCustomValues(qtc models.QTestRemoteTestCase, qtestIDField models.CustomFieldDefinition, fieldDefsByName map[string]models.CustomFieldDefinition, existing []*models.CustomFieldValue) []*models.CustomFieldValue {
	testCaseLabel := qtc.PID
	if strings.TrimSpace(testCaseLabel) == "" {
		testCaseLabel = fmt.Sprintf("#%d", qtc.ID)
	}
	log.Printf("[DEBUG] qtest import: mapping properties for %s (%s), received %d properties", testCaseLabel, qtc.Name, len(qtc.Properties))

	result := make([]*models.CustomFieldValue, 0, len(existing)+len(qtc.Properties)+1)
	byFieldID := make(map[string]*models.CustomFieldValue, len(existing)+len(qtc.Properties)+1)

	for _, current := range existing {
		if current == nil {
			continue
		}
		copyValue := *current
		byFieldID[current.CustomFieldID] = &copyValue
	}

	qtestIDJSON, _ := json.Marshal(fmt.Sprintf("%d", qtc.ID))
	existingQTestIDValue := byFieldID[qtestIDField.ID]
	byFieldID[qtestIDField.ID] = &models.CustomFieldValue{
		ID: func() string {
			if existingQTestIDValue != nil {
				return existingQTestIDValue.ID
			}
			return ""
		}(),
		CustomFieldID: qtestIDField.ID,
		Value:         qtestIDJSON,
	}
	log.Printf("[DEBUG] qtest import: %s -> mapped property %q to TTGO field %q with value %q", testCaseLabel, "QTestId", qtestIDField.Name, fmt.Sprintf("%d", qtc.ID))

	for _, property := range qtc.Properties {
		propertyName := strings.TrimSpace(property.Name)
		fieldDef, ok := fieldDefsByName[strings.ToLower(propertyName)]
		if !ok {
			log.Printf("[DEBUG] qtest import: %s -> skipped property %q: no matching TTGO custom field", testCaseLabel, propertyName)
			continue
		}

		rawValue := qtestPropertyValueText(property)
		if rawValue == "" {
			log.Printf("[DEBUG] qtest import: %s -> skipped property %q: empty qTest value; raw=%s", testCaseLabel, propertyName, string(property.Raw))
			continue
		}

		var encoded json.RawMessage
		switch fieldDef.Type {
		case models.FieldTypeText:
			encoded, _ = json.Marshal(rawValue)
		case models.FieldTypeSelect:
			options := parseCustomFieldOptions(fieldDef.Options)
			matched := matchSelectOption(rawValue, options)
			if matched == "" {
				log.Printf("[DEBUG] qtest import: %s -> skipped property %q: qTest value %q does not match TTGO options %v", testCaseLabel, propertyName, rawValue, options)
				continue
			}
			encoded, _ = json.Marshal(matched)
			rawValue = matched
		case models.FieldTypeNumber:
			numberValue, err := strconv.ParseFloat(rawValue, 64)
			if err != nil {
				log.Printf("[DEBUG] qtest import: %s -> skipped property %q: value %q is not a valid number", testCaseLabel, propertyName, rawValue)
				continue
			}
			encoded, _ = json.Marshal(numberValue)
		case models.FieldTypeCheckbox:
			lower := strings.ToLower(rawValue)
			if lower != "true" && lower != "false" {
				log.Printf("[DEBUG] qtest import: %s -> skipped property %q: value %q is not a valid checkbox", testCaseLabel, propertyName, rawValue)
				continue
			}
			encoded, _ = json.Marshal(lower == "true")
		default:
			log.Printf("[DEBUG] qtest import: %s -> skipped property %q: unsupported TTGO field type %q", testCaseLabel, propertyName, fieldDef.Type)
			continue
		}

		existingValue := byFieldID[fieldDef.ID]
		byFieldID[fieldDef.ID] = &models.CustomFieldValue{
			ID: func() string {
				if existingValue != nil {
					return existingValue.ID
				}
				return ""
			}(),
			CustomFieldID: fieldDef.ID,
			Value:         encoded,
		}
		log.Printf("[DEBUG] qtest import: %s -> mapped property %q to TTGO field %q with value %q", testCaseLabel, propertyName, fieldDef.Name, rawValue)
	}

	for _, value := range byFieldID {
		result = append(result, value)
	}

	return result
}

// ImportQTestTestCases imports selected QTest test cases into a TTGO folder.
func (s *Store) ImportQTestTestCases(cfg *models.QTestConfig, projectID int64, moduleID int64, modulePath string, folderID string, qtestCases []models.QTestRemoteTestCase, onConflict string, preserveHierarchy bool) (*models.QTestBulkResult, error) {
	var qtestIDField models.CustomFieldDefinition
	if err := s.db.Where("name = ?", "QTestId").First(&qtestIDField).Error; err != nil {
		return nil, fmt.Errorf("custom field 'QTestId' not found — please create it before importing")
	}

	allFieldDefs, err := s.ListCustomFieldDefinitions()
	if err != nil {
		return nil, err
	}
	fieldDefsByName := make(map[string]models.CustomFieldDefinition, len(allFieldDefs))
	for _, fieldDef := range allFieldDefs {
		fieldDefsByName[strings.ToLower(strings.TrimSpace(fieldDef.Name))] = fieldDef
	}

	buildSteps := func(qtc models.QTestRemoteTestCase) []*models.TestStep {
		steps := make([]*models.TestStep, len(qtc.Steps))
		for i, step := range qtc.Steps {
			steps[i] = &models.TestStep{
				Action:         step.Description,
				ExpectedResult: step.Expected,
				OrderIndex:     i,
			}
		}
		return steps
	}

	qtestURLFor := func(qtc models.QTestRemoteTestCase) string {
		return fmt.Sprintf("%s/p/%d/portal/project#id=%d", cfg.BaseURL, projectID, qtc.ID)
	}

	ensureMapping := func(tx *gorm.DB, testCaseID string, qtc models.QTestRemoteTestCase, hash string, effectiveModuleID int64, effectiveModulePath string) error {
		qtestURL := qtestURLFor(qtc)
		existingMapping, err := s.getQTestMappingByTestCaseTx(tx, testCaseID)
		if err != nil {
			return err
		}
		if existingMapping == nil {
			_, err = s.createQTestMappingTx(tx, testCaseID, qtc.ID, qtc.PID, effectiveModulePath, effectiveModuleID, qtestURL, hash, projectID)
			return err
		}
		now := time.Now()
		return tx.Model(&models.QTestMapping{}).
			Where("id = ?", existingMapping.ID).
			Updates(map[string]interface{}{
				"qtest_test_case_id":  qtc.ID,
				"qtest_test_case_pid": qtc.PID,
				"qtest_module_id":     effectiveModuleID,
				"qtest_module_path":   effectiveModulePath,
				"qtest_project_id":    projectID,
				"qtest_url":           qtestURL,
				"content_hash":        hash,
				"sync_status":         "synced",
				"error_message":       "",
				"last_synced_at":      now,
				"updated_at":          now,
			}).Error
	}

	folderCache := map[string]string{"": folderID}
	ensureFolderPath := func(tx *gorm.DB, qtc models.QTestRemoteTestCase) (string, error) {
		if !preserveHierarchy {
			return folderID, nil
		}

		casePath := strings.TrimSpace(qtc.ModulePath)
		rootPath := strings.TrimSpace(modulePath)
		if casePath == "" || rootPath == "" || casePath == rootPath {
			return folderID, nil
		}

		prefix := rootPath + " / "
		if !strings.HasPrefix(casePath, prefix) {
			return folderID, nil
		}

		relativePath := strings.TrimPrefix(casePath, prefix)
		if cachedFolderID, ok := folderCache[relativePath]; ok {
			return cachedFolderID, nil
		}

		currentParentID := folderID
		currentPath := ""
		for _, rawSegment := range strings.Split(relativePath, " / ") {
			segment := strings.TrimSpace(rawSegment)
			if segment == "" {
				continue
			}
			if currentPath == "" {
				currentPath = segment
			} else {
				currentPath += " / " + segment
			}
			if cachedFolderID, ok := folderCache[currentPath]; ok {
				currentParentID = cachedFolderID
				continue
			}

			var existing models.Folder
			query := tx.Where("name = ? AND parent_id = ?", segment, currentParentID).Limit(1).Find(&existing)
			if query.Error != nil {
				return "", query.Error
			}
			if query.RowsAffected == 0 {
				parentID := currentParentID
				created, createErr := s.createFolderTx(tx, segment, &parentID)
				if createErr != nil {
					retry := tx.Where("name = ? AND parent_id = ?", segment, currentParentID).Limit(1).Find(&existing)
					if retry.Error != nil {
						return "", retry.Error
					}
					if retry.RowsAffected == 0 {
						return "", createErr
					}
				} else {
					existing = *created
				}
			}

			currentParentID = existing.ID
			folderCache[currentPath] = existing.ID
		}

		return currentParentID, nil
	}

	result := &models.QTestBulkResult{Total: len(qtestCases)}

	for _, qtc := range qtestCases {
		qtestIDStr := fmt.Sprintf("%d", qtc.ID)
		steps := buildSteps(qtc)
		hash := models.ComputeTestCaseContentHash(qtc.Name, qtc.Description, steps)
		qtestURL := qtestURLFor(qtc)
		item := models.QTestBulkResultItem{
			TestCaseName:    qtc.Name,
			QTestTestCaseID: qtc.ID,
			QTestURL:        qtestURL,
		}
		itemStatus := "failed"

		effectiveModuleID := moduleID
		if qtc.ModuleID != 0 {
			effectiveModuleID = qtc.ModuleID
		}
		effectiveModulePath := modulePath
		if strings.TrimSpace(qtc.ModulePath) != "" {
			effectiveModulePath = qtc.ModulePath
		}

		s.qtestImportMu.Lock()
		err := s.db.Transaction(func(tx *gorm.DB) error {
			destinationFolderID, folderErr := ensureFolderPath(tx, qtc)
			if folderErr != nil {
				return fmt.Errorf("failed to prepare destination folder: %v", folderErr)
			}

			existing, existingErr := s.getTestCaseByCustomFieldTx(tx, "QTestId", qtestIDStr)
			if existingErr != nil && !errors.Is(existingErr, gorm.ErrRecordNotFound) {
				return fmt.Errorf("failed to look up existing test case by QTestId: %v", existingErr)
			}

			if existingErr == nil && existing != nil {
				item.TestCaseID = existing.ID

				if onConflict == "skip" {
					if err := ensureMapping(tx, existing.ID, qtc, hash, effectiveModuleID, effectiveModulePath); err != nil {
						return fmt.Errorf("failed to save qTest mapping: %v", err)
					}
					item.TestCaseName = existing.Name
					itemStatus = "skipped"
					return nil
				}

				existing.Name = qtc.Name
				existing.Description = qtc.Description
				existing.FolderID = destinationFolderID
				existing.Steps = steps
				existing.CustomValues = buildQTestCustomValues(qtc, qtestIDField, fieldDefsByName, existing.CustomValues)
				if err := s.updateTestCaseTx(tx, existing); err != nil {
					return fmt.Errorf("failed to update existing test case: %v", err)
				}
				if err := ensureMapping(tx, existing.ID, qtc, hash, effectiveModuleID, effectiveModulePath); err != nil {
					return fmt.Errorf("failed to save qTest mapping: %v", err)
				}
				if err := s.logQTestEventTx(tx, fmt.Sprintf("reimported:%d→%s", qtc.ID, existing.ID)); err != nil {
					log.Printf("[WARN] qtest import: updated test case %s but failed to write audit log: %v", existing.ID, err)
				}

				item.TestCaseName = qtc.Name
				itemStatus = "success"
				return nil
			}

			tc := &models.TestCase{
				FolderID:     destinationFolderID,
				Name:         qtc.Name,
				Description:  qtc.Description,
				Steps:        steps,
				CustomValues: buildQTestCustomValues(qtc, qtestIDField, fieldDefsByName, nil),
			}

			if err := s.createTestCaseTx(tx, tc); err != nil {
				return fmt.Errorf("failed to create test case: %v", err)
			}
			if err := ensureMapping(tx, tc.ID, qtc, hash, effectiveModuleID, effectiveModulePath); err != nil {
				return fmt.Errorf("failed to save qTest mapping: %v", err)
			}
			if err := s.logQTestEventTx(tx, fmt.Sprintf("imported:%d→%s", qtc.ID, tc.ID)); err != nil {
				log.Printf("[WARN] qtest import: created test case %s but failed to write audit log: %v", tc.ID, err)
			}

			item.TestCaseID = tc.ID
			item.TestCaseName = tc.Name
			itemStatus = "success"
			return nil
		})
		s.qtestImportMu.Unlock()
		if err != nil {
			result.Failed++
			item.Status = "failed"
			item.Error = err.Error()
			result.Items = append(result.Items, item)
			continue
		}

		item.Status = itemStatus
		result.Items = append(result.Items, item)
		if itemStatus == "skipped" {
			result.Skipped++
			continue
		}
		result.Succeeded++
	}

	return result, nil
}

// ────────────────────────────────────────────────────────────────────────────
// Create QTest module
// ────────────────────────────────────────────────────────────────────────────

// CreateQTestModule creates a new module (folder) in QTest under the given parent.
// parentModuleID == 0 means root-level module.
// Returns the created module ID.
func (s *Store) CreateQTestModule(cfg *models.QTestConfig, projectID int64, parentModuleID int64, name string) (int64, error) {
	payload := map[string]interface{}{
		"name": name,
	}
	if parentModuleID != 0 {
		payload["parent_id"] = parentModuleID
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("failed to build QTest create-module request: %v", err)
	}

	resp, err := s.qtestRequest(cfg, http.MethodPost,
		fmt.Sprintf("/api/v3/projects/%d/modules", projectID),
		strings.NewReader(string(body)))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if checkErr := qtestCheckStatus(resp); checkErr != nil {
		return 0, checkErr
	}

	var created struct {
		ID int64 `json:"id"`
	}
	if err := decodeQTestResponse(resp.Body, &created); err != nil {
		return 0, fmt.Errorf("failed to parse QTest create-module response: %v", err)
	}
	return created.ID, nil
}

// ────────────────────────────────────────────────────────────────────────────
// Upload folder to QTest (creates module + uploads all test cases)
// ────────────────────────────────────────────────────────────────────────────

// collectFolderTestCaseIDs recursively collects all test case IDs under a folder.
func (s *Store) collectFolderTestCaseIDs(folderID string) ([]string, error) {
	var ids []string
	// Direct test cases in this folder
	var directIDs []string
	if err := s.db.Model(&models.TestCase{}).Where("folder_id = ?", folderID).Pluck("id", &directIDs).Error; err != nil {
		return nil, err
	}
	ids = append(ids, directIDs...)

	// Recurse into sub-folders
	var subFolderIDs []string
	if err := s.db.Model(&models.Folder{}).Where("parent_id = ?", folderID).Pluck("id", &subFolderIDs).Error; err != nil {
		return nil, err
	}
	for _, sfID := range subFolderIDs {
		subIDs, err := s.collectFolderTestCaseIDs(sfID)
		if err != nil {
			return nil, err
		}
		ids = append(ids, subIDs...)
	}
	return ids, nil
}

// findChildQTestModuleByName searches the module tree for a child of parentModuleID
// (or a root module when parentModuleID == 0) whose name matches exactly. Returns 0
// when no match is found. Used to reuse existing modules during recursive uploads.
func findChildQTestModuleByName(modules []*models.QTestModule, parentModuleID int64, name string) int64 {
	for _, m := range modules {
		if isModuleChildOf(m, parentModuleID) && m.Name == name {
			return m.ID
		}
		if id := findChildQTestModuleByName(m.Children, parentModuleID, name); id != 0 {
			return id
		}
	}
	return 0
}

// isModuleChildOf reports whether m is a direct child of parentModuleID. A nil
// ParentID is treated as root (parent = 0).
func isModuleChildOf(m *models.QTestModule, parentModuleID int64) bool {
	if parentModuleID == 0 {
		return m.ParentID == nil || *m.ParentID == 0
	}
	return m.ParentID != nil && *m.ParentID == parentModuleID
}

// findOrCreateQTestModule looks for an existing module with the given name under
// parentModuleID and returns its ID; otherwise creates a new module. The caller
// passes a snapshot of the project's module tree to avoid re-fetching it for
// every folder during a recursive upload.
func (s *Store) findOrCreateQTestModule(cfg *models.QTestConfig, projectID, parentModuleID int64, name string, modules []*models.QTestModule) (int64, error) {
	if id := findChildQTestModuleByName(modules, parentModuleID, name); id != 0 {
		return id, nil
	}
	return s.CreateQTestModule(cfg, projectID, parentModuleID, name)
}

// UploadFolderToQTest creates a new module in QTest matching the folder name,
// then uploads all test cases in that folder to the new module.
//
// When recursive is true, the entire folder subtree is mirrored: each TTGO
// folder becomes a qTest module under its parent, and each folder's direct test
// cases are uploaded to the matching module. Modules with a name already present
// under the same parent are reused, so re-running with on_conflict=skip is
// idempotent.
func (s *Store) UploadFolderToQTest(folderID string, projectID int64, parentModuleID int64, onConflict string, recursive bool) (*models.QTestBulkResult, int64, error) {
	cfg, err := s.GetQTestConfig()
	if err != nil {
		return nil, 0, err
	}
	if cfg == nil || !cfg.Enabled {
		return nil, 0, fmt.Errorf("QTest integration is not configured or disabled")
	}

	folder, err := s.GetFolder(folderID)
	if err != nil {
		return nil, 0, fmt.Errorf("folder not found: %v", err)
	}

	if !recursive {
		// Create module in QTest
		moduleID, err := s.CreateQTestModule(cfg, projectID, parentModuleID, folder.Name)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create QTest module: %v", err)
		}

		// Collect direct test case IDs in this folder only.
		var testCaseIDs []string
		if err := s.db.Model(&models.TestCase{}).Where("folder_id = ?", folderID).Pluck("id", &testCaseIDs).Error; err != nil {
			return nil, moduleID, fmt.Errorf("failed to list test cases: %v", err)
		}
		if len(testCaseIDs) == 0 {
			return &models.QTestBulkResult{Total: 0}, moduleID, nil
		}
		result, err := s.UploadTestCasesToQTest(testCaseIDs, moduleID, folder.Name, onConflict, projectID)
		if err != nil {
			return nil, moduleID, err
		}
		return result, moduleID, nil
	}

	// Recursive path: mirror the full subtree. Snapshot the module tree once so
	// every level can reuse modules without paying for a fetch per folder.
	modules, err := s.ListQTestModules(cfg, projectID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list QTest modules: %v", err)
	}

	rootModuleID, err := s.findOrCreateQTestModule(cfg, projectID, parentModuleID, folder.Name, modules)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create QTest module %q: %v", folder.Name, err)
	}

	combined := &models.QTestBulkResult{}
	if err := s.uploadFolderSubtree(cfg, projectID, folderID, folder.Name, rootModuleID, onConflict, modules, combined); err != nil {
		return combined, rootModuleID, err
	}
	return combined, rootModuleID, nil
}

// uploadFolderSubtree uploads the direct test cases of folderID to moduleID, then
// recurses into each subfolder. modules is the full project module tree snapshot
// used by findOrCreateQTestModule. Per-folder results are merged into combined so
// the caller sees a single aggregate at the end.
func (s *Store) uploadFolderSubtree(
	cfg *models.QTestConfig,
	projectID int64,
	folderID, modulePath string,
	moduleID int64,
	onConflict string,
	modules []*models.QTestModule,
	combined *models.QTestBulkResult,
) error {
	var testCaseIDs []string
	if err := s.db.Model(&models.TestCase{}).Where("folder_id = ?", folderID).Pluck("id", &testCaseIDs).Error; err != nil {
		return fmt.Errorf("failed to list test cases for folder %s: %v", folderID, err)
	}
	if len(testCaseIDs) > 0 {
		result, err := s.UploadTestCasesToQTest(testCaseIDs, moduleID, modulePath, onConflict, projectID)
		if err != nil {
			return err
		}
		mergeQTestBulkResults(combined, result)
	}

	var subFolders []models.Folder
	if err := s.db.Where("parent_id = ?", folderID).Order("name asc").Find(&subFolders).Error; err != nil {
		return fmt.Errorf("failed to list subfolders of %s: %v", folderID, err)
	}
	for _, sub := range subFolders {
		childModuleID, err := s.findOrCreateQTestModule(cfg, projectID, moduleID, sub.Name, modules)
		if err != nil {
			return fmt.Errorf("failed to create QTest module %q under %q: %v", sub.Name, modulePath, err)
		}
		childPath := modulePath + " / " + sub.Name
		if err := s.uploadFolderSubtree(cfg, projectID, sub.ID, childPath, childModuleID, onConflict, modules, combined); err != nil {
			return err
		}
	}
	return nil
}

// mergeQTestBulkResults accumulates src into dst.
func mergeQTestBulkResults(dst, src *models.QTestBulkResult) {
	if src == nil {
		return
	}
	dst.Total += src.Total
	dst.Succeeded += src.Succeeded
	dst.Failed += src.Failed
	dst.Skipped += src.Skipped
	if src.RateLimited {
		dst.RateLimited = true
	}
	dst.Items = append(dst.Items, src.Items...)
}

// ────────────────────────────────────────────────────────────────────────────
// T032: Sync status for test case list
// ────────────────────────────────────────────────────────────────────────────

// GetQTestSyncStatusMap returns a map of test_case_id → sync_status for all mapped test cases.
// If a mapping exists, the current content hash is compared to detect "changes_pending".
func (s *Store) GetQTestSyncStatusMap() (map[string]*models.QTestMapping, error) {
	var mappings []models.QTestMapping
	if err := s.db.Find(&mappings).Error; err != nil {
		return nil, err
	}
	result := make(map[string]*models.QTestMapping, len(mappings))
	for i := range mappings {
		result[mappings[i].TestCaseID] = &mappings[i]
	}
	return result, nil
}

// ────────────────────────────────────────────────────────────────────────────
// Helpers
// ────────────────────────────────────────────────────────────────────────────

// getTestCaseWithSteps loads a test case and its steps from the database.
func (s *Store) getTestCaseWithSteps(testCaseID string) (*models.TestCase, []*models.TestStep, error) {
	var tc models.TestCase
	if err := s.db.First(&tc, "id = ?", testCaseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, fmt.Errorf("test case not found: %s", testCaseID)
		}
		return nil, nil, err
	}
	var steps []*models.TestStep
	if err := s.db.Where("test_case_id = ?", testCaseID).Order("order_index").Find(&steps).Error; err != nil {
		return nil, nil, err
	}
	return &tc, steps, nil
}

// logQTestEvent creates an audit log entry for QTest operations.
func (s *Store) logQTestEvent(action string) error {
	return s.logQTestEventTx(s.db, action)
}

func (s *Store) logQTestEventTx(tx *gorm.DB, action string) error {
	return tx.Create(&models.AuditLog{
		ID:        uuid.New().String(),
		Action:    fmt.Sprintf("qtest:%s", action),
		Timestamp: time.Now(),
	}).Error
}

// stripHTMLTags removes HTML tags from a string for QTest compatibility.
func stripHTMLTags(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}
	return result.String()
}
