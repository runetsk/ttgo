package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
	"ttgo/internal/safehttp"
	"ttgo/pkg/tracker/models"

	"github.com/google/uuid"
)

// WebhookDispatchEntry is a transient struct used by the dispatch worker to log attempts.
type WebhookDispatchEntry struct {
	WebhookID    string
	RunID        string
	Attempt      int
	Status       string
	HTTPCode     *int
	ErrorMsg     string
	DurationMs   int64
	DispatchedAt time.Time
}

// CreateWebhookConfig creates a new webhook configuration.
// URL must be HTTPS.
func (s *Store) CreateWebhookConfig(url, description, eventType string) (*models.WebhookConfig, error) {
	if !strings.HasPrefix(strings.ToLower(url), "https://") {
		return nil, fmt.Errorf("webhook URL must use HTTPS")
	}
	// SSRF guard: reject loopback/private/link-local/metadata destinations at creation
	// time (the dispatch worker's guarded client also re-checks at connect time) (F-001).
	if err := safehttp.ValidatePublicURL(url); err != nil {
		return nil, fmt.Errorf("webhook URL rejected: %w", err)
	}

	// Limit to 10 webhooks
	var count int64
	if err := s.db.Model(&models.WebhookConfig{}).Count(&count).Error; err != nil {
		return nil, err
	}
	if count >= 10 {
		return nil, fmt.Errorf("maximum 10 webhook configurations allowed")
	}

	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, fmt.Errorf("failed to generate webhook secret: %w", err)
	}
	wh := &models.WebhookConfig{
		ID:          uuid.New().String(),
		URL:         url,
		Description: description,
		EventType:   eventType,
		IsActive:    true,
		Secret:      hex.EncodeToString(secretBytes), // HMAC signing key (F-066)
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := s.db.Create(wh).Error; err != nil {
		return nil, err
	}
	return wh, nil
}

// ListWebhookConfigs returns all webhook configurations.
func (s *Store) ListWebhookConfigs() ([]models.WebhookConfig, error) {
	var configs []models.WebhookConfig
	if err := s.db.Order("created_at DESC").Find(&configs).Error; err != nil {
		return nil, err
	}
	return configs, nil
}

// DeleteWebhookConfig deletes a webhook config by ID.
func (s *Store) DeleteWebhookConfig(id string) error {
	return s.db.Delete(&models.WebhookConfig{}, "id = ?", id).Error
}

// GetActiveWebhooks returns all active webhook configs for a given event type.
func (s *Store) GetActiveWebhooks(eventType string) ([]models.WebhookConfig, error) {
	var configs []models.WebhookConfig
	if err := s.db.Where("is_active = ? AND event_type = ?", true, eventType).Find(&configs).Error; err != nil {
		return nil, err
	}
	return configs, nil
}

// GetDispatchLogs returns paginated dispatch logs for a webhook config.
func (s *Store) GetDispatchLogs(webhookID string, limit, offset int) ([]models.WebhookDispatchLog, int64, error) {
	var logs []models.WebhookDispatchLog
	var total int64

	query := s.db.Model(&models.WebhookDispatchLog{}).Where("webhook_id = ?", webhookID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("dispatched_at DESC").Limit(limit).Offset(offset).Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

// SaveDispatchLog persists a webhook dispatch attempt from the worker.
func (s *Store) SaveDispatchLog(entry *WebhookDispatchEntry) error {
	log := &models.WebhookDispatchLog{
		ID:           uuid.New().String(),
		WebhookID:    entry.WebhookID,
		RunID:        entry.RunID,
		Attempt:      entry.Attempt,
		Status:       entry.Status,
		HTTPCode:     entry.HTTPCode,
		ErrorMsg:     entry.ErrorMsg,
		DurationMs:   entry.DurationMs,
		DispatchedAt: entry.DispatchedAt,
		CreatedAt:    time.Now(),
	}
	return s.db.Create(log).Error
}
