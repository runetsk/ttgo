package ai_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"ttgo/pkg/tracker/models"

	"golang.org/x/crypto/bcrypt"
)

func decodeTS(t *testing.T, rr interface{ Result() *http.Response }) models.TypeSafeSettingsResponse {
	t.Helper()
	var out models.TypeSafeSettingsResponse
	if err := json.NewDecoder(rr.Result().Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return out
}

func TestTypeSafeSettings_GetDefaults(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	rr := doRequest(env, "GET", "/api/settings/typesafe", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	s := decodeTS(t, rr)
	if s.Enabled || s.Model != models.TypeSafeDefaultModel || s.APIKeyStatus != models.TypeSafeKeyStatusMissing {
		t.Errorf("unexpected defaults: %+v", s)
	}
}

func TestTypeSafeSettings_PutMasksAndPreservesKey(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	rr := doRequest(env, "PUT", "/api/settings/typesafe", map[string]interface{}{"api_key": "ts-live-key-4321", "enabled": true})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	s := decodeTS(t, rr)
	if s.APIKeyMasked != "…4321" || s.APIKeyStatus != models.TypeSafeKeyStatusOK || !s.Enabled {
		t.Errorf("unexpected: %+v", s)
	}
	if strings.Contains(rr.Body.String(), "ts-live-key-4321") {
		t.Error("full key must never be serialised")
	}
	rr2 := doRequest(env, "PUT", "/api/settings/typesafe", map[string]interface{}{"api_key": "", "model": "jev-latest"})
	s2 := decodeTS(t, rr2)
	if s2.APIKeyMasked != "…4321" || s2.Model != "jev-latest" {
		t.Errorf("blank key must preserve; got %+v", s2)
	}
	rr3 := doRequest(env, "PUT", "/api/settings/typesafe", map[string]interface{}{"clear_api_key": true})
	if decodeTS(t, rr3).APIKeyStatus != models.TypeSafeKeyStatusMissing {
		t.Error("clear_api_key must remove the key")
	}
}

func TestTypeSafeSettings_PutValidation(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	for name, body := range map[string]map[string]interface{}{
		"key and clear": {"api_key": "x", "clear_api_key": true},
		"empty model":   {"model": ""},
		"timeout low":   {"timeout_seconds": 1},
		"timeout high":  {"timeout_seconds": 999},
		"escalate low":  {"escalate_below_pct": -1},
		"escalate high": {"escalate_below_pct": 101},
	} {
		rr := doRequest(env, "PUT", "/api/settings/typesafe", body)
		if rr.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: expected 422, got %d: %s", name, rr.Code, rr.Body.String())
		}
	}
}

func TestTypeSafeSettings_TestConnectionWithoutKey(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	rr := doRequest(env, "POST", "/api/settings/typesafe/test", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var out map[string]interface{}
	_ = json.NewDecoder(rr.Body).Decode(&out)
	if out["ok"] != false || out["category"] != "auth" {
		t.Errorf("expected ok:false category:auth, got %v", out)
	}
}

// Authorization matrix: any authenticated user may read the masked settings; only admins may
// change them or run the connection test. Mirrors backups/handlers_test.go's member session.
func TestTypeSafeSettings_AuthorizationMatrix(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	hash, err := bcrypt.GenerateFromPassword([]byte("memberpassword1"), 12)
	if err != nil {
		t.Fatal(err)
	}
	user, err := env.store.CreateUser("member@example.com", "Member", string(hash), "member")
	if err != nil {
		t.Fatal(err)
	}
	sess, err := env.store.CreateSession(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	asMember := func(method, path string, body interface{}) *httptest.ResponseRecorder {
		var buf bytes.Buffer
		if body != nil {
			_ = json.NewEncoder(&buf).Encode(body)
		}
		req := httptest.NewRequest(method, path, &buf)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "session_token", Value: sess.ID})
		rr := httptest.NewRecorder()
		env.srv.ServeHTTP(rr, req)
		return rr
	}
	if rr := asMember("GET", "/api/settings/typesafe", nil); rr.Code != http.StatusOK {
		t.Errorf("member GET: expected 200, got %d", rr.Code)
	}
	if rr := asMember("PUT", "/api/settings/typesafe", map[string]interface{}{"enabled": true}); rr.Code != http.StatusForbidden {
		t.Errorf("member PUT: expected 403, got %d", rr.Code)
	}
	if rr := asMember("POST", "/api/settings/typesafe/test", nil); rr.Code != http.StatusForbidden {
		t.Errorf("member test: expected 403, got %d", rr.Code)
	}
	unauth := httptest.NewRequest("GET", "/api/settings/typesafe", nil)
	w := httptest.NewRecorder()
	env.srv.ServeHTTP(w, unauth)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("anonymous GET: expected 401, got %d", w.Code)
	}
}
