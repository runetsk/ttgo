package store

import (
	"errors"
	"log/slog"
	"time"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"

	"gorm.io/gorm"
)

const failureAnalysisSettingsID = "singleton"

// seedFailureAnalysisSettings creates the singleton row if missing and keeps
// default_prompt_template aligned with the shipped default.
func (s *Store) seedFailureAnalysisSettings() error {
	var row models.AIFailureAnalysisSettings
	err := s.db.First(&row, "id = ?", failureAnalysisSettingsID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = models.AIFailureAnalysisSettings{
			ID:                    failureAnalysisSettingsID,
			EnabledOnCompletion:   false,
			MaxAnalysesPerRun:     20,
			DedupEnabled:          true,
			RedactionEnabled:      true,
			PromptTemplate:        failureanalysis.DefaultPromptTemplate,
			DefaultPromptTemplate: failureanalysis.DefaultPromptTemplate,
			CreatedAt:             time.Now(),
			UpdatedAt:             time.Now(),
		}
		return s.db.Create(&row).Error
	}
	if err != nil {
		return err
	}
	// Capture the previously-shipped default before realigning the column below.
	// If the admin never customized the prompt (it still equals that previous
	// default), auto-upgrade it to the new shipped default so unmodified installs
	// adopt template improvements. A customized template (!= old) is left as-is.
	old := row.DefaultPromptTemplate
	if old == failureanalysis.DefaultPromptTemplate {
		return nil
	}
	updates := map[string]interface{}{
		"default_prompt_template": failureanalysis.DefaultPromptTemplate,
	}
	if row.PromptTemplate == old {
		updates["prompt_template"] = failureanalysis.DefaultPromptTemplate
		slog.Info("failure-analysis: auto-upgraded unmodified prompt template to new shipped default")
	}
	return s.db.Model(&row).Updates(updates).Error
}

// GetFailureAnalysisSettings returns the singleton settings row.
func (s *Store) GetFailureAnalysisSettings() (*models.AIFailureAnalysisSettings, error) {
	var row models.AIFailureAnalysisSettings
	if err := s.db.First(&row, "id = ?", failureAnalysisSettingsID).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// UpdateFailureAnalysisSettings overwrites mutable fields. DefaultPromptTemplate is immutable.
func (s *Store) UpdateFailureAnalysisSettings(in *models.AIFailureAnalysisSettings) (*models.AIFailureAnalysisSettings, error) {
	updates := map[string]interface{}{
		"enabled_on_completion": in.EnabledOnCompletion,
		"max_analyses_per_run":  in.MaxAnalysesPerRun,
		"dedup_enabled":         in.DedupEnabled,
		"redaction_enabled":     in.RedactionEnabled,
		"prompt_template":       in.PromptTemplate,
		"updated_at":            time.Now(),
	}
	if err := s.db.Model(&models.AIFailureAnalysisSettings{}).
		Where("id = ?", failureAnalysisSettingsID).
		Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetFailureAnalysisSettings()
}

// ResetFailureAnalysisPrompt restores PromptTemplate to DefaultPromptTemplate.
func (s *Store) ResetFailureAnalysisPrompt() error {
	return s.db.Model(&models.AIFailureAnalysisSettings{}).
		Where("id = ?", failureAnalysisSettingsID).
		Updates(map[string]interface{}{
			"prompt_template": failureanalysis.DefaultPromptTemplate,
			"updated_at":      time.Now(),
		}).Error
}
