// @title           TTGO API
// @version         1.0
// @description     Test tracking and run management REST API.
//
// @host            localhost:8080
// @BasePath        /api
//
// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                 Enter: "Bearer <your-token>"

package api

import (
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"
	apiai "ttgo/internal/api/ai"
	"ttgo/internal/api/authctx"
	apibackups "ttgo/internal/api/backups"
	"ttgo/internal/api/httpx"
	apiws "ttgo/internal/api/websocket"
	"ttgo/internal/logging"
	"ttgo/internal/ratelimit"
	"ttgo/internal/safehttp"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"

	_ "ttgo/docs" // generated swagger docs

	"github.com/go-pkgz/routegroup"
	"github.com/google/uuid"
	"github.com/microcosm-cc/bluemonday"
)

const (
	webhookWorkerCount      = 5
	webhookQueueCap         = 100
	webhookMaxAttempts      = 3
	webhookTimeout          = 10 * time.Second
	maxRequestBodySize      = 5 << 20  // 5 MB for JSON endpoints
	maxExternalResponseSize = 10 << 20 // 10 MB for external API responses (Jira, Confluence, etc.)
)

// WebhookEvent is the payload dispatched to webhook workers.
type WebhookEvent struct {
	Event      string    `json:"event"`
	RunID      string    `json:"run_id"`
	RunName    string    `json:"run_name"`
	CategoryID *string   `json:"category_id"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

// Server holds dependencies for all HTTP handlers.
type Server struct {
	store         *store.Store
	handler       http.Handler
	Hub           *apiws.Hub // WebSocket hub for real-time event broadcasting
	backups       *apibackups.Manager
	webhookQueue  chan *WebhookEvent
	sanitizer     *bluemonday.Policy // singleton HTML sanitizer
	corsOrigin    string             // allowed CORS origin (from config)
	seedMu        sync.Mutex         // guards concurrent seed/remove operations
	aiHandler     *apiai.Handler     // shared AI handler, also used by failure-analysis worker
	webhookClient *http.Client       // SSRF-guarded client for outbound webhook delivery (F-001)
	loginLimiter  *ratelimit.Limiter // per-IP login throttle (F-042)
	llmLimiter    *ratelimit.Limiter // per-token/IP throttle on LLM-calling endpoints (F-007)
	rateStop      chan struct{}      // stops limiter janitors on shutdown
}

func NewServer(s *store.Store, opts ...func(*Server)) *Server {
	mux := http.NewServeMux()
	hub := apiws.NewHub()
	go hub.Run()

	srv := &Server{
		store:         s,
		Hub:           hub,
		webhookQueue:  make(chan *WebhookEvent, webhookQueueCap),
		sanitizer:     bluemonday.UGCPolicy(),
		corsOrigin:    "http://localhost:5173",
		webhookClient: safehttp.GuardedClient(webhookTimeout),
		loginLimiter:  ratelimit.New(0.5, 10), // ~30/min sustained, burst 10 per IP
		llmLimiter:    ratelimit.New(0.2, 8),  // ~12/min sustained, burst 8 per token/IP
		rateStop:      make(chan struct{}),
	}
	srv.loginLimiter.StartJanitor(5*time.Minute, 15*time.Minute, srv.rateStop)
	srv.llmLimiter.StartJanitor(5*time.Minute, 15*time.Minute, srv.rateStop)
	srv.backups = apibackups.NewManager(s, hub)
	for _, opt := range opts {
		opt(srv)
	}
	srv.mountRoutes(mux)

	// Start webhook worker pool (T014)
	for i := 0; i < webhookWorkerCount; i++ {
		go srv.webhookWorker()
	}

	// Wrap with global middleware
	handler := srv.maxBodyMiddleware(srv.maintenanceCheck(mux))
	handler = srv.rateLimitMiddleware(handler)
	handler = srv.loggingMiddleware(handler)
	handler = srv.corsMiddleware(handler)
	handler = srv.requestIDMiddleware(handler)
	handler = srv.recoveryMiddleware(handler)

	srv.handler = handler
	return srv
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

func (s *Server) Shutdown() {
	if s.webhookQueue != nil {
		close(s.webhookQueue)
	}
	if s.rateStop != nil {
		close(s.rateStop)
	}
}

// rateLimitMiddleware throttles brute-force-prone and operator-cost-bearing POST
// endpoints: login (per IP, F-042) and LLM-calling routes (per token/IP, F-007).
func (s *Server) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			switch {
			case r.URL.Path == "/api/auth/login":
				if !s.loginLimiter.Allow("ip:" + ratelimit.ClientIP(r)) {
					w.Header().Set("Retry-After", "5")
					httpx.JSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many login attempts, slow down"})
					return
				}
			case isLLMPath(r.URL.Path):
				if !s.llmLimiter.Allow(ratelimit.BearerOrIP(r)) {
					w.Header().Set("Retry-After", "5")
					httpx.JSON(w, http.StatusTooManyRequests, map[string]string{"error": "AI request rate limit exceeded, slow down"})
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

// isLLMPath reports whether a POST path triggers an operator-billed LLM call.
func isLLMPath(p string) bool {
	switch {
	case strings.HasSuffix(p, "/generate-tests"),
		strings.HasSuffix(p, "/analyze"),
		strings.HasSuffix(p, "/analyze-failures"),
		p == "/api/import/parse":
		return true
	}
	return strings.Contains(p, "/llm-providers/") && strings.HasSuffix(p, "/test")
}

func (s *Server) mountRoutes(mux *http.ServeMux) {
	group := routegroup.New(mux)
	api := group.Mount("/api")

	mountAPIRoutes(s, api)
	mountSwaggerRoutes(mux)
}

// requireAuth middleware accepts either a valid Bearer token or a valid session cookie.
// Bearer-token-authenticated requests do NOT populate the user context (backward-compat).
// Session-cookie-authenticated requests attach *models.User to the context via contextKeyUser.
func (s *Server) requireAuth(scope string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")

		if strings.HasPrefix(authHeader, "Bearer ") {
			// ── Bearer token path (existing CI/CD behaviour, unchanged) ──
			rawToken := strings.TrimPrefix(authHeader, "Bearer ")
			token, err := s.store.ValidateToken(rawToken)
			if err != nil || token == nil {
				httpx.JSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired token"})
				return
			}
			if scope == "write" && token.Scope != "write" {
				httpx.JSON(w, http.StatusForbidden, map[string]string{"error": "insufficient scope"})
				return
			}
			// Attach the token so Bearer-authenticated writes are attributable (F-024).
			next(w, r.WithContext(authctx.WithToken(r.Context(), token)))
			return
		}

		// ── Session cookie path ──
		cookie, err := r.Cookie("session_token")
		if err != nil {
			httpx.JSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
			return
		}
		user, err := s.store.ValidateSession(cookie.Value)
		if err != nil || user == nil {
			httpx.JSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired session"})
			return
		}

		// Attach user to context for downstream handlers
		ctx := authctx.WithUser(r.Context(), user)
		next(w, r.WithContext(ctx))
	}
}

// requireAdmin middleware requires a valid session cookie belonging to an admin user.
// Bearer tokens are rejected (no user context).
func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Must be authenticated via session (not Bearer)
		s.requireAuth("write", func(w http.ResponseWriter, r *http.Request) {
			user := authctx.UserFromRequest(r)
			if user == nil {
				// Bearer token authenticated — no user context → forbidden
				httpx.JSON(w, http.StatusForbidden, map[string]string{"error": "admin session required"})
				return
			}
			if user.Role != "admin" {
				httpx.JSON(w, http.StatusForbidden, map[string]string{"error": "admin role required"})
				return
			}
			next(w, r)
		})(w, r)
	}
}

// userFromContext extracts the authenticated user from the request context (may be nil for Bearer auth).
func userFromContext(r *http.Request) *models.User {
	return authctx.UserFromRequest(r)
}

// webhookWorker processes WebhookEvents with exponential backoff retry (T055).
func (s *Server) webhookWorker() {
	for event := range s.webhookQueue {
		s.dispatchWebhook(event)
	}
}

func (s *Server) dispatchWebhook(event *WebhookEvent) {
	configs, err := s.store.GetActiveWebhooks(event.Event)
	if err != nil {
		slog.Error("webhook: failed to fetch configs", "error", err)
		return
	}

	payload, err := json.Marshal(map[string]interface{}{
		"event":       event.Event,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"run_id":      event.RunID,
		"run_name":    event.RunName,
		"category_id": event.CategoryID,
		"status":      event.Status,
		"created_at":  event.CreatedAt,
	})
	if err != nil {
		slog.Error("webhook: failed to marshal payload", "error", err)
		return
	}

	for _, cfg := range configs {
		s.dispatchToEndpoint(cfg.ID, event.RunID, cfg.URL, cfg.Secret, payload)
	}
}

func (s *Server) dispatchToEndpoint(webhookID, runID, url, secret string, payload []byte) {
	const baseDelay = time.Second

	// One delivery id for all retries; sign the payload so receivers can verify
	// authenticity and reject replays (F-066).
	deliveryID := uuid.New().String()
	var signature string
	if secret != "" {
		// Sign "<delivery-id>.<payload>" so the signature binds the delivery id —
		// a replay can't be passed off as a fresh delivery by swapping the id (F-066).
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(deliveryID))
		mac.Write([]byte{'.'})
		mac.Write(payload)
		signature = "sha256=" + hex.EncodeToString(mac.Sum(nil))
	}

	for attempt := 1; attempt <= webhookMaxAttempts; attempt++ {
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), webhookTimeout)

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			cancel()
			slog.Error("webhook: bad URL", "url", url, "error", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-TTGO-Delivery", deliveryID)
		if signature != "" {
			req.Header.Set("X-TTGO-Signature", signature)
		}

		resp, err := s.webhookClient.Do(req)
		cancel()
		dur := time.Since(start).Milliseconds()

		logEntry := &store.WebhookDispatchEntry{
			WebhookID:    webhookID,
			RunID:        runID,
			Attempt:      attempt,
			DurationMs:   dur,
			DispatchedAt: time.Now(),
		}

		if err != nil {
			logEntry.Status = "retrying"
			logEntry.ErrorMsg = err.Error()
			_ = s.store.SaveDispatchLog(logEntry)
			slog.Warn("webhook attempt failed", "attempt", attempt, "max_attempts", webhookMaxAttempts, "error", err)
		} else {
			code := resp.StatusCode
			resp.Body.Close()
			logEntry.HTTPCode = &code

			if code >= 200 && code < 300 {
				logEntry.Status = "success"
				_ = s.store.SaveDispatchLog(logEntry)
				slog.Info("webhook delivered", "url", url, "attempt", attempt, "duration_ms", dur)
				return
			}
			logEntry.Status = "retrying"
			logEntry.ErrorMsg = fmt.Sprintf("HTTP %d", code)
			_ = s.store.SaveDispatchLog(logEntry)
			slog.Warn("webhook attempt got non-2xx", "attempt", attempt, "max_attempts", webhookMaxAttempts, "http_code", code)
		}

		if attempt < webhookMaxAttempts {
			// Full-jitter backoff: random(0, min(32s, base * 2^attempt))
			cap := float64(baseDelay) * float64(int(1)<<attempt)
			if cap > float64(32*time.Second) {
				cap = float64(32 * time.Second)
			}
			jitter := time.Duration(rand.Float64() * cap) //nolint:gosec
			time.Sleep(jitter)
		} else {
			logEntry.Status = "failed"
			_ = s.store.SaveDispatchLog(logEntry)
		}
	}
}

// maintenanceCheck middleware returns 503 if the system is in maintenance mode.
// Excluded paths: /api/backups, /api/backup-schedule, /api/maintenance-status.
func (s *Server) maintenanceCheck(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		// Skip maintenance check for backup-related endpoints
		if strings.HasPrefix(path, "/api/backups") ||
			strings.HasPrefix(path, "/api/backup-schedule") ||
			path == "/api/maintenance-status" ||
			strings.HasPrefix(path, "/swagger") {
			next.ServeHTTP(w, r)
			return
		}

		if s.backups != nil && s.backups.IsInMaintenanceMode() {
			httpx.JSON(w, http.StatusServiceUnavailable, map[string]string{
				"error": "System is under maintenance — restore in progress",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// statusRecorder wraps http.ResponseWriter to capture the status code.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

func (sr *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := sr.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response does not implement http.Hijacker")
	}
	return hj.Hijack()
}

func (sr *statusRecorder) Flush() {
	if fl, ok := sr.ResponseWriter.(http.Flusher); ok {
		fl.Flush()
	}
}

func (sr *statusRecorder) Push(target string, opts *http.PushOptions) error {
	if p, ok := sr.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}

func (sr *statusRecorder) ReadFrom(src io.Reader) (int64, error) {
	if rf, ok := sr.ResponseWriter.(io.ReaderFrom); ok {
		return rf.ReadFrom(src)
	}
	return io.Copy(sr.ResponseWriter, src)
}

func (sr *statusRecorder) Unwrap() http.ResponseWriter {
	return sr.ResponseWriter
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		logging.FromContext(r.Context()).Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

// requestIDMiddleware checks X-Request-ID header; if missing, generates a UUID.
// Stores it in context via logging.WithRequestID and sets the response header.
func (s *Server) requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-ID")
		if rid == "" {
			rid = uuid.New().String()
		}
		ctx := logging.WithRequestID(r.Context(), rid)
		w.Header().Set("X-Request-ID", rid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// WithCORSOrigin sets the allowed CORS origin for the server.
func WithCORSOrigin(origin string) func(*Server) {
	return func(s *Server) {
		if origin != "" {
			s.corsOrigin = origin
		}
	}
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == s.corsOrigin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE, PATCH")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// recoveryMiddleware catches panics in downstream handlers and returns a 500 response
// instead of crashing the process.
func (s *Server) recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.ErrorContext(r.Context(), "panic recovered", "panic", rec, "stack", string(debug.Stack()))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// maxBodyMiddleware limits request body size to prevent memory exhaustion.
// Multipart uploads (backup restore) are excluded — they have their own ParseMultipartForm limit.
func (s *Server) maxBodyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
		}
		next.ServeHTTP(w, r)
	})
}
