package websocket

import (
	"encoding/json"
	"net/http"
	"strings"
	"ttgo/internal/api/httpx"
	"ttgo/pkg/tracker/models"

	"log/slog"

	"github.com/gorilla/websocket"
)

// SessionValidator resolves a session token into an authenticated user.
type SessionValidator func(sessionToken string) (*models.User, error)

// newUpgrader creates a WebSocket upgrader that checks the origin against the given allowed origin.
func newUpgrader(allowedOrigin string) websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			// Require an Origin for these cookie-authenticated upgrades; a missing
			// Origin must not be a free pass (CSWSH defense) (F-019).
			if origin == "" {
				return false
			}
			for _, a := range strings.Split(allowedOrigin, ",") {
				if strings.TrimSpace(a) == origin {
					return true
				}
			}
			// Exact same-host match (reverse-proxy / LAN) — NOT a suffix match,
			// which the attacker-influenceable Host header could satisfy (F-019).
			if r.Host != "" && (origin == "http://"+r.Host || origin == "https://"+r.Host) {
				return true
			}
			return false
		},
	}
}

// NewHandler upgrades the HTTP connection to a WebSocket, authenticates via
// session cookie, registers the client with the Hub, and starts the read/write pumps.
//
// @Summary      WebSocket connection
// @Description  Upgrade to a WebSocket connection for real-time event streaming. Authenticates via session_token cookie. Sends JSON-encoded events for entity changes (test cases, runs, backups, etc.).
// @Tags         websocket
// @Success      101  {string}  string  "Switching Protocols"
// @Failure      401  {object}  map[string]string
// @Router       /ws [get]
// @Security     SessionCookie
func NewHandler(hub *Hub, validateSession SessionValidator, allowedOrigin string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if hub == nil || validateSession == nil {
			slog.ErrorContext(r.Context(), "ws handler misconfigured")
			httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": "websocket unavailable"})
			return
		}

		cookie, err := r.Cookie("session_token")
		if err != nil {
			slog.WarnContext(r.Context(), "ws auth failed: no session cookie", "remote", r.RemoteAddr)
			httpx.JSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
			return
		}
		user, err := validateSession(cookie.Value)
		if err != nil || user == nil {
			slog.WarnContext(r.Context(), "ws auth failed: invalid session", "remote", r.RemoteAddr)
			httpx.JSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired session"})
			return
		}

		up := newUpgrader(allowedOrigin)
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			slog.ErrorContext(r.Context(), "ws upgrade failed", "user", user.Email, "error", err)
			return
		}

		client := newClient(hub, conn, user)
		client.sessionToken = cookie.Value // for periodic session re-validation (F-018)
		client.validate = validateSession
		hub.register <- client

		ack, _ := json.Marshal(map[string]interface{}{
			"type": "connected",
			"data": map[string]string{"client_id": client.ID},
		})
		client.send <- ack

		go client.writePump()
		go client.readPump()
	}
}
