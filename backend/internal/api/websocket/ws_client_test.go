package websocket

import (
	"testing"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/assert"
)

func TestClient_HasTopicMatch_ExactMatch(t *testing.T) {
	c := &Client{
		User:   &models.User{Role: "member"},
		topics: map[string]bool{"run:abc": true},
	}
	assert.True(t, c.hasTopicMatch("run:abc"))
	assert.False(t, c.hasTopicMatch("run:xyz"))
}

func TestClient_HasTopicMatch_WildcardMatch(t *testing.T) {
	c := &Client{
		User:   &models.User{Role: "member"},
		topics: map[string]bool{"runs:*": true},
	}
	assert.True(t, c.hasTopicMatch("runs:*"))
	assert.True(t, c.hasTopicMatch("run:abc-123"))
	assert.False(t, c.hasTopicMatch("backups:*"))
	assert.False(t, c.hasTopicMatch("folders:xyz"))
}

func TestClient_HasTopicMatch_MultipleSubscriptions(t *testing.T) {
	c := &Client{
		User:   &models.User{Role: "admin"},
		topics: map[string]bool{"run:abc": true, "backups:*": true},
	}
	assert.True(t, c.hasTopicMatch("run:abc"))
	assert.True(t, c.hasTopicMatch("backups:xyz"))
	assert.False(t, c.hasTopicMatch("run:xyz"))
	assert.False(t, c.hasTopicMatch("settings:*"))
}

func TestClient_HasTopicMatch_NoSubscriptions(t *testing.T) {
	c := &Client{
		User:   &models.User{Role: "member"},
		topics: map[string]bool{},
	}
	assert.False(t, c.hasTopicMatch("run:abc"))
	assert.False(t, c.hasTopicMatch("runs:*"))
}
