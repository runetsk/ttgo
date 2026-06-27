package websocket

import (
	"time"

	"log/slog"
)

// Hub maintains the set of active WebSocket clients and broadcasts events
// to clients that are subscribed to matching topics and have sufficient permissions.
type Hub struct {
	// Registered clients.
	clients map[*Client]bool

	// Inbound events to broadcast to clients.
	broadcast chan *Event

	// Register requests from new clients.
	register chan *Client

	// Unregister requests from disconnecting clients.
	unregister chan *Client
}

// NewHub creates a new Hub instance.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan *Event, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the Hub's main event loop. It should be launched in a goroutine.
func (h *Hub) Run() {
	statsTicker := time.NewTicker(60 * time.Second)
	defer statsTicker.Stop()
	var broadcastCount int64

	for {
		select {
		case <-statsTicker.C:
			if broadcastCount > 0 {
				slog.Info("ws stats", "clients", len(h.clients), "broadcasts", broadcastCount)
				broadcastCount = 0
			}

		case client := <-h.register:
			// Bound resource use: reject beyond a global cap and a per-user cap so
			// a single user/token cannot exhaust goroutines/FDs/memory (F-040).
			if len(h.clients) >= maxTotalClients {
				slog.Warn("ws connection rejected: global client cap reached", "cap", maxTotalClients)
				client.conn.Close() // reject without closing send (handler still sends an ack)
				continue
			}
			perUser := 0
			for c := range h.clients {
				if c.User != nil && client.User != nil && c.User.ID == client.User.ID {
					perUser++
				}
			}
			if perUser >= maxClientsPerUser {
				slog.Warn("ws connection rejected: per-user cap reached", "user", client.User.Email)
				client.conn.Close()
				continue
			}
			h.clients[client] = true
			slog.Info("ws client connected", "client_id", client.ID, "user", client.User.Email, "clients", len(h.clients))

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				slog.Info("ws client disconnected", "client_id", client.ID, "user", client.User.Email, "clients", len(h.clients))
			}

		case event := <-h.broadcast:
			broadcastCount++
			data, err := event.JSON()
			if err != nil {
				slog.Error("ws failed to marshal event", "type", event.Type, "error", err)
				continue
			}

			for client := range h.clients {
				// Permission check: does the client's role meet the event's MinRole?
				if !roleHasAccess(client.User.Role, event.MinRole) {
					continue
				}

				// Topic check: is the client subscribed to a matching topic?
				if !client.hasTopicMatch(event.Topic) {
					continue
				}

				// Non-blocking send; drop if client buffer is full.
				select {
				case client.send <- data:
				default:
					// Client is too slow — disconnect it and close the connection
					// directly so the FD/goroutines are released immediately (F-067).
					close(client.send)
					delete(h.clients, client)
					client.conn.Close()
					slog.Warn("ws slow client dropped", "client_id", client.ID, "user", client.User.Email)
				}
			}
		}
	}
}

// Broadcast sends an event to the Hub for distribution to matching clients.
// This is safe to call from any goroutine (handler goroutines).
// It is non-blocking; if the broadcast channel is full, the event is dropped with a warning.
func (h *Hub) Broadcast(event *Event) {
	select {
	case h.broadcast <- event:
	default:
		slog.Warn("ws broadcast channel full, dropping event", "type", event.Type, "topic", event.Topic)
	}
}
