package ai_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

// countingLLM is an OpenAI-compatible fake provider that counts the calls it receives and
// always answers with content. Each test uses its own server, so the LLM-path rate limit
// (burst 8 per client) is never reached.
func countingLLM(t *testing.T, content string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"model": "fake-model",
			"choices": []map[string]interface{}{
				{"finish_reason": "stop", "message": map[string]string{"content": content}},
			},
			"usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 20, "total_tokens": 30},
		})
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func switchAIOff(t *testing.T, env *testEnv) {
	t.Helper()
	rr := doRequest(env, "PUT", "/api/settings/ai-features", map[string]interface{}{"enabled": false})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
}

func requireAIOff409(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	require.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())
	require.JSONEq(t, `{"error":"AI features are switched off"}`, rr.Body.String())
}

func TestAISwitchOff_GenerationEndpointsAre409(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	fake, calls := countingLLM(t, fakeEnvelopeJSON)
	providerID := createFakeProvider(t, env, fake.URL)
	reqID := createPreviewRequirement(t, env, "REQ-OFF-1", "Login", "Users must be able to sign in.")
	switchAIOff(t, env)

	requireAIOff409(t, doRequest(env, "POST", "/api/ai-generations", map[string]string{
		"requirement_id": reqID, "provider_id": providerID, "idempotency_key": "off-1",
	}))
	requireAIOff409(t, doRequest(env, "POST", "/api/requirements/"+reqID+"/generate-tests",
		map[string]string{"provider_id": providerID}))
	require.Zero(t, calls.Load(), "no LLM call while AI is off")
}

func TestAISwitchOff_RegenerateIs409(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	fake, calls := countingLLM(t, fakeEnvelopeJSON)
	providerID := createFakeProvider(t, env, fake.URL)
	reqID := createPreviewRequirement(t, env, "REQ-OFF-2", "Login", "Users must be able to sign in.")
	runID, draftIDs := createCompletedRun(t, env, reqID, providerID) // AI still on
	before := calls.Load()
	switchAIOff(t, env)

	requireAIOff409(t, doRequest(env, "POST", "/api/ai-generations/"+runID+"/drafts/"+draftIDs[0]+"/regenerate",
		map[string]interface{}{"instruction": "sharpen it", "action": "make_more_specific"}))
	require.Equal(t, before, calls.Load(), "no LLM call while AI is off")
}

// Admins configure and test connections before switching AI on, and the prompt preview
// makes no call, so none of them are gated.
func TestAISwitchOff_ConnectionTestsAndPreviewStillWork(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	fake, calls := countingLLM(t, `{"ok":true}`)
	providerID := createFakeProvider(t, env, fake.URL)
	switchAIOff(t, env)

	rr := doRequest(env, "POST", "/api/settings/llm-providers/"+providerID+"/test", nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.Contains(t, rr.Body.String(), `"success":true`)
	require.Equal(t, int32(1), calls.Load())

	rr = doRequest(env, "POST", "/api/settings/typesafe/test", nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.Contains(t, rr.Body.String(), "no API key stored")

	rr = doRequest(env, "POST", "/api/ai-gen/prompt-preview", map[string]string{"coverage_level": "essential"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
}

// Import is not an AI feature: the deterministic parsers always run, and only the LLM
// fallback is skipped, with the same 422 shape as when no provider is configured.
func TestParseImport_AIOffSkipsOnlyTheLLMFallback(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	fake, calls := countingLLM(t, `[]`)
	createFakeProvider(t, env, fake.URL)
	gibberish := models.ParseImportRequest{Content: "random gibberish with no structure at all here"}

	// Control: with AI on, unparseable content reaches the provider.
	doRequest(env, "POST", "/api/import/parse", gibberish)
	require.Equal(t, int32(1), calls.Load(), "the fallback must reach the provider while AI is on")

	switchAIOff(t, env)
	rr := doRequest(env, "POST", "/api/import/parse", gibberish)
	require.Equal(t, http.StatusUnprocessableEntity, rr.Code, rr.Body.String())
	var body map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.True(t, strings.HasSuffix(body["error"], "AI-powered parsing also failed: AI features are switched off"), body["error"])
	require.Equal(t, int32(1), calls.Load(), "no LLM call while AI is off")

	rr = doRequest(env, "POST", "/api/import/parse", models.ParseImportRequest{
		Content: `[{"name":"Login test","steps":[{"action":"Sign in","expected_result":"Signed in"}]}]`,
	})
	require.Equal(t, http.StatusOK, rr.Code, "deterministic parsing still works with AI off: %s", rr.Body.String())
}
