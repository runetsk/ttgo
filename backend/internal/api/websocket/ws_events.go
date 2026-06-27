package websocket

import (
	"encoding/json"
	"strings"
	"time"
)

// ── Event type constants ──

const (
	EventRunCreated            = "run_created"
	EventRunUpdated            = "run_updated"
	EventRunDeleted            = "run_deleted"
	EventResultUpdated         = "result_updated"
	EventResultBulkUpdated     = "result_bulk_updated"
	EventResultRetried         = "result_retried"
	EventResultDeleted         = "result_deleted"
	EventFolderCreated         = "folder_created"
	EventFolderUpdated         = "folder_updated"
	EventFolderDeleted         = "folder_deleted"
	EventCommentAdded          = "comment_added"
	EventCommentDeleted        = "comment_deleted"
	EventBackupCreated         = "backup_created"
	EventBackupDeleted         = "backup_deleted"
	EventBackupRestored        = "backup_restored"
	EventBackupScheduleUpdated = "backup_schedule_updated"
	EventMaintenanceChanged    = "maintenance_changed"
	EventUserUpdated           = "user_updated"
	EventSettingsChanged       = "settings_changed"
	EventRequirementUpdated    = "requirement_updated"

	EventRunAnalysisProgress      = "run_analysis.progress"
	EventRunAnalysisCompleted     = "run_analysis.completed"
	EventRunResultAnalysisCreated = "run_result_analysis.created"
)

// Role constants for MinRole filtering.
const (
	RoleMember = "member"
	RoleAdmin  = "admin"
)

// Event is a typed message broadcast from the server to connected WebSocket clients.
type Event struct {
	Type      string      `json:"type"`
	Topic     string      `json:"topic"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
	MinRole   string      `json:"-"` // not sent to client; used by Hub for permission filtering
}

// MarshalJSON produces the wire-format JSON for an Event.
func (e *Event) JSON() ([]byte, error) {
	return json.Marshal(struct {
		Type      string      `json:"type"`
		Topic     string      `json:"topic"`
		Timestamp time.Time   `json:"timestamp"`
		Data      interface{} `json:"data"`
	}{
		Type:      e.Type,
		Topic:     e.Topic,
		Timestamp: e.Timestamp,
		Data:      e.Data,
	})
}

// minRoleForEvent returns the minimum user role required to receive the given event type.
func minRoleForEvent(eventType string) string {
	switch eventType {
	case EventBackupCreated, EventBackupDeleted, EventBackupRestored,
		EventBackupScheduleUpdated, EventMaintenanceChanged,
		EventUserUpdated, EventSettingsChanged:
		return RoleAdmin
	default:
		return RoleMember
	}
}

// NewEvent creates an Event with the given type, topic, and data.
// MinRole is automatically determined from the event type.
// Timestamp is set to the current time.
func NewEvent(eventType, topic string, data interface{}) *Event {
	return &Event{
		Type:      eventType,
		Topic:     topic,
		Timestamp: time.Now().UTC(),
		Data:      data,
		MinRole:   minRoleForEvent(eventType),
	}
}

// topicMatches checks whether a client subscription matches the event topic.
// Subscriptions can be exact ("run:abc-123") or wildcard ("runs:*").
// A wildcard subscription "runs:*" matches any topic starting with "runs:" or "run:".
// The special mapping: "runs:*" also matches "run:{id}" topics since they are
// logically part of the same resource collection.
func topicMatches(subscription, eventTopic string) bool {
	if subscription == eventTopic {
		return true
	}
	// Wildcard subscription: "runs:*" matches "runs:*" and "run:xyz"
	if strings.HasSuffix(subscription, ":*") {
		prefix := strings.TrimSuffix(subscription, ":*")
		eventPrefix := strings.SplitN(eventTopic, ":", 2)[0]
		if eventPrefix == prefix {
			return true
		}
		// "runs:*" should also match "run:{id}" topics
		if prefix == "runs" && eventPrefix == "run" {
			return true
		}
	}
	return false
}

// roleHasAccess returns true if the given user role meets or exceeds the minimum role.
func roleHasAccess(userRole, minRole string) bool {
	if minRole == RoleMember {
		return true // both member and admin can see member events
	}
	return userRole == RoleAdmin
}
