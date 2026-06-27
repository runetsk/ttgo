package store

import (
	"errors"
	"fmt"
	"time"
	"ttgo/pkg/tracker/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ────────────────────────────────────────────────────────────────────────────
// LLM Provider Config CRUD
// ────────────────────────────────────────────────────────────────────────────

// CreateProviderConfig persists a new LLM provider configuration.
func (s *Store) CreateProviderConfig(cfg *models.LLMProviderConfig) error {
	if cfg.ID == "" {
		cfg.ID = uuid.New().String()
	}
	now := time.Now()
	cfg.CreatedAt = now
	cfg.UpdatedAt = now
	plain := cfg.APIKey
	cfg.APIKey = s.encryptSecret(cfg.APIKey) // encrypt at rest (F-016)
	err := s.db.Create(cfg).Error
	cfg.APIKey = plain // restore plaintext for the caller's masked response
	if err != nil {
		if isUniqueConstraintError(err) {
			return fmt.Errorf("a provider with this label already exists")
		}
		return err
	}
	return nil
}

// GetAllProviderConfigs returns all LLM provider configurations ordered by label.
func (s *Store) GetAllProviderConfigs() ([]*models.LLMProviderConfig, error) {
	var cfgs []*models.LLMProviderConfig
	if err := s.db.Order("label").Find(&cfgs).Error; err != nil {
		return nil, err
	}
	for _, c := range cfgs {
		c.APIKey = s.decryptSecret(c.APIKey) // at-rest decryption (F-016)
	}
	return cfgs, nil
}

// GetProviderConfigByID returns a single LLM provider configuration by ID.
func (s *Store) GetProviderConfigByID(id string) (*models.LLMProviderConfig, error) {
	var cfg models.LLMProviderConfig
	if err := s.db.First(&cfg, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("provider config not found")
		}
		return nil, err
	}
	cfg.APIKey = s.decryptSecret(cfg.APIKey) // at-rest decryption (F-016)
	return &cfg, nil
}

// UpdateProviderConfig updates an existing LLM provider configuration.
// If newAPIKey is empty, the existing key is preserved.
func (s *Store) UpdateProviderConfig(id string, updates map[string]interface{}, newAPIKey string) (*models.LLMProviderConfig, error) {
	existing, err := s.GetProviderConfigByID(id)
	if err != nil {
		return nil, err
	}

	if newAPIKey != "" {
		updates["api_key"] = s.encryptSecret(newAPIKey) // encrypt at rest (F-016)
	}
	updates["updated_at"] = time.Now()

	if err := s.db.Model(existing).Updates(updates).Error; err != nil {
		if isUniqueConstraintError(err) {
			return nil, fmt.Errorf("a provider with this label already exists")
		}
		return nil, err
	}
	// Re-fetch to get updated state.
	return s.GetProviderConfigByID(id)
}

// DeleteProviderConfig removes an LLM provider configuration by ID.
func (s *Store) DeleteProviderConfig(id string) error {
	result := s.db.Delete(&models.LLMProviderConfig{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("provider config not found")
	}
	return nil
}

// GetDefaultProviderConfig returns the approved default provider (enabled + is_default),
// or (nil, nil) when none is configured.
func (s *Store) GetDefaultProviderConfig() (*models.LLMProviderConfig, error) {
	var cfg models.LLMProviderConfig
	if err := s.db.Where("is_default = ? AND enabled = ?", true, true).First(&cfg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	cfg.APIKey = s.decryptSecret(cfg.APIKey) // at-rest decryption (F-016)
	return &cfg, nil
}

// SetDefaultProviderConfig sets the given config as default and clears all others.
func (s *Store) SetDefaultProviderConfig(id string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Clear all defaults first.
		if err := tx.Model(&models.LLMProviderConfig{}).
			Where("is_default = ?", true).
			Update("is_default", false).Error; err != nil {
			return err
		}
		// Set the target as default.
		result := tx.Model(&models.LLMProviderConfig{}).
			Where("id = ?", id).
			Update("is_default", true)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("provider config not found")
		}
		return nil
	})
}

// ────────────────────────────────────────────────────────────────────────────
// AI Generation Template (singleton)
// ────────────────────────────────────────────────────────────────────────────

const aiGenTemplateSingletonID = "singleton"

// defaultPromptTemplate is the built-in default template used when no custom template is set.
const defaultPromptTemplate = `You are an expert QA engineer generating test cases for a software requirement.

## Requirement
Title: {{TITLE}}
Description: {{DESCRIPTION}}
{{CHILDREN}}

## Task
Generate test cases at "{{DETAIL_LEVEL}}" detail level.
Coverage: {{COVERAGE}}
{{ADDITIONAL_INSTRUCTIONS}}

## Coverage Guidelines
Distribute tests across these categories as applicable:
- Positive/Happy path: core functionality works as designed
- Negative: invalid inputs, missing data, unauthorized access
- Boundary: min/max values, empty fields, character limits
- Edge case: special characters, concurrency, unusual workflows

## Detail Level
- "Standard": 3–6 clear steps per test case
- "More Detailed": 6–12 granular steps including setup, verification and teardown
- "More Simplified": 1–3 high-level steps summarizing the test intent

## Output Format
Return ONLY a valid JSON array — no markdown fences, no explanation, no extra text.

[
  {
    "name": "Descriptive unique test title",
    "category": "Category name",
    "description": "One sentence: what this test validates and why it matters",
    "steps": [
      {
        "action": "Specific tester action with concrete test data where applicable",
        "expected_result": "Observable, verifiable outcome"
      }
    ]
  }
]

## Quality Rules

### Category Assignment
- Assign exactly one category per test case via the "category" JSON field
- Use one of: Functional, Negative, Boundary, Edge Case, Security, Performance, API, Mobile/Responsive, Accessibility
- If no standard category fits, use a brief descriptive category name

### Naming
- Use the pattern: "[Category] Verb + Object + Condition" (e.g. "[Negative] Submit login form with expired session token")
- Each name must be unique — no two test cases may share the same name
- Keep names under 80 characters; put context in the description

### Step Writing
- Follow the order: Precondition/Setup → Action → Verification for each logical block
- Each action must name the exact UI element, field, button, endpoint, or keyboard shortcut (e.g. "Click the 'Save Draft' button in the upper-right toolbar", not "Save the item")
- Include concrete, realistic test data inline: specific emails (user@example.com), dates (2025-01-01), amounts ($0.00, $9999.99), strings with special characters (O'Brien, <script>), boundary lengths
- One action per step — do not combine multiple interactions into a single step

### Expected Results
- State the exact observable outcome: specific text, numeric value, HTTP status, UI state change, or database effect
- BAD: "Login works correctly", "Error is shown", "Page updates"
- GOOD: "Toast notification displays 'Password updated successfully'", "HTTP 401 Unauthorized is returned with body {\"error\":\"invalid_token\"}", "The 'Submit' button becomes disabled and changes to grey (#9CA3AF)"
- For negative tests: specify both what SHOULD happen (error message, blocked action) AND what should NOT happen (data must not be persisted, navigation must not occur)
- Verify state persistence when relevant: "Refresh the page and confirm the updated email still reads 'new@example.com'"

### Test Independence & Scope
- Each test must be self-contained: include its own setup steps so it can run in isolation without depending on other tests
- Each test must validate one specific behaviour — do not combine unrelated assertions
- No two tests should verify the same condition; overlap wastes execution time
- For negative/boundary tests, always include at least one positive verification step to confirm the system rejects only the invalid input and still works for valid input

### Security & Robustness (when applicable)
- Test authorization: verify that the action fails with an unprivileged or unauthenticated user
- Test input sanitization: include payloads with HTML tags, SQL fragments, or script injections where the requirement involves user-supplied text
- Test idempotency: repeating the same action should produce a consistent result, not duplicates or corruption

## Final Reminder
Follow the coverage guidance above to determine the right number of test cases.`

// defaultParentPromptTemplate is the template used when generating tests for a parent
// requirement with child issues. It is lighter than the standard template — focused on
// coverage across children rather than deep single-requirement quality rules.
const defaultParentPromptTemplate = `You are an expert QA engineer generating test cases for a requirement that has multiple child issues.

## Parent Requirement
Title: {{TITLE}}
Description: {{DESCRIPTION}}

## Child Issues
{{CHILDREN}}

## Task
Generate test cases at "{{DETAIL_LEVEL}}" detail level.
Coverage: {{COVERAGE}}
{{ADDITIONAL_INSTRUCTIONS}}

## Strategy
- Generate at least one test case per child issue to ensure full coverage
- Group related children into a single test case only if they represent the same user flow
- Name each test so it is clear which child issue(s) it covers
- Include cross-cutting tests (e.g. interactions between children) when applicable

## Detail Level
- "Standard": 3–6 clear steps per test case
- "More Detailed": 6–12 granular steps including setup, verification and teardown
- "More Simplified": 1–3 high-level steps summarizing the test intent

## Output Format
Return ONLY a valid JSON array — no markdown fences, no explanation, no extra text.

[
  {
    "name": "Descriptive unique test title",
    "category": "Category name",
    "description": "One sentence: what this test validates and which child issue(s) it covers",
    "steps": [
      {
        "action": "Specific tester action with concrete test data",
        "expected_result": "Observable, verifiable outcome"
      }
    ]
  }
]

## Rules
- Category must be one of: Functional, Negative, Boundary, Edge Case, Security, Performance, API, Mobile/Responsive, Accessibility (or a brief custom category)
- Name pattern: "[Category] Verb + Object + Condition"
- Each name must be unique
- Actions must name exact UI elements and include concrete test data
- Expected results must state the exact observable outcome
- Each test must be self-contained`

// GetOrCreateDefaultTemplate returns the singleton template, creating it with the default content if it doesn't exist.
func (s *Store) GetOrCreateDefaultTemplate() (*models.AIGenTemplate, error) {
	var tmpl models.AIGenTemplate
	err := s.db.First(&tmpl, "id = ?", aiGenTemplateSingletonID).Error
	if err == nil {
		return &tmpl, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	// Create singleton.
	now := time.Now()
	tmpl = models.AIGenTemplate{
		ID:                   aiGenTemplateSingletonID,
		Content:              defaultPromptTemplate,
		DefaultContent:       defaultPromptTemplate,
		ParentContent:        defaultParentPromptTemplate,
		DefaultParentContent: defaultParentPromptTemplate,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := s.db.Create(&tmpl).Error; err != nil {
		return nil, fmt.Errorf("failed to create default template: %w", err)
	}
	return &tmpl, nil
}

// UpdateTemplateContent updates the editable content of the singleton template.
func (s *Store) UpdateTemplateContent(content string) (*models.AIGenTemplate, error) {
	tmpl, err := s.GetOrCreateDefaultTemplate()
	if err != nil {
		return nil, err
	}
	if err := s.db.Model(tmpl).Updates(map[string]interface{}{
		"content":    content,
		"updated_at": time.Now(),
	}).Error; err != nil {
		return nil, err
	}
	tmpl.Content = content
	return tmpl, nil
}

// ResetTemplateToDefault copies default_content back into content.
func (s *Store) ResetTemplateToDefault() (*models.AIGenTemplate, error) {
	tmpl, err := s.GetOrCreateDefaultTemplate()
	if err != nil {
		return nil, err
	}
	if err := s.db.Model(tmpl).Updates(map[string]interface{}{
		"content":    tmpl.DefaultContent,
		"updated_at": time.Now(),
	}).Error; err != nil {
		return nil, err
	}
	tmpl.Content = tmpl.DefaultContent
	return tmpl, nil
}

// UpdateParentTemplateContent updates the parent (children-focused) template content.
func (s *Store) UpdateParentTemplateContent(content string) (*models.AIGenTemplate, error) {
	tmpl, err := s.GetOrCreateDefaultTemplate()
	if err != nil {
		return nil, err
	}
	if err := s.db.Model(tmpl).Updates(map[string]interface{}{
		"parent_content": content,
		"updated_at":     time.Now(),
	}).Error; err != nil {
		return nil, err
	}
	tmpl.ParentContent = content
	return tmpl, nil
}

// ResetParentTemplateToDefault copies default_parent_content back into parent_content.
func (s *Store) ResetParentTemplateToDefault() (*models.AIGenTemplate, error) {
	tmpl, err := s.GetOrCreateDefaultTemplate()
	if err != nil {
		return nil, err
	}
	if err := s.db.Model(tmpl).Updates(map[string]interface{}{
		"parent_content": tmpl.DefaultParentContent,
		"updated_at":     time.Now(),
	}).Error; err != nil {
		return nil, err
	}
	tmpl.ParentContent = tmpl.DefaultParentContent
	return tmpl, nil
}

// ────────────────────────────────────────────────────────────────────────────
// AI Generation Coverage Config (singleton)
// ────────────────────────────────────────────────────────────────────────────

const aiGenCoverageSingletonID = "singleton"

// GetOrCreateCoverageConfig returns the singleton coverage config, creating it with defaults if missing.
func (s *Store) GetOrCreateCoverageConfig() (*models.AIGenCoverageConfig, error) {
	var cfg models.AIGenCoverageConfig
	err := s.db.First(&cfg, "id = ?", aiGenCoverageSingletonID).Error
	if err == nil {
		return &cfg, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	now := time.Now()
	cfg = models.AIGenCoverageConfig{
		ID:                     aiGenCoverageSingletonID,
		EssentialMaxTokens:     4096,
		ThoroughMaxTokens:      8192,
		ComprehensiveMaxTokens: 16384,
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	if err := s.db.Create(&cfg).Error; err != nil {
		return nil, fmt.Errorf("failed to create default coverage config: %w", err)
	}
	return &cfg, nil
}

// UpdateCoverageConfig updates the singleton coverage config and returns it.
func (s *Store) UpdateCoverageConfig(updates map[string]interface{}) (*models.AIGenCoverageConfig, error) {
	cfg, err := s.GetOrCreateCoverageConfig()
	if err != nil {
		return nil, err
	}
	updates["updated_at"] = time.Now()
	if err := s.db.Model(cfg).Updates(updates).Error; err != nil {
		return nil, err
	}
	// Re-read to get updated values.
	return s.GetOrCreateCoverageConfig()
}

// ────────────────────────────────────────────────────────────────────────────
// AI Feature Settings (singleton) — global master switch for all AI features
// ────────────────────────────────────────────────────────────────────────────

// Same literal as the other singleton tables (template, coverage) — each is
// scoped to its own GORM model/table, so the shared value never collides.
const aiFeatureSettingsSingletonID = "singleton"

// GetOrCreateAIFeatureSettings returns the singleton AI feature settings,
// creating it with Enabled=true (AI on by default) when missing.
func (s *Store) GetOrCreateAIFeatureSettings() (*models.AIFeatureSettings, error) {
	var cfg models.AIFeatureSettings
	err := s.db.First(&cfg, "id = ?", aiFeatureSettingsSingletonID).Error
	if err == nil {
		return &cfg, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	now := time.Now()
	cfg = models.AIFeatureSettings{
		ID:        aiFeatureSettingsSingletonID,
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.db.Create(&cfg).Error; err != nil {
		return nil, fmt.Errorf("failed to create default AI feature settings: %w", err)
	}
	return &cfg, nil
}

// UpdateAIFeatureSettings sets the global Enabled flag and returns the row.
// Uses a map so the zero value (false) is written rather than skipped.
func (s *Store) UpdateAIFeatureSettings(enabled bool) (*models.AIFeatureSettings, error) {
	if _, err := s.GetOrCreateAIFeatureSettings(); err != nil {
		return nil, err
	}
	if err := s.db.Model(&models.AIFeatureSettings{}).
		Where("id = ?", aiFeatureSettingsSingletonID).
		Updates(map[string]interface{}{
			"enabled":    enabled,
			"updated_at": time.Now(),
		}).Error; err != nil {
		return nil, err
	}
	return s.GetOrCreateAIFeatureSettings()
}
