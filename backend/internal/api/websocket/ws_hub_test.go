package websocket

import (
	"encoding/json"
	"testing"
	"time"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeTestClient creates a Client with the given role and subscriptions for testing.
func makeTestClient(hub *Hub, role string, topics ...string) *Client {
	c := &Client{
		ID:     "test-" + role,
		hub:    hub,
		User:   &models.User{Role: role, Email: role + "@test.com"},
		send:   make(chan []byte, 256),
		topics: make(map[string]bool),
	}
	for _, t := range topics {
		c.topics[t] = true
	}
	return c
}

func TestHub_RegisterAndUnregister(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := makeTestClient(hub, "member", "runs:*")

	// Register
	hub.register <- client
	time.Sleep(50 * time.Millisecond)

	// Broadcast an event to confirm client is registered
	hub.Broadcast(NewEvent(EventRunCreated, "runs:*", map[string]string{"id": "123"}))
	select {
	case msg := <-client.send:
		assert.Contains(t, string(msg), "run_created")
	case <-time.After(time.Second):
		t.Fatal("expected message after register, timed out")
	}

	// Unregister
	hub.unregister <- client
	time.Sleep(50 * time.Millisecond)

	// After unregister, the send channel is closed by the hub.
	// Verify the client was removed by checking that a new broadcast doesn't reach it.
	// The channel is closed, so reads return immediately with ok=false.
	_, ok := <-client.send
	assert.False(t, ok, "send channel should be closed after unregister")
}

func TestHub_BroadcastTopicFiltering(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	runClient := makeTestClient(hub, "member", "run:abc")
	backupClient := makeTestClient(hub, "admin", "backups:*")
	hub.register <- runClient
	hub.register <- backupClient
	time.Sleep(50 * time.Millisecond)

	// Broadcast a run event — only runClient should receive
	hub.Broadcast(NewEvent(EventRunUpdated, "run:abc", map[string]string{"name": "test"}))
	time.Sleep(50 * time.Millisecond)

	select {
	case msg := <-runClient.send:
		assert.Contains(t, string(msg), "run_updated")
	case <-time.After(time.Second):
		t.Fatal("runClient should have received run_updated")
	}

	select {
	case <-backupClient.send:
		t.Fatal("backupClient should NOT receive run_updated (topic mismatch)")
	case <-time.After(200 * time.Millisecond):
		// expected
	}
}

func TestHub_BroadcastRoleFiltering(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Both subscribe to backups:*, but only admin should receive admin events
	memberClient := makeTestClient(hub, "member", "backups:*")
	adminClient := makeTestClient(hub, "admin", "backups:*")
	hub.register <- memberClient
	hub.register <- adminClient
	time.Sleep(50 * time.Millisecond)

	hub.Broadcast(NewEvent(EventBackupCreated, "backups:*", map[string]string{"id": "bk1"}))
	time.Sleep(50 * time.Millisecond)

	// Admin should receive
	select {
	case msg := <-adminClient.send:
		assert.Contains(t, string(msg), "backup_created")
	case <-time.After(time.Second):
		t.Fatal("admin should have received backup_created")
	}

	// Member should NOT receive (admin-only event)
	select {
	case <-memberClient.send:
		t.Fatal("member should NOT receive backup_created (admin event)")
	case <-time.After(200 * time.Millisecond):
		// expected
	}
}

func TestHub_WildcardRunsMatchesRunID(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := makeTestClient(hub, "member", "runs:*")
	hub.register <- client
	time.Sleep(50 * time.Millisecond)

	// "runs:*" subscription should match "run:abc" topic
	hub.Broadcast(NewEvent(EventRunUpdated, "run:abc", map[string]string{"id": "abc"}))
	time.Sleep(50 * time.Millisecond)

	select {
	case msg := <-client.send:
		assert.Contains(t, string(msg), "run_updated")
	case <-time.After(time.Second):
		t.Fatal("runs:* subscriber should receive run:abc events")
	}
}

func TestHub_BroadcastFullPayload(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := makeTestClient(hub, "member", "run:xyz")
	hub.register <- client
	time.Sleep(50 * time.Millisecond)

	payload := map[string]interface{}{
		"id":     "xyz",
		"name":   "Test Run",
		"status": "active",
	}
	hub.Broadcast(NewEvent(EventRunUpdated, "run:xyz", payload))
	time.Sleep(50 * time.Millisecond)

	select {
	case msg := <-client.send:
		var event map[string]interface{}
		require.NoError(t, json.Unmarshal(msg, &event))
		assert.Equal(t, "run_updated", event["type"])
		assert.Equal(t, "run:xyz", event["topic"])
		data := event["data"].(map[string]interface{})
		assert.Equal(t, "xyz", data["id"])
		assert.Equal(t, "Test Run", data["name"])
		assert.Equal(t, "active", data["status"])
	case <-time.After(time.Second):
		t.Fatal("expected message with full payload")
	}
}

func TestHub_BroadcastNonBlocking(t *testing.T) {
	hub := NewHub()
	// Don't start Run() — channel will buffer then drop
	for i := 0; i < 256; i++ {
		hub.Broadcast(NewEvent(EventRunCreated, "runs:*", nil))
	}
	// 257th event should be dropped without blocking
	hub.Broadcast(NewEvent(EventRunCreated, "runs:*", nil))
	// If we get here without hanging, the test passes
}

func TestHub_MultipleSubscriptions(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := makeTestClient(hub, "member", "run:abc", "runs:*")
	hub.register <- client
	time.Sleep(50 * time.Millisecond)

	hub.Broadcast(NewEvent(EventRunUpdated, "run:abc", nil))
	time.Sleep(50 * time.Millisecond)

	// Should receive exactly one message (not duplicated)
	select {
	case <-client.send:
		// good
	case <-time.After(time.Second):
		t.Fatal("should receive at least one message")
	}

	// Should NOT receive a second copy
	select {
	case <-client.send:
		// This is acceptable — the hub sends once per client, not once per matching subscription
		// The hub iterates clients, not subscriptions. So this should not happen.
		t.Fatal("should not receive duplicate message for same client")
	case <-time.After(200 * time.Millisecond):
		// expected
	}
}
