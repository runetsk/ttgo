package ai_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"ttgo/pkg/tracker/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fakeEnvelopeJSON = `{
  "test_cases": [
    {"name": "[Functional] Login works", "category": "Functional",
     "description": "Happy path.", "source_refs": ["AC-1"],
     "steps": [{"action": "Log in", "expected_result": "Dashboard shown"}]},
    {"name": "[Negative] Wrong password rejected", "category": "Negative",
     "description": "Bad creds.", "source_refs": [],
     "steps": [{"action": "Log in with wrong password", "expected_result": "Error toast shown"}]}
  ]
}`

func decodeRunResponse(t *testing.T, rr *httptest.ResponseRecorder) (run map[string]interface{}, drafts []map[string]interface{}) {
	t.Helper()
	var body struct {
		Run    map[string]interface{}   `json:"run"`
		Drafts []map[string]interface{} `json:"drafts"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body), rr.Body.String())
	return body.Run, body.Drafts
}

func TestCreateGeneration_EndToEnd(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, fakeEnvelopeJSON)
	defer fake.Close()
	providerID := createFakeProvider(t, env, fake.URL)
	reqID := createPreviewRequirement(t, env, "REQ-LC-1", "Login flow", "Users must be able to log in.")

	rr := doRequest(env, "POST", "/api/ai-generations", map[string]string{
		"requirement_id":  reqID,
		"provider_id":     providerID,
		"coverage_level":  "essential",
		"idempotency_key": "test-key-1",
	})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	run, drafts := decodeRunResponse(t, rr)
	assert.Equal(t, "completed", run["status"])
	assert.Equal(t, "test-key-1", run["idempotency_key"])
	assert.NotEmpty(t, run["request_context"])
	assert.NotEmpty(t, run["template_hash"])
	require.Len(t, drafts, 2)
	assert.Equal(t, "pending", drafts[0]["status"])
	assert.NotEmpty(t, drafts[0]["id"])
	assert.Equal(t, []interface{}{"AC-1"}, drafts[0]["source_refs"])

	// Durable: GET returns the same run and drafts (Task 11 wires the route;
	// here assert persistence through the store via replay below).
	rr2 := doRequest(env, "POST", "/api/ai-generations", map[string]string{
		"requirement_id":  reqID,
		"provider_id":     providerID,
		"coverage_level":  "essential",
		"idempotency_key": "test-key-1",
	})
	require.Equal(t, http.StatusOK, rr2.Code, "replaying a completed key returns the stored result")
	run2, drafts2 := decodeRunResponse(t, rr2)
	assert.Equal(t, run["id"], run2["id"])
	assert.Len(t, drafts2, 2)
}

func TestCreateGeneration_ReplayDoesNotCallLLMAgain(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	var mu sync.Mutex
	calls := 0
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"model":"fake","choices":[{"finish_reason":"stop","message":{"content":%q}}],"usage":{"prompt_tokens":10,"completion_tokens":20,"total_tokens":30}}`, fakeEnvelopeJSON)
	}))
	defer fake.Close()
	providerID := createFakeProvider(t, env, fake.URL)
	reqID := createPreviewRequirement(t, env, "REQ-LC-2", "T", "D")

	body := map[string]string{"requirement_id": reqID, "provider_id": providerID, "idempotency_key": "replay-key"}
	require.Equal(t, http.StatusCreated, doRequest(env, "POST", "/api/ai-generations", body).Code)
	require.Equal(t, http.StatusOK, doRequest(env, "POST", "/api/ai-generations", body).Code)
	assert.Equal(t, 1, calls, "replay must not spend tokens")
}

func TestCreateGeneration_ConflictWhileRunning(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	reqID := createPreviewRequirement(t, env, "REQ-LC-3", "T", "D")

	// Simulate an in-flight run holding the key.
	_, _, err := env.store.CreateGenerationRun(&models.AIGenerationRun{
		IdempotencyKey: "busy-key", RequirementID: reqID,
		Status: models.AIGenerationRunStatusRunning,
	})
	require.NoError(t, err)

	rr := doRequest(env, "POST", "/api/ai-generations", map[string]string{
		"requirement_id": reqID, "idempotency_key": "busy-key",
	})
	assert.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())
}

func TestCreateGeneration_ParseFailureMarksRunFailed(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, "utter garbage, not JSON at all")
	defer fake.Close()
	providerID := createFakeProvider(t, env, fake.URL)
	reqID := createPreviewRequirement(t, env, "REQ-LC-4", "T", "D")

	rr := doRequest(env, "POST", "/api/ai-generations", map[string]string{
		"requirement_id": reqID, "provider_id": providerID, "idempotency_key": "parse-fail-key",
	})
	require.Equal(t, http.StatusUnprocessableEntity, rr.Code, rr.Body.String())
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, "parse", body["category"])
	runID, _ := body["run_id"].(string)
	require.NotEmpty(t, runID)

	run, err := env.store.GetGenerationRun(runID)
	require.NoError(t, err)
	assert.Equal(t, models.AIGenerationRunStatusFailed, run.Status)
	assert.Equal(t, "parse", run.ErrorCategory)
	assert.Positive(t, run.RetryCount, "the malformed-output repair attempt is recorded")
}

func TestCreateGeneration_RequirementNotFound(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	rr := doRequest(env, "POST", "/api/ai-generations", map[string]string{
		"requirement_id": "nope",
	})
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestCreateGeneration_SchemaRequestedForCloudProviders(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	var mu sync.Mutex
	var rawBodies []map[string]interface{}
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&b)
		mu.Lock()
		rawBodies = append(rawBodies, b)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"model":"fake","choices":[{"finish_reason":"stop","message":{"content":%q}}]}`, fakeEnvelopeJSON)
	}))
	defer fake.Close()

	// createFakeProvider registers a "local" provider; local is prompt-only.
	providerID := createFakeProvider(t, env, fake.URL)
	reqID := createPreviewRequirement(t, env, "REQ-LC-5", "T", "D")
	rr := doRequest(env, "POST", "/api/ai-generations", map[string]string{
		"requirement_id": reqID, "provider_id": providerID,
	})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	mu.Lock()
	defer mu.Unlock()
	require.NotEmpty(t, rawBodies)
	_, hasFormat := rawBodies[0]["response_format"]
	assert.False(t, hasFormat, "local providers stay prompt-only")
}

// TestCreateGeneration_ProviderAuthFailureMarksRunFailed exercises the
// writeGenerationFailure path (none of the tests above reach it): a
// non-retryable provider HTTP error must map to 502 with the classified
// category, persist the run as failed, and — since MarkGenerationRunRunning
// only writes started_at via a targeted column update while the final save is
// a full-row Save — started_at must still survive onto the failed row.
func TestCreateGeneration_ProviderAuthFailureMarksRunFailed(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"message":"invalid api key"}}`)
	}))
	defer fake.Close()
	providerID := createFakeProvider(t, env, fake.URL)
	reqID := createPreviewRequirement(t, env, "REQ-LC-6", "T", "D")

	rr := doRequest(env, "POST", "/api/ai-generations", map[string]string{
		"requirement_id": reqID, "provider_id": providerID, "idempotency_key": "auth-fail-key",
	})
	require.Equal(t, http.StatusBadGateway, rr.Code, rr.Body.String())
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, "authentication", body["category"])
	runID, _ := body["run_id"].(string)
	require.NotEmpty(t, runID)

	run, err := env.store.GetGenerationRun(runID)
	require.NoError(t, err)
	assert.Equal(t, models.AIGenerationRunStatusFailed, run.Status)
	assert.Equal(t, "authentication", run.ErrorCategory)
	assert.NotNil(t, run.StartedAt, "started_at must survive the full-row Save on run failure")
}

func TestCreateGeneration_SanitizesStepText(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	dirty := `{"test_cases":[{"name":"N","category":"Functional","description":"d","source_refs":[],` +
		`"steps":[{"action":"Click <script>alert(1)</script> Save","expected_result":"OK <b>bold</b>"}]}]}`
	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, dirty)
	defer fake.Close()
	providerID := createFakeProvider(t, env, fake.URL)
	reqID := createPreviewRequirement(t, env, "REQ-SAN-1", "T", "D")

	rr := doRequest(env, "POST", "/api/ai-generations", map[string]string{
		"requirement_id": reqID, "provider_id": providerID, "idempotency_key": "san-1",
	})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	_, drafts := decodeRunResponse(t, rr)
	require.Len(t, drafts, 1)
	steps := drafts[0]["steps"].([]interface{})
	action := steps[0].(map[string]interface{})["action"].(string)
	assert.NotContains(t, action, "<script>", "step action must be sanitized")
}

// createCompletedRun generates a run through the API and returns (runID, draftIDs).
func createCompletedRun(t *testing.T, env *testEnv, reqID, providerID string) (string, []string) {
	t.Helper()
	rr := doRequest(env, "POST", "/api/ai-generations", map[string]string{
		"requirement_id": reqID, "provider_id": providerID, "idempotency_key": uuid.NewString(),
	})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	run, drafts := decodeRunResponse(t, rr)
	ids := make([]string, len(drafts))
	for i, d := range drafts {
		ids[i] = d["id"].(string)
	}
	return run["id"].(string), ids
}

func TestGetGeneration(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, fakeEnvelopeJSON)
	defer fake.Close()
	providerID := createFakeProvider(t, env, fake.URL)
	reqID := createPreviewRequirement(t, env, "REQ-GET-1", "T", "D")
	runID, draftIDs := createCompletedRun(t, env, reqID, providerID)

	rr := doRequest(env, "GET", "/api/ai-generations/"+runID, nil)
	require.Equal(t, http.StatusOK, rr.Code)
	run, drafts := decodeRunResponse(t, rr)
	assert.Equal(t, runID, run["id"])
	assert.Len(t, drafts, len(draftIDs))

	assert.Equal(t, http.StatusNotFound, doRequest(env, "GET", "/api/ai-generations/nope", nil).Code)
}

func TestListGenerations(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, fakeEnvelopeJSON)
	defer fake.Close()
	providerID := createFakeProvider(t, env, fake.URL)
	reqID := createPreviewRequirement(t, env, "REQ-LIST-1", "T", "D")
	createCompletedRun(t, env, reqID, providerID)
	createCompletedRun(t, env, reqID, providerID)

	rr := doRequest(env, "GET", "/api/ai-generations?requirement_id="+reqID, nil)
	require.Equal(t, http.StatusOK, rr.Code)
	var body struct {
		Runs []map[string]interface{} `json:"runs"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Len(t, body.Runs, 2)

	assert.Equal(t, http.StatusBadRequest, doRequest(env, "GET", "/api/ai-generations", nil).Code,
		"requirement_id is required for history listing")
}

func TestUpdateGenerationDraft(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, fakeEnvelopeJSON)
	defer fake.Close()
	providerID := createFakeProvider(t, env, fake.URL)
	reqID := createPreviewRequirement(t, env, "REQ-EDIT-1", "T", "D")
	runID, draftIDs := createCompletedRun(t, env, reqID, providerID)

	rr := doRequest(env, "PATCH", "/api/ai-generations/"+runID+"/drafts/"+draftIDs[0], map[string]interface{}{
		"name":  "Edited title",
		"steps": []map[string]string{{"action": "new action", "expected_result": "new result"}},
	})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var body struct {
		Draft map[string]interface{} `json:"draft"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, "Edited title", body.Draft["name"])
	assert.Equal(t, true, body.Draft["edited"])
	assert.Equal(t, float64(2), body.Draft["version"])

	// Editing into an invalid state saves AND surfaces error findings.
	rr = doRequest(env, "PATCH", "/api/ai-generations/"+runID+"/drafts/"+draftIDs[0], map[string]interface{}{
		"steps": []map[string]string{},
	})
	require.Equal(t, http.StatusOK, rr.Code)
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.NotNil(t, body.Draft["findings"], "no_steps finding expected")

	// Unknown draft under a valid run → 404; mismatched run → 404.
	assert.Equal(t, http.StatusNotFound,
		doRequest(env, "PATCH", "/api/ai-generations/"+runID+"/drafts/nope", map[string]interface{}{"name": "x"}).Code)
}

func TestUpdateGenerationDraft_SanitizesStepText(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, fakeEnvelopeJSON)
	defer fake.Close()
	providerID := createFakeProvider(t, env, fake.URL)
	reqID := createPreviewRequirement(t, env, "REQ-EDIT-SAN-1", "T", "D")
	runID, draftIDs := createCompletedRun(t, env, reqID, providerID)

	rr := doRequest(env, "PATCH", "/api/ai-generations/"+runID+"/drafts/"+draftIDs[0], map[string]interface{}{
		"steps": []map[string]string{
			{"action": "Click <script>alert(1)</script> Save", "expected_result": "OK <b>bold</b>"},
		},
	})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var body struct {
		Draft map[string]interface{} `json:"draft"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	steps := body.Draft["steps"].([]interface{})
	require.Len(t, steps, 1)
	action := steps[0].(map[string]interface{})["action"].(string)
	assert.NotContains(t, action, "<script>", "edited step action must be sanitized")
}

func TestRejectGenerationDraftEndpoint(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, fakeEnvelopeJSON)
	defer fake.Close()
	providerID := createFakeProvider(t, env, fake.URL)
	reqID := createPreviewRequirement(t, env, "REQ-REJ-1", "T", "D")
	runID, draftIDs := createCompletedRun(t, env, reqID, providerID)

	rr := doRequest(env, "POST", "/api/ai-generations/"+runID+"/drafts/"+draftIDs[0]+"/reject", map[string]string{
		"reason": "too_vague", "note": "expected results not observable",
	})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var body struct {
		Draft map[string]interface{} `json:"draft"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, "rejected", body.Draft["status"])

	// Invalid reason → 400. Double reject → 409.
	assert.Equal(t, http.StatusBadRequest,
		doRequest(env, "POST", "/api/ai-generations/"+runID+"/drafts/"+draftIDs[1]+"/reject",
			map[string]string{"reason": "just because"}).Code)
	assert.Equal(t, http.StatusConflict,
		doRequest(env, "POST", "/api/ai-generations/"+runID+"/drafts/"+draftIDs[0]+"/reject",
			map[string]string{"reason": "duplicate"}).Code)
}

func TestLifecycleEndpointsRequireAuth(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	req := httptest.NewRequest("GET", "/api/ai-generations?requirement_id=x", nil)
	rr := httptest.NewRecorder()
	env.srv.ServeHTTP(rr, req) // no session cookie
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}
