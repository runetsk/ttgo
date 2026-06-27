package ai_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"ttgo/pkg/tracker/models"
)

func TestAIFeatureSettings_GetReturnsEnabledByDefault(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	rr := doRequest(env, "GET", "/api/settings/ai-features", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var s models.AIFeatureSettings
	if err := json.NewDecoder(rr.Body).Decode(&s); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !s.Enabled {
		t.Error("expected Enabled=true by default")
	}
}

func TestAIFeatureSettings_UpdateTogglesOffAndPersists(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	rr := doRequest(env, "PUT", "/api/settings/ai-features", map[string]interface{}{"enabled": false})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var s models.AIFeatureSettings
	if err := json.NewDecoder(rr.Body).Decode(&s); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if s.Enabled {
		t.Error("expected Enabled=false after update")
	}

	rr2 := doRequest(env, "GET", "/api/settings/ai-features", nil)
	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200 on re-fetch, got %d: %s", rr2.Code, rr2.Body.String())
	}
	var s2 models.AIFeatureSettings
	if err := json.NewDecoder(rr2.Body).Decode(&s2); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if s2.Enabled {
		t.Error("expected persisted Enabled=false")
	}
}

func TestAIFeatureSettings_UpdateRejectsMissingEnabled(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	rr := doRequest(env, "PUT", "/api/settings/ai-features", map[string]interface{}{})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing enabled, got %d: %s", rr.Code, rr.Body.String())
	}
	if body := rr.Body.String(); !strings.Contains(body, "enabled is required") {
		t.Errorf("expected error message 'enabled is required', got: %s", body)
	}
}
