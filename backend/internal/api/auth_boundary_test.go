package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	api "ttgo/internal/api"
	"ttgo/pkg/tracker/store"
)

// TestWriteBearerCannotReachAdminEndpoints verifies the M2 auth-boundary change:
// a write-scoped Bearer token (the CI credential) is forbidden from token
// administration, custom-field schema, and integration-config writes (F-004/5/14),
// while still being able to perform ordinary content writes (no CI regression).
func TestWriteBearerCannotReachAdminEndpoints(t *testing.T) {
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SeedAdminIfNeeded("admin@test.com", "adminpassword123"); err != nil {
		t.Fatal(err)
	}
	_, raw, err := s.CreateToken("ci", "write", nil)
	if err != nil {
		t.Fatal(err)
	}
	srv := api.NewServer(s)

	do := func(method, path, body string) int {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+raw)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		return w.Code
	}

	// Admin-only now: must be forbidden for a write Bearer token.
	forbidden := []struct {
		method, path, body string
	}{
		{"POST", "/api/tokens", `{"description":"x","scope":"write"}`},
		{"GET", "/api/tokens", ``},
		{"POST", "/api/custom-fields", `{"name":"Sev","type":"TEXT"}`},
		{"PUT", "/api/settings/jira", `{"base_url":"https://x.atlassian.net","email":"a@b.c"}`},
		{"PUT", "/api/settings/confluence", `{"base_url":"https://x.atlassian.net","email":"a@b.c"}`},
	}
	for _, c := range forbidden {
		if code := do(c.method, c.path, c.body); code != http.StatusForbidden {
			t.Errorf("%s %s with write Bearer = %d, want 403", c.method, c.path, code)
		}
	}

	// Content writes must still work for the CI token (regression guard).
	if code := do("POST", "/api/folders", `{"name":"CI Folder"}`); code != http.StatusCreated && code != http.StatusOK {
		t.Errorf("POST /api/folders with write Bearer = %d, want 2xx (content writes must still work)", code)
	}
}
