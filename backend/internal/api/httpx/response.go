package httpx

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// JSON writes a JSON response body with the provided status code.
func JSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

// Error writes a JSON error response. For 5xx errors, the raw error is logged
// but a generic message is sent to the client to avoid leaking internal details.
func Error(w http.ResponseWriter, status int, err error) {
	if status >= 500 {
		slog.Error("server error", "status", status, "error", err)
		JSON(w, status, map[string]string{"error": "internal server error"})
		return
	}
	JSON(w, status, map[string]string{"error": err.Error()})
}
