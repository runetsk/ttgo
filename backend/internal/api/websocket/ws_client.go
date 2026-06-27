package websocket

import (
	"encoding/json"
	"sync"
	"time"
	"ttgo/pkg/tracker/models"

	"log/slog"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = 30 * time.Second

	// Maximum message size allowed from peer (subscribe/unsubscribe messages are small).
	maxMessageSize = 1024

	// Send channel buffer size per client.
	sendBufferSize = 256

	// Connection caps to bound resource use (F-040).
	maxTotalClients   = 1000
	maxClientsPerUser = 20
)

// Client represents a single WebSocket connection from a browser tab.
type Client struct {
	ID           string
	hub          *Hub
	conn         *websocket.Conn
	User         *models.User
	send         chan []byte
	topics       map[string]bool
	mu           sync.RWMutex     // protects topics map
	sessionToken string           // for periodic re-validation (F-018)
	validate     SessionValidator // re-checks the session is still valid (F-018)
}

// ClientMessage is an inbound message from client to server (subscribe/unsubscribe).
type ClientMessage struct {
	Action string `json:"action"`
	Topic  string `json:"topic"`
}

// newClient creates a new Client for the given connection and user.
func newClient(hub *Hub, conn *websocket.Conn, user *models.User) *Client {
	return &Client{
		ID:     uuid.New().String(),
		hub:    hub,
		conn:   conn,
		User:   user,
		send:   make(chan []byte, sendBufferSize),
		topics: make(map[string]bool),
	}
}

// hasTopicMatch checks whether the client is subscribed to any topic matching the event topic.
func (c *Client) hasTopicMatch(eventTopic string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for sub := range c.topics {
		if topicMatches(sub, eventTopic) {
			return true
		}
	}
	return false
}

// readPump pumps messages from the WebSocket connection to the hub.
// It reads subscribe/unsubscribe messages and updates the client's topics.
// The application runs readPump in a per-connection goroutine.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				slog.Warn("ws readPump unexpected close", "client_id", c.ID, "error", err)
			}
			break
		}

		var msg ClientMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			slog.Warn("ws invalid message from client", "client_id", c.ID, "error", err)
			continue
		}

		switch msg.Action {
		case "subscribe":
			if msg.Topic != "" {
				c.mu.Lock()
				c.topics[msg.Topic] = true
				c.mu.Unlock()
			}
		case "unsubscribe":
			c.mu.Lock()
			delete(c.topics, msg.Topic)
			c.mu.Unlock()
		default:
			slog.Warn("ws unknown action from client", "action", msg.Action, "client_id", c.ID)
		}
	}
}

// writePump pumps messages from the hub to the WebSocket connection.
// A goroutine running writePump is started for each connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			// Re-validate the session each ping so a logged-out / expired /
			// deactivated / role-changed user's socket is torn down within one
			// ping period rather than living for the connection's lifetime (F-018).
			if c.validate != nil {
				u, err := c.validate(c.sessionToken)
				// Close on logout/expiry/deactivation (invalid session) AND on role
				// change, since the hub uses the connect-time role for permission
				// checks — a demoted admin must lose the live socket too (F-018).
				if err != nil || u == nil || u.Role != c.User.Role {
					slog.Info("ws closing connection: session invalid or role changed", "client_id", c.ID)
					return
				}
			}
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
