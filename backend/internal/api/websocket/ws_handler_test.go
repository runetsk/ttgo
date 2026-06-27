package websocket

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"ttgo/pkg/tracker/models"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wsURL converts an httptest.Server URL to a WebSocket URL.
func wsURL(s *httptest.Server, path string) string {
	return "ws" + strings.TrimPrefix(s.URL, "http") + path
}

func newTestWSServer() (*Hub, *httptest.Server, map[string]*models.User) {
	hub := NewHub()
	go hub.Run()

	sessions := map[string]*models.User{}
	validateSession := func(sessionToken string) (*models.User, error) {
		user, ok := sessions[sessionToken]
		if !ok {
			return nil, errors.New("invalid session")
		}
		return user, nil
	}

	return hub, httptest.NewServer(NewHandler(hub, validateSession, "http://localhost:5173")), sessions
}

func TestWebSocketHandler_RejectsUnauthenticated(t *testing.T) {
	_, ts, _ := newTestWSServer()
	defer ts.Close()

	// Attempt WebSocket connection without session cookie
	_, resp, err := websocket.DefaultDialer.Dial(wsURL(ts, ""), nil)
	require.Error(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestWebSocketHandler_RejectsInvalidSession(t *testing.T) {
	_, ts, _ := newTestWSServer()
	defer ts.Close()

	// Attempt with a fake session token
	header := http.Header{}
	header.Set("Cookie", "session_token=invalid-token-abc")
	_, resp, err := websocket.DefaultDialer.Dial(wsURL(ts, ""), header)
	require.Error(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestWebSocketHandler_ConnectsWithValidSession(t *testing.T) {
	_, ts, sessions := newTestWSServer()
	defer ts.Close()

	sessions["valid-admin"] = &models.User{ID: "u1", Email: "ws@test.com", Role: RoleAdmin}

	header := http.Header{}
	header.Set("Cookie", "session_token=valid-admin")
	header.Set("Origin", "http://localhost:5173")

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL(ts, ""), header)
	require.NoError(t, err)
	defer conn.Close()
	assert.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)

	// Should receive connected acknowledgement
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	require.NoError(t, err)

	var ack map[string]interface{}
	require.NoError(t, json.Unmarshal(msg, &ack))
	assert.Equal(t, "connected", ack["type"])
	data := ack["data"].(map[string]interface{})
	assert.NotEmpty(t, data["client_id"])
}

func TestWebSocketHandler_SubscribeAndReceiveEvents(t *testing.T) {
	hub, ts, sessions := newTestWSServer()
	defer ts.Close()

	sessions["valid-admin"] = &models.User{ID: "u2", Email: "ws2@test.com", Role: RoleAdmin}

	header := http.Header{}
	header.Set("Cookie", "session_token=valid-admin")
	header.Set("Origin", "http://localhost:5173")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(ts, ""), header)
	require.NoError(t, err)
	defer conn.Close()

	// Read and discard "connected" ack
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err = conn.ReadMessage()
	require.NoError(t, err)

	// Subscribe to runs:*
	err = conn.WriteJSON(map[string]string{"action": "subscribe", "topic": "runs:*"})
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond) // let readPump process

	// Broadcast an event through the hub
	hub.Broadcast(NewEvent(EventRunCreated, "runs:*", map[string]string{"id": "new-run"}))

	// Should receive the event
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	require.NoError(t, err)

	var event map[string]interface{}
	require.NoError(t, json.Unmarshal(msg, &event))
	assert.Equal(t, "run_created", event["type"])
	assert.Equal(t, "runs:*", event["topic"])
	data := event["data"].(map[string]interface{})
	assert.Equal(t, "new-run", data["id"])
}

func TestWebSocketHandler_UnsubscribeStopsEvents(t *testing.T) {
	hub, ts, sessions := newTestWSServer()
	defer ts.Close()

	sessions["valid-admin"] = &models.User{ID: "u3", Email: "ws3@test.com", Role: RoleAdmin}

	header := http.Header{}
	header.Set("Cookie", "session_token=valid-admin")
	header.Set("Origin", "http://localhost:5173")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(ts, ""), header)
	require.NoError(t, err)
	defer conn.Close()

	// Read connected ack
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err = conn.ReadMessage()
	require.NoError(t, err)

	// Subscribe
	err = conn.WriteJSON(map[string]string{"action": "subscribe", "topic": "runs:*"})
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	// Unsubscribe
	err = conn.WriteJSON(map[string]string{"action": "unsubscribe", "topic": "runs:*"})
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	// Broadcast — should NOT receive since unsubscribed
	hub.Broadcast(NewEvent(EventRunCreated, "runs:*", nil))

	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, _, err = conn.ReadMessage()
	assert.Error(t, err, "should not receive event after unsubscribe")
}

func TestWebSocketHandler_MemberDoesNotReceiveAdminEvents(t *testing.T) {
	hub, ts, sessions := newTestWSServer()
	defer ts.Close()

	sessions["valid-member"] = &models.User{ID: "u4", Email: "member@test.com", Role: RoleMember}

	header := http.Header{}
	header.Set("Cookie", "session_token=valid-member")
	header.Set("Origin", "http://localhost:5173")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(ts, ""), header)
	require.NoError(t, err)
	defer conn.Close()

	// Read connected ack
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err = conn.ReadMessage()
	require.NoError(t, err)

	// Subscribe to backups:*
	err = conn.WriteJSON(map[string]string{"action": "subscribe", "topic": "backups:*"})
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	// Broadcast admin-only event
	hub.Broadcast(NewEvent(EventBackupCreated, "backups:*", map[string]string{"id": "bk1"}))

	// Member should NOT receive it
	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, _, err = conn.ReadMessage()
	assert.Error(t, err, "member should not receive admin-only events")
}

func TestWebSocketHandler_WildcardRunsReceivesRunIDEvents(t *testing.T) {
	hub, ts, sessions := newTestWSServer()
	defer ts.Close()

	sessions["valid-admin"] = &models.User{ID: "u5", Email: "ws4@test.com", Role: RoleAdmin}

	header := http.Header{}
	header.Set("Cookie", "session_token=valid-admin")
	header.Set("Origin", "http://localhost:5173")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(ts, ""), header)
	require.NoError(t, err)
	defer conn.Close()

	// Read connected ack
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err = conn.ReadMessage()
	require.NoError(t, err)

	// Subscribe to runs:* wildcard
	err = conn.WriteJSON(map[string]string{"action": "subscribe", "topic": "runs:*"})
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	// Broadcast with run:specific-id topic
	hub.Broadcast(NewEvent(EventResultUpdated, "run:specific-id", map[string]string{"status": "PASS"}))

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	require.NoError(t, err)

	var event map[string]interface{}
	require.NoError(t, json.Unmarshal(msg, &event))
	assert.Equal(t, "result_updated", event["type"])
	assert.Equal(t, "run:specific-id", event["topic"])
}
