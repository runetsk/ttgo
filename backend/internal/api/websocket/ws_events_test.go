package websocket

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── topicMatches ──────────────────────────────────────────────────────────────

func TestTopicMatches_ExactMatch(t *testing.T) {
	assert.True(t, topicMatches("run:abc-123", "run:abc-123"))
	assert.True(t, topicMatches("runs:*", "runs:*"))
	assert.True(t, topicMatches("backups:*", "backups:*"))
}

func TestTopicMatches_ExactMismatch(t *testing.T) {
	assert.False(t, topicMatches("run:abc", "run:xyz"))
	assert.False(t, topicMatches("runs:*", "backups:*"))
}

func TestTopicMatches_WildcardMatchesSamePrefix(t *testing.T) {
	assert.True(t, topicMatches("backups:*", "backups:abc"))
	assert.True(t, topicMatches("folders:*", "folders:xyz"))
	assert.True(t, topicMatches("settings:*", "settings:anything"))
}

func TestTopicMatches_WildcardRunsMatchesRunID(t *testing.T) {
	// "runs:*" should match individual run topics "run:{id}"
	assert.True(t, topicMatches("runs:*", "run:abc-123"))
	assert.True(t, topicMatches("runs:*", "run:some-uuid"))
}

func TestTopicMatches_WildcardDoesNotCrossPrefix(t *testing.T) {
	assert.False(t, topicMatches("backups:*", "run:abc"))
	assert.False(t, topicMatches("runs:*", "folders:abc"))
	assert.False(t, topicMatches("settings:*", "backups:abc"))
}

func TestTopicMatches_NonWildcardDoesNotMatchWildcard(t *testing.T) {
	// A specific subscription should not match a wildcard event topic
	assert.False(t, topicMatches("run:abc", "runs:*"))
}

// ── roleHasAccess ─────────────────────────────────────────────────────────────

func TestRoleHasAccess_MemberEventAllowsBoth(t *testing.T) {
	assert.True(t, roleHasAccess("member", RoleMember))
	assert.True(t, roleHasAccess("admin", RoleMember))
}

func TestRoleHasAccess_AdminEventOnlyAdmin(t *testing.T) {
	assert.True(t, roleHasAccess("admin", RoleAdmin))
	assert.False(t, roleHasAccess("member", RoleAdmin))
}

// ── minRoleForEvent ───────────────────────────────────────────────────────────

func TestMinRoleForEvent_MemberEvents(t *testing.T) {
	memberEvents := []string{
		EventRunCreated, EventRunUpdated, EventRunDeleted,
		EventResultUpdated, EventResultBulkUpdated, EventResultRetried, EventResultDeleted,
		EventFolderCreated, EventFolderUpdated, EventFolderDeleted,
		EventCommentAdded, EventCommentDeleted,
		EventRequirementUpdated,
	}
	for _, evt := range memberEvents {
		assert.Equal(t, RoleMember, minRoleForEvent(evt), "event %s should require member role", evt)
	}
}

func TestMinRoleForEvent_AdminEvents(t *testing.T) {
	adminEvents := []string{
		EventBackupCreated, EventBackupDeleted, EventBackupRestored,
		EventBackupScheduleUpdated, EventMaintenanceChanged,
		EventUserUpdated, EventSettingsChanged,
	}
	for _, evt := range adminEvents {
		assert.Equal(t, RoleAdmin, minRoleForEvent(evt), "event %s should require admin role", evt)
	}
}

// ── NewEvent ──────────────────────────────────────────────────────────────────

func TestNewEvent_SetsFieldsCorrectly(t *testing.T) {
	data := map[string]string{"id": "abc-123"}
	event := NewEvent(EventRunCreated, "runs:*", data)

	assert.Equal(t, EventRunCreated, event.Type)
	assert.Equal(t, "runs:*", event.Topic)
	assert.Equal(t, RoleMember, event.MinRole)
	assert.Equal(t, data, event.Data)
	assert.False(t, event.Timestamp.IsZero())
}

func TestNewEvent_AdminEventMinRole(t *testing.T) {
	event := NewEvent(EventBackupCreated, "backups:*", nil)
	assert.Equal(t, RoleAdmin, event.MinRole)
}

// ── Event.JSON ────────────────────────────────────────────────────────────────

func TestEventJSON_ExcludesMinRole(t *testing.T) {
	event := NewEvent(EventRunUpdated, "run:abc", map[string]string{"name": "test"})

	raw, err := event.JSON()
	require.NoError(t, err)

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &decoded))

	assert.Equal(t, EventRunUpdated, decoded["type"])
	assert.Equal(t, "run:abc", decoded["topic"])
	assert.NotNil(t, decoded["timestamp"])
	assert.NotNil(t, decoded["data"])
	// MinRole must NOT appear in the wire format
	_, hasMinRole := decoded["min_role"]
	assert.False(t, hasMinRole, "MinRole should not be serialized to clients")
}

func TestEventJSON_DataPayload(t *testing.T) {
	type TestData struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	event := NewEvent(EventRunCreated, "runs:*", TestData{Name: "Run1", Status: "active"})
	raw, err := event.JSON()
	require.NoError(t, err)

	var decoded map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &decoded))

	var data TestData
	require.NoError(t, json.Unmarshal(decoded["data"], &data))
	assert.Equal(t, "Run1", data.Name)
	assert.Equal(t, "active", data.Status)
}
