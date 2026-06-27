package models

import "time"

// WebhookConfig stores an outbound webhook endpoint configuration.
type WebhookConfig struct {
	ID          string    `json:"id" gorm:"primaryKey"`
	URL         string    `json:"url"`
	Description string    `json:"description"`
	EventType   string    `json:"event_type"` // "run.completed"
	IsActive    bool      `json:"is_active" gorm:"default:true"`
	Secret      string    `json:"-" gorm:"column:secret;default:''"` // HMAC signing key, shown once at creation (F-066)
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// WebhookDispatchLog records each attempt to deliver a webhook event.
type WebhookDispatchLog struct {
	ID           string    `json:"id" gorm:"primaryKey"`
	WebhookID    string    `json:"webhook_id" gorm:"index"`
	RunID        string    `json:"run_id" gorm:"index"`
	Attempt      int       `json:"attempt"`
	Status       string    `json:"status"` // "success" | "failed" | "retrying"
	HTTPCode     *int      `json:"http_code,omitempty"`
	ErrorMsg     string    `json:"error_msg,omitempty"`
	DurationMs   int64     `json:"duration_ms"`
	DispatchedAt time.Time `json:"dispatched_at"`
	CreatedAt    time.Time `json:"created_at"`
}
