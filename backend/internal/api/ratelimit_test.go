package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	api "ttgo/internal/api"
	"ttgo/pkg/tracker/store"
)

// TestLoginRateLimited verifies the per-IP login throttle (F-042): rapid repeated
// login attempts from one client eventually receive HTTP 429.
func TestLoginRateLimited(t *testing.T) {
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	srv := api.NewServer(s)

	got429 := false
	for i := 0; i < 30; i++ {
		req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"email":"x@y.z","password":"wrong"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		if w.Code == http.StatusTooManyRequests {
			got429 = true
			break
		}
	}
	if !got429 {
		t.Error("expected a 429 Too Many Requests after exceeding the login burst")
	}
}
