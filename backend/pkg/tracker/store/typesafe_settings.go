package store

import (
	"errors"
	"time"
	"ttgo/pkg/tracker/models"

	"gorm.io/gorm"
)

// ErrKeyAndClearExclusive: a patch may replace the key or clear it, not both.
var ErrKeyAndClearExclusive = errors.New("api_key and clear_api_key are mutually exclusive")

func (s *Store) seedTypeSafeSettings() error {
	var row models.TypeSafeSettings
	err := s.db.First(&row, "id = ?", models.TypeSafeSettingsID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		now := time.Now()
		row = models.TypeSafeSettings{
			ID: models.TypeSafeSettingsID, Enabled: false, Model: models.TypeSafeDefaultModel,
			TimeoutSeconds: 30, VerdictEngineEnabled: true, SemanticDedupEnabled: true,
			AllowAutoFailureAnalysis: false, CreatedAt: now, UpdatedAt: now,
		}
		return s.db.Create(&row).Error
	}
	return err
}

// GetTypeSafeSettings returns the singleton with the key still encrypted.
func (s *Store) GetTypeSafeSettings() (*models.TypeSafeSettings, error) {
	var row models.TypeSafeSettings
	if err := s.db.First(&row, "id = ?", models.TypeSafeSettingsID).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// TypeSafeAPIKey returns the decrypted key ("" when none is stored). Strict: a stored
// value that cannot be decrypted is an error.
func (s *Store) TypeSafeAPIKey() (string, error) {
	row, err := s.GetTypeSafeSettings()
	if err != nil {
		return "", err
	}
	return s.decryptSecretStrict(row.APIKey)
}

// TypeSafeSettingsResponse builds the masked view, reporting the key's status instead of
// failing when it cannot be decrypted.
func (s *Store) TypeSafeSettingsResponse() (models.TypeSafeSettingsResponse, error) {
	row, err := s.GetTypeSafeSettings()
	if err != nil {
		return models.TypeSafeSettingsResponse{}, err
	}
	resp := models.TypeSafeSettingsResponse{
		ID: row.ID, Enabled: row.Enabled, Model: row.Model, TimeoutSeconds: row.TimeoutSeconds,
		VerdictEngineEnabled: row.VerdictEngineEnabled, SemanticDedupEnabled: row.SemanticDedupEnabled,
		AllowAutoFailureAnalysis: row.AllowAutoFailureAnalysis, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		APIKeyStatus: models.TypeSafeKeyStatusMissing,
	}
	if row.APIKey == "" {
		return resp, nil
	}
	plain, derr := s.decryptSecretStrict(row.APIKey)
	if derr != nil {
		resp.APIKeyStatus = models.TypeSafeKeyStatusUndecryptable
		return resp, nil
	}
	resp.APIKeyStatus = models.TypeSafeKeyStatusOK
	if len(plain) >= 4 {
		resp.APIKeyMasked = "…" + plain[len(plain)-4:]
	} else {
		resp.APIKeyMasked = "…"
	}
	return resp, nil
}

// UpdateTypeSafeSettings applies a partial update. Encryption is strict: a patch that
// cannot encrypt its key changes nothing.
func (s *Store) UpdateTypeSafeSettings(p models.TypeSafeSettingsPatch) (*models.TypeSafeSettings, error) {
	if p.ClearAPIKey && p.APIKey != nil && *p.APIKey != "" {
		return nil, ErrKeyAndClearExclusive
	}
	updates := map[string]interface{}{"updated_at": time.Now()}
	if p.Enabled != nil {
		updates["enabled"] = *p.Enabled
	}
	if p.Model != nil {
		updates["model"] = *p.Model
	}
	if p.TimeoutSeconds != nil {
		updates["timeout_seconds"] = *p.TimeoutSeconds
	}
	if p.VerdictEngineEnabled != nil {
		updates["verdict_engine_enabled"] = *p.VerdictEngineEnabled
	}
	if p.SemanticDedupEnabled != nil {
		updates["semantic_dedup_enabled"] = *p.SemanticDedupEnabled
	}
	if p.AllowAutoFailureAnalysis != nil {
		updates["allow_auto_failure_analysis"] = *p.AllowAutoFailureAnalysis
	}
	if p.ClearAPIKey {
		updates["api_key"] = ""
	} else if p.APIKey != nil && *p.APIKey != "" {
		enc, err := s.encryptSecretStrict(*p.APIKey)
		if err != nil {
			return nil, err
		}
		updates["api_key"] = enc
	}
	if err := s.db.Model(&models.TypeSafeSettings{}).
		Where("id = ?", models.TypeSafeSettingsID).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetTypeSafeSettings()
}
