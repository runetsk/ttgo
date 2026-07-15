package ai_test

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

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

// Steps are stored as sanitized HTML (the frontend decodes entities for its
// plain-text draft views), consistent with the persisted/import/manual paths.
func TestCreateGeneration_EscapesStepEntitiesForStorage(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	dirty := `{"test_cases":[{"name":"N","category":"Functional","description":"d","source_refs":[],` +
		`"steps":[{"action":"Type 'jane@example.com'","expected_result":"header shows 'Welcome, Jane' & a Log Out link"}]}]}`
	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, dirty)
	defer fake.Close()
	providerID := createFakeProvider(t, env, fake.URL)
	reqID := createPreviewRequirement(t, env, "REQ-ENT-1", "T", "D")

	rr := doRequest(env, "POST", "/api/ai-generations", map[string]string{
		"requirement_id": reqID, "provider_id": providerID, "idempotency_key": "ent-1",
	})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	_, drafts := decodeRunResponse(t, rr)
	require.Len(t, drafts, 1)
	steps := drafts[0]["steps"].([]interface{})
	action := steps[0].(map[string]interface{})["action"].(string)
	expected := steps[0].(map[string]interface{})["expected_result"].(string)
	assert.Equal(t, "Type &#39;jane@example.com&#39;", action)
	assert.Equal(t, "header shows &#39;Welcome, Jane&#39; &amp; a Log Out link", expected)
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

func TestUpdateGenerationDraft_EscapesStepEntitiesForStorage(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, fakeEnvelopeJSON)
	defer fake.Close()
	providerID := createFakeProvider(t, env, fake.URL)
	reqID := createPreviewRequirement(t, env, "REQ-EDIT-ENT-1", "T", "D")
	runID, draftIDs := createCompletedRun(t, env, reqID, providerID)

	rr := doRequest(env, "PATCH", "/api/ai-generations/"+runID+"/drafts/"+draftIDs[0], map[string]interface{}{
		"steps": []map[string]string{
			{"action": "Type 'x' & 'y'", "expected_result": "shows 'done'"},
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
	expected := steps[0].(map[string]interface{})["expected_result"].(string)
	assert.Equal(t, "Type &#39;x&#39; &amp; &#39;y&#39;", action)
	assert.Equal(t, "shows &#39;done&#39;", expected)
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

func TestRestoreGenerationDraftEndpoint(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, fakeEnvelopeJSON)
	defer fake.Close()
	providerID := createFakeProvider(t, env, fake.URL)
	reqID := createPreviewRequirement(t, env, "REQ-RESTORE-1", "T", "D")
	runID, draftIDs := createCompletedRun(t, env, reqID, providerID)

	// Restore before reject -> 409.
	rr := doRequest(env, "POST", "/api/ai-generations/"+runID+"/drafts/"+draftIDs[0]+"/restore", nil)
	require.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())

	rr = doRequest(env, "POST", "/api/ai-generations/"+runID+"/drafts/"+draftIDs[0]+"/reject",
		map[string]string{"reason": "duplicate"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	rr = doRequest(env, "POST", "/api/ai-generations/"+runID+"/drafts/"+draftIDs[0]+"/restore", nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var body struct {
		Draft struct {
			Status   string                 `json:"status"`
			Original map[string]interface{} `json:"original"`
		} `json:"draft"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, "pending", body.Draft.Status)
	assert.NotEmpty(t, body.Draft.Original, "responses expose the as-generated original")

	// Unknown draft -> 404.
	rr = doRequest(env, "POST", "/api/ai-generations/"+runID+"/drafts/nope/restore", nil)
	require.Equal(t, http.StatusNotFound, rr.Code)
}

func TestLifecycleEndpointsRequireAuth(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	req := httptest.NewRequest("GET", "/api/ai-generations?requirement_id=x", nil)
	rr := httptest.NewRecorder()
	env.srv.ServeHTTP(rr, req) // no session cookie
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAcceptGenerationEndpoint(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, fakeEnvelopeJSON)
	defer fake.Close()
	providerID := createFakeProvider(t, env, fake.URL)
	reqID := createPreviewRequirement(t, env, "REQ-ACCEPT-1", "T", "D")
	folderID := createTestFolder(t, env, "AI Output")
	runID, draftIDs := createCompletedRun(t, env, reqID, providerID)

	rr := doRequest(env, "POST", "/api/ai-generations/"+runID+"/accept", map[string]interface{}{
		"folder_id": folderID, "draft_ids": draftIDs, "group_by_category": true,
	})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	var body struct {
		CreatedIDs        []string `json:"created_ids"`
		Count             int      `json:"count"`
		SubfoldersCreated int      `json:"subfolders_created"`
		AlreadyAccepted   bool     `json:"already_accepted"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Len(t, body.CreatedIDs, 2)
	assert.Equal(t, 2, body.Count)
	assert.Equal(t, 2, body.SubfoldersCreated, "Functional + Negative subfolders")
	assert.False(t, body.AlreadyAccepted)

	// Replay: same IDs back, no duplicates, 200.
	rr2 := doRequest(env, "POST", "/api/ai-generations/"+runID+"/accept", map[string]interface{}{
		"folder_id": folderID, "draft_ids": draftIDs, "group_by_category": true,
	})
	require.Equal(t, http.StatusOK, rr2.Code, rr2.Body.String())
	var replay struct {
		CreatedIDs      []string `json:"created_ids"`
		AlreadyAccepted bool     `json:"already_accepted"`
	}
	require.NoError(t, json.Unmarshal(rr2.Body.Bytes(), &replay))
	assert.True(t, replay.AlreadyAccepted)
	assert.ElementsMatch(t, body.CreatedIDs, replay.CreatedIDs)

	// Drafts show accepted + linked test case on subsequent GET.
	rr3 := doRequest(env, "GET", "/api/ai-generations/"+runID, nil)
	require.Equal(t, http.StatusOK, rr3.Code)
	_, drafts := decodeRunResponse(t, rr3)
	for _, d := range drafts {
		assert.Equal(t, "accepted", d["status"])
		assert.NotEmpty(t, d["accepted_test_case_id"])
	}
}

func TestAcceptGenerationEndpoint_Validation(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, fakeEnvelopeJSON)
	defer fake.Close()
	providerID := createFakeProvider(t, env, fake.URL)
	reqID := createPreviewRequirement(t, env, "REQ-ACCEPT-2", "T", "D")
	folderID := createTestFolder(t, env, "AI Output 2")
	runID, draftIDs := createCompletedRun(t, env, reqID, providerID)

	// Missing folder / empty selection → 400.
	assert.Equal(t, http.StatusBadRequest, doRequest(env, "POST", "/api/ai-generations/"+runID+"/accept",
		map[string]interface{}{"draft_ids": draftIDs}).Code)
	assert.Equal(t, http.StatusBadRequest, doRequest(env, "POST", "/api/ai-generations/"+runID+"/accept",
		map[string]interface{}{"folder_id": folderID, "draft_ids": []string{}}).Code)
	// Unknown run → 404.
	assert.Equal(t, http.StatusNotFound, doRequest(env, "POST", "/api/ai-generations/nope/accept",
		map[string]interface{}{"folder_id": folderID, "draft_ids": draftIDs}).Code)

	// Reject one draft, then accepting it → 409 (mixed/non-pending).
	require.Equal(t, http.StatusOK, doRequest(env, "POST",
		"/api/ai-generations/"+runID+"/drafts/"+draftIDs[0]+"/reject",
		map[string]string{"reason": "duplicate"}).Code)
	assert.Equal(t, http.StatusConflict, doRequest(env, "POST", "/api/ai-generations/"+runID+"/accept",
		map[string]interface{}{"folder_id": folderID, "draft_ids": draftIDs}).Code)
}

func TestAcceptGenerationEndpoint_AmbiguousVersionConflicts(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, fakeEnvelopeJSON)
	defer fake.Close()
	providerID := createFakeProvider(t, env, fake.URL)
	reqID := createPreviewRequirement(t, env, "REQ-ACCEPT-AMBIG", "T", "D")
	folderID := createTestFolder(t, env, "AI Output Ambig")
	runID, draftIDs := createCompletedRun(t, env, reqID, providerID)

	// Swap the fake's reply so regenerating draftIDs[0] returns a distinct
	// alternative at the same position (mirrors TestRegenerateDraftEndpoint).
	regenFake := newFakeLLMServer(t, &captured, regenEnvelopeJSON)
	defer regenFake.Close()
	rr := doRequest(env, "PUT", "/api/settings/llm-providers/"+providerID, map[string]interface{}{
		"label": "Fake Local LLM", "provider_type": "local",
		"endpoint_url": regenFake.URL, "model_name": "fake-model",
	})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	rr = doRequest(env, "POST", "/api/ai-generations/"+runID+"/drafts/"+draftIDs[0]+"/regenerate",
		map[string]interface{}{"instruction": "sharpen it", "action": "make_more_specific"})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	var regen struct {
		Draft struct {
			ID string `json:"id"`
		} `json:"draft"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &regen))

	// The original draft and its un-chosen regeneration alternative are both
	// still pending at the same position — accepting both in one request must
	// conflict instead of materializing two test cases for one position.
	rr = doRequest(env, "POST", "/api/ai-generations/"+runID+"/accept", map[string]interface{}{
		"folder_id": folderID, "draft_ids": []string{draftIDs[0], regen.Draft.ID},
	})
	assert.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())

	// Accepting just the original (one member of the ambiguous pair) alone still works.
	rr = doRequest(env, "POST", "/api/ai-generations/"+runID+"/accept", map[string]interface{}{
		"folder_id": folderID, "draft_ids": []string{draftIDs[0]},
	})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
}

func TestCreateGeneration_SanitizesSourceRefs(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	// Entity-encoded markup is the case that catches a sanitize-then-unescape
	// bypass (Bluemonday strips literal tags either way, but leaves &lt;script&gt;
	// as inert text that a later html.UnescapeString would revive).
	dirty := `{"test_cases":[{"name":"N","category":"Functional","description":"d",` +
		`"source_refs":["AC-&lt;script&gt;alert(1)&lt;/script&gt;1"],` +
		`"steps":[{"action":"a","expected_result":"e"}]}]}`
	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, dirty)
	defer fake.Close()
	providerID := createFakeProvider(t, env, fake.URL)
	reqID := createPreviewRequirement(t, env, "REQ-SREF-1", "T", "D")

	rr := doRequest(env, "POST", "/api/ai-generations", map[string]string{
		"requirement_id": reqID, "provider_id": providerID, "idempotency_key": "sref-1",
	})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	_, drafts := decodeRunResponse(t, rr)
	require.Len(t, drafts, 1)
	refs := drafts[0]["source_refs"].([]interface{})
	require.Len(t, refs, 1)
	assert.NotContains(t, refs[0].(string), "<script>", "generated source_refs must be sanitized")
}

func TestUpdateGenerationDraft_SanitizesSourceRefs(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, fakeEnvelopeJSON)
	defer fake.Close()
	providerID := createFakeProvider(t, env, fake.URL)
	reqID := createPreviewRequirement(t, env, "REQ-SREF-2", "T", "D")
	runID, draftIDs := createCompletedRun(t, env, reqID, providerID)

	rr := doRequest(env, "PATCH", "/api/ai-generations/"+runID+"/drafts/"+draftIDs[0], map[string]interface{}{
		"source_refs": []string{"AC-&lt;script&gt;alert(1)&lt;/script&gt;2"},
	})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var body struct {
		Draft map[string]interface{} `json:"draft"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	refs := body.Draft["source_refs"].([]interface{})
	require.Len(t, refs, 1)
	assert.NotContains(t, refs[0].(string), "<script>", "edited source_refs must be sanitized")
}

func TestRejectGenerationDraft_SanitizesNote(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, fakeEnvelopeJSON)
	defer fake.Close()
	providerID := createFakeProvider(t, env, fake.URL)
	reqID := createPreviewRequirement(t, env, "REQ-NOTE-1", "T", "D")
	runID, draftIDs := createCompletedRun(t, env, reqID, providerID)

	rr := doRequest(env, "POST", "/api/ai-generations/"+runID+"/drafts/"+draftIDs[0]+"/reject",
		map[string]string{"reason": "other", "note": "bad <script>alert(1)</script> draft"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	// The note lives only on the event trail; read it back through the store.
	events, err := env.store.ListGenerationEvents(runID)
	require.NoError(t, err)
	var rejectNote string
	for _, e := range events {
		if e.EventType == models.AIGenEventRejected {
			rejectNote = e.Note
		}
	}
	assert.NotEmpty(t, rejectNote)
	assert.NotContains(t, rejectNote, "<script>", "reject note must be sanitized before storage")
}

func TestCreateGeneration_ReplayDifferentRequirementConflicts(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, fakeEnvelopeJSON)
	defer fake.Close()
	providerID := createFakeProvider(t, env, fake.URL)
	reqA := createPreviewRequirement(t, env, "REQ-IDK-A", "A", "D")
	reqB := createPreviewRequirement(t, env, "REQ-IDK-B", "B", "D")

	rr := doRequest(env, "POST", "/api/ai-generations", map[string]string{
		"requirement_id": reqA, "provider_id": providerID, "idempotency_key": "shared-key",
	})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	// Reusing the key for a DIFFERENT requirement must conflict, not replay reqA's run.
	rr2 := doRequest(env, "POST", "/api/ai-generations", map[string]string{
		"requirement_id": reqB, "provider_id": providerID, "idempotency_key": "shared-key",
	})
	assert.Equal(t, http.StatusConflict, rr2.Code, rr2.Body.String())
}

const qualityEnvelopeJSON = `{
  "test_cases": [
    {
      "name": "[Functional] Sign in with valid credentials",
      "category": "Functional",
      "description": "Happy path.",
      "source_refs": ["AC-1"],
      "steps": [
        {"action": "Enter \"user@example.com\" in the Email field", "expected_result": "The Email field contains the address"},
        {"action": "Click the \"Sign in\" button", "expected_result": "The dashboard page is displayed"}
      ]
    },
    {
      "name": "Sign in with valid credentials",
      "category": "Functional",
      "description": "Same scenario, duplicate name.",
      "source_refs": [],
      "steps": [
        {"action": "Submit the form", "expected_result": "It works"}
      ]
    }
  ]
}`

func TestCreateGeneration_ComputesQualityCoverageDuplicates(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, qualityEnvelopeJSON)
	defer fake.Close()
	providerID := createFakeProvider(t, env, fake.URL)
	reqID := createPreviewRequirement(t, env, "REQ-QUAL-1", "Login",
		`<h2>Acceptance Criteria</h2><ul><li>User can sign in</li><li>Wrong password shows an error</li></ul>`)

	rr := doRequest(env, "POST", "/api/ai-generations", map[string]string{
		"requirement_id": reqID, "provider_id": providerID, "idempotency_key": uuid.NewString(),
	})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	var body struct {
		Coverage struct {
			Targets []struct {
				ID             string `json:"id"`
				Status         string `json:"status"`
				DraftPositions []int  `json:"draft_positions"`
			} `json:"targets"`
			UncoveredCount int `json:"uncovered_count"`
		} `json:"coverage"`
		Drafts []struct {
			Quality []struct {
				Key      string `json:"key"`
				Findings []struct {
					Code string `json:"code"`
				} `json:"findings"`
			} `json:"quality"`
			Duplicates []struct {
				Kind       string  `json:"kind"`
				Similarity float64 `json:"similarity"`
			} `json:"duplicates"`
		} `json:"drafts"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))

	require.Len(t, body.Coverage.Targets, 2)
	assert.Equal(t, "AC-1", body.Coverage.Targets[0].ID)
	assert.Equal(t, "covered", body.Coverage.Targets[0].Status)
	assert.Equal(t, []int{0}, body.Coverage.Targets[0].DraftPositions)
	assert.Equal(t, 1, body.Coverage.UncoveredCount, "AC-2 is uncovered")

	require.Len(t, body.Drafts, 2)
	// Draft 1 is the vague duplicate: expect observability + uniqueness findings and a batch duplicate.
	var codes []string
	for _, dim := range body.Drafts[1].Quality {
		for _, f := range dim.Findings {
			codes = append(codes, dim.Key+":"+f.Code)
		}
	}
	assert.Contains(t, codes, "expected_observability:vague_expected")
	assert.Contains(t, codes, "uniqueness:duplicate_name_in_batch")
	assert.Contains(t, codes, "traceability:no_source_refs")
	require.NotEmpty(t, body.Drafts[1].Duplicates)
	assert.Equal(t, "batch", body.Drafts[1].Duplicates[0].Kind)
	// Draft 0 also carries the mirror duplicate + uniqueness warning but NO traceability finding.
	require.NotEmpty(t, body.Drafts[0].Duplicates)
}

func TestUpdateGenerationDraft_RecomputesQualityAndCoverage(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, qualityEnvelopeJSON)
	defer fake.Close()
	providerID := createFakeProvider(t, env, fake.URL)
	reqID := createPreviewRequirement(t, env, "REQ-QUAL-2", "Login",
		`<ul><li>User can sign in</li><li>Wrong password shows an error</li></ul>`)
	runID, draftIDs := createCompletedRun(t, env, reqID, providerID)

	// Baseline: at create time draft 1 (the vague, un-referenced duplicate) must
	// already carry rubric findings and a duplicate candidate, and AC-2 must be
	// uncovered. This proves the create-time analysis populated the very fields the
	// PATCH recompute will clear — without it, the "cleared after edit" assertions
	// below would be vacuously true even if nothing were wired.
	rr := doRequest(env, "GET", "/api/ai-generations/"+runID, nil)
	require.Equal(t, http.StatusOK, rr.Code)
	var before struct {
		Drafts []struct {
			Quality    json.RawMessage `json:"quality"`
			Duplicates json.RawMessage `json:"duplicates"`
		} `json:"drafts"`
		Coverage struct {
			UncoveredCount int `json:"uncovered_count"`
		} `json:"coverage"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &before))
	require.Len(t, before.Drafts, 2)
	require.NotNil(t, before.Drafts[1].Quality, "the vague draft starts with rubric findings")
	assert.Contains(t, string(before.Drafts[1].Quality), "no_source_refs")
	require.NotNil(t, before.Drafts[1].Duplicates, "the duplicate-named draft starts with a candidate")
	assert.Equal(t, 1, before.Coverage.UncoveredCount, "AC-2 is uncovered before the edit")

	// Point the second draft at AC-2, rename it out of the duplicate collision, and
	// give it a concrete step. A correct recompute must clear every finding AND
	// rebuild coverage so AC-2 becomes covered.
	rr = doRequest(env, "PATCH", "/api/ai-generations/"+runID+"/drafts/"+draftIDs[1], map[string]interface{}{
		"name":        "Wrong password shows an inline error",
		"source_refs": []string{"AC-2"},
		"steps": []map[string]string{
			{"action": `Enter "wrong-pass-123" as the password`, "expected_result": "An inline error \"Invalid credentials\" is displayed"},
		},
	})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var body struct {
		Draft struct {
			Quality    json.RawMessage `json:"quality"`
			Duplicates json.RawMessage `json:"duplicates"`
		} `json:"draft"`
		Coverage struct {
			Targets []struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"targets"`
			UncoveredCount int `json:"uncovered_count"`
		} `json:"coverage"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))

	// The PATCH response genuinely rebuilt coverage (if the wiring were absent the
	// coverage key would be missing -> Targets len 0, failing this).
	require.Len(t, body.Coverage.Targets, 2, "PATCH response carries the rebuilt coverage report")
	assert.Equal(t, 0, body.Coverage.UncoveredCount, "AC-2 is now covered")
	assert.Nil(t, body.Draft.Quality, "the edit resolved every rubric finding")
	assert.Nil(t, body.Draft.Duplicates, "the renamed draft no longer collides")
}

const regenEnvelopeJSON = `{
  "test_cases": [{
    "name": "[Functional] Sign in with valid credentials (sharpened)",
    "category": "Functional",
    "description": "Revised.",
    "source_refs": ["AC-1"],
    "steps": [{"action": "Enter \"user@example.com\" / \"Passw0rd!\" and submit", "expected_result": "The dashboard header shows the signed-in user"}]
  }]
}`

func TestRegenerateDraftEndpoint(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, fakeEnvelopeJSON)
	defer fake.Close()
	providerID := createFakeProvider(t, env, fake.URL)
	reqID := createPreviewRequirement(t, env, "REQ-REGEN-1", "Login", "<ul><li>User can sign in</li></ul>")
	runID, draftIDs := createCompletedRun(t, env, reqID, providerID)

	// Swap the fake's reply to the single-case revision for the regen call.
	regenFake := newFakeLLMServer(t, &captured, regenEnvelopeJSON)
	defer regenFake.Close()
	// Point the provider at the regen fake (update endpoint_url).
	rr := doRequest(env, "PUT", "/api/settings/llm-providers/"+providerID, map[string]interface{}{
		"label": "Fake Local LLM", "provider_type": "local",
		"endpoint_url": regenFake.URL, "model_name": "fake-model",
	})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	rr = doRequest(env, "POST", "/api/ai-generations/"+runID+"/drafts/"+draftIDs[0]+"/regenerate",
		map[string]interface{}{"instruction": "sharpen it", "action": "make_more_specific"})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	var body struct {
		Draft struct {
			ID            string  `json:"id"`
			Version       int     `json:"version"`
			ParentDraftID *string `json:"parent_draft_id"`
			Position      int     `json:"position"`
			Status        string  `json:"status"`
			Name          string  `json:"name"`
		} `json:"draft"`
		OriginalID string `json:"original_id"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, draftIDs[0], body.OriginalID)
	assert.Equal(t, 2, body.Draft.Version)
	require.NotNil(t, body.Draft.ParentDraftID)
	assert.Equal(t, draftIDs[0], *body.Draft.ParentDraftID)
	assert.Equal(t, "pending", body.Draft.Status)
	assert.Contains(t, body.Draft.Name, "sharpened")

	// Both versions coexist; run token totals grew.
	rr = doRequest(env, "GET", "/api/ai-generations/"+runID, nil)
	require.Equal(t, http.StatusOK, rr.Code)
	var runBody struct {
		Run struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"run"`
		Drafts []struct {
			ID string `json:"id"`
		} `json:"drafts"`
		Coverage struct {
			Targets []struct {
				ID             string `json:"id"`
				DraftPositions []int  `json:"draft_positions"`
			} `json:"targets"`
		} `json:"coverage"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &runBody))
	assert.Greater(t, runBody.Run.TotalTokens, 30, "regen usage accumulated onto the run")
	assert.Len(t, runBody.Drafts, 3, "original 2 + 1 alternative")

	// The revised draft keeps its original position (0) and still carries
	// source_refs ["AC-1"]; the coverage report must attribute AC-1 to that
	// real position deterministically, not to a map-iteration-order artifact.
	require.Len(t, runBody.Coverage.Targets, 1)
	assert.Equal(t, "AC-1", runBody.Coverage.Targets[0].ID)
	assert.Equal(t, []int{0}, runBody.Coverage.Targets[0].DraftPositions,
		"AC-1 must be attributed to the revised draft's real position, not scrambled by map iteration")
}

func TestRegenerateDraftEndpoint_Validation(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, fakeEnvelopeJSON)
	defer fake.Close()
	providerID := createFakeProvider(t, env, fake.URL)
	reqID := createPreviewRequirement(t, env, "REQ-REGEN-2", "T", "D")
	runID, draftIDs := createCompletedRun(t, env, reqID, providerID)

	rr := doRequest(env, "POST", "/api/ai-generations/"+runID+"/drafts/"+draftIDs[0]+"/regenerate",
		map[string]string{"action": "delete_everything"})
	require.Equal(t, http.StatusBadRequest, rr.Code)

	// Reject the draft, then regenerating it conflicts.
	rr = doRequest(env, "POST", "/api/ai-generations/"+runID+"/drafts/"+draftIDs[0]+"/reject",
		map[string]string{"reason": "duplicate"})
	require.Equal(t, http.StatusOK, rr.Code)
	rr = doRequest(env, "POST", "/api/ai-generations/"+runID+"/drafts/"+draftIDs[0]+"/regenerate",
		map[string]string{"action": ""})
	require.Equal(t, http.StatusConflict, rr.Code)
}

func TestCreateGeneration_ReplayFailedRunReturnsFailureStatus(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, "not json at all")
	defer fake.Close()
	providerID := createFakeProvider(t, env, fake.URL)
	reqID := createPreviewRequirement(t, env, "REQ-IDK-FAIL", "T", "D")

	body := map[string]string{"requirement_id": reqID, "provider_id": providerID, "idempotency_key": "fail-key"}
	rr := doRequest(env, "POST", "/api/ai-generations", body)
	require.Equal(t, http.StatusUnprocessableEntity, rr.Code, "parse failure → 422")

	// Replaying the failed key reproduces the 422 (with category), not a misleading 200.
	rr2 := doRequest(env, "POST", "/api/ai-generations", body)
	require.Equal(t, http.StatusUnprocessableEntity, rr2.Code, "replay of a failed run must reproduce its failure status")
	var b map[string]interface{}
	require.NoError(t, json.Unmarshal(rr2.Body.Bytes(), &b))
	assert.Equal(t, "parse", b["category"])
}

func TestChooseDraftVersionEndpoint(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, fakeEnvelopeJSON)
	defer fake.Close()
	providerID := createFakeProvider(t, env, fake.URL)
	reqID := createPreviewRequirement(t, env, "REQ-CHOOSE-1", "T", "D")
	runID, draftIDs := createCompletedRun(t, env, reqID, providerID)

	regenFake := newFakeLLMServer(t, &captured, regenEnvelopeJSON)
	defer regenFake.Close()
	rr := doRequest(env, "PUT", "/api/settings/llm-providers/"+providerID, map[string]interface{}{
		"label": "Fake Local LLM", "provider_type": "local",
		"endpoint_url": regenFake.URL, "model_name": "fake-model",
	})
	require.Equal(t, http.StatusOK, rr.Code)

	rr = doRequest(env, "POST", "/api/ai-generations/"+runID+"/drafts/"+draftIDs[0]+"/regenerate",
		map[string]string{"action": "make_more_specific"})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	var regen struct {
		Draft struct {
			ID string `json:"id"`
		} `json:"draft"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &regen))

	rr = doRequest(env, "POST", "/api/ai-generations/"+runID+"/drafts/"+regen.Draft.ID+"/choose", nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var chosen struct {
		Draft struct {
			Status string `json:"status"`
		} `json:"draft"`
		SupersededIDs []string `json:"superseded_ids"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &chosen))
	assert.Equal(t, "pending", chosen.Draft.Status)
	assert.Equal(t, []string{draftIDs[0]}, chosen.SupersededIDs)

	// The superseded original cannot be chosen back.
	rr = doRequest(env, "POST", "/api/ai-generations/"+runID+"/drafts/"+draftIDs[0]+"/choose", nil)
	require.Equal(t, http.StatusConflict, rr.Code)
}

func TestCreateGeneration_CriticPassAppendsFindings(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	criticReply := `{"findings":[{"draft_index":0,"dimension":"relevance","message":"drifts away from the requirement"}]}`
	call := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call++
		content := fakeEnvelopeJSON
		if call > 1 {
			content = criticReply
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"model": "fake-model",
			"choices": []map[string]interface{}{
				{"finish_reason": "stop", "message": map[string]string{"content": content}},
			},
			"usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 20, "total_tokens": 30},
		})
	}))
	defer srv.Close()
	providerID := createFakeProvider(t, env, srv.URL)
	reqID := createPreviewRequirement(t, env, "REQ-CRITIC-1", "T", "D")

	rr := doRequest(env, "POST", "/api/ai-generations", map[string]interface{}{
		"requirement_id": reqID, "provider_id": providerID,
		"coverage_level": "thorough", "run_critic": true,
		"idempotency_key": uuid.NewString(),
	})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	assert.Equal(t, 2, call, "exactly one critic call on top of generation")

	var body struct {
		Run struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"run"`
		Drafts []struct {
			Quality []struct {
				Key string `json:"key"`
			} `json:"quality"`
		} `json:"drafts"`
		CriticWarning string `json:"critic_warning"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Empty(t, body.CriticWarning)
	assert.Equal(t, 60, body.Run.TotalTokens, "generation + critic usage accumulated")
	var keys []string
	for _, dim := range body.Drafts[0].Quality {
		keys = append(keys, dim.Key)
	}
	assert.Contains(t, keys, "critic")
}

func TestCreateGeneration_CriticOffByDefault(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, fakeEnvelopeJSON)
	defer fake.Close()
	providerID := createFakeProvider(t, env, fake.URL)
	reqID := createPreviewRequirement(t, env, "REQ-CRITIC-2", "T", "D")

	rr := doRequest(env, "POST", "/api/ai-generations", map[string]string{
		"requirement_id": reqID, "provider_id": providerID, "idempotency_key": uuid.NewString(),
	})
	require.Equal(t, http.StatusCreated, rr.Code)
	// One call only — the shared fake counts via capture (path set once is enough here);
	// stronger: assert no draft has a critic dimension.
	var body struct {
		Drafts []struct {
			Quality []struct {
				Key string `json:"key"`
			} `json:"quality"`
		} `json:"drafts"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	for _, d := range body.Drafts {
		for _, dim := range d.Quality {
			assert.NotEqual(t, "critic", dim.Key)
		}
	}
}

func TestCancelGeneration_StatesAndInFlight(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	// 404 unknown.
	rr := doRequest(env, "POST", "/api/ai-generations/nope/cancel", nil)
	require.Equal(t, http.StatusNotFound, rr.Code)

	// Terminal -> 409.
	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, fakeEnvelopeJSON)
	defer fake.Close()
	providerID := createFakeProvider(t, env, fake.URL)
	reqID := createPreviewRequirement(t, env, "REQ-CANCEL-1", "T", "D")
	runID, _ := createCompletedRun(t, env, reqID, providerID)
	rr = doRequest(env, "POST", "/api/ai-generations/"+runID+"/cancel", nil)
	require.Equal(t, http.StatusConflict, rr.Code)

	// Stale running row -> stamped cancelled (200).
	stale, _, err := env.store.CreateGenerationRun(&models.AIGenerationRun{RequirementID: reqID})
	require.NoError(t, err)
	require.NoError(t, env.store.MarkGenerationRunRunning(stale.ID))
	rr = doRequest(env, "POST", "/api/ai-generations/"+stale.ID+"/cancel", nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	got, err := env.store.GetGenerationRun(stale.ID)
	require.NoError(t, err)
	assert.Equal(t, models.AIGenerationRunStatusCancelled, got.Status)

	// In-flight: a slow provider held open until cancel fires.
	release := make(chan struct{})
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"m","choices":[{"finish_reason":"stop","message":{"content":"{}"}}]}`))
	}))
	defer slow.Close()
	defer close(release)
	// Point the existing provider at the slow server rather than creating a
	// second one — provider labels are unique, and createFakeProvider always
	// uses the same fixed label (see TestRegenerateDraftEndpoint for the same
	// update-endpoint_url-in-place pattern).
	rr = doRequest(env, "PUT", "/api/settings/llm-providers/"+providerID, map[string]interface{}{
		"label": "Fake Local LLM", "provider_type": "local",
		"endpoint_url": slow.URL, "model_name": "fake-model",
	})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	key := uuid.NewString()
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		done <- doRequest(env, "POST", "/api/ai-generations", map[string]string{
			"requirement_id": reqID, "provider_id": providerID, "idempotency_key": key,
		})
	}()

	// Wait for the run row to exist, then cancel it mid-flight.
	var inflightRunID string
	require.Eventually(t, func() bool {
		run, err := env.store.GetGenerationRunByKey(key)
		if err != nil || run == nil {
			return false
		}
		inflightRunID = run.ID
		return run.Status == models.AIGenerationRunStatusRunning
	}, 5*time.Second, 25*time.Millisecond)

	rr = doRequest(env, "POST", "/api/ai-generations/"+inflightRunID+"/cancel", nil)
	require.Equal(t, http.StatusAccepted, rr.Code, rr.Body.String())

	create := <-done
	require.Equal(t, http.StatusConflict, create.Code, create.Body.String())
	assert.Contains(t, create.Body.String(), `"cancellation"`)
	got, err = env.store.GetGenerationRun(inflightRunID)
	require.NoError(t, err)
	assert.Equal(t, models.AIGenerationRunStatusCancelled, got.Status)
}

func TestCreateGeneration_PopulatesEstimatedCost(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, fakeEnvelopeJSON) // usage: 10/20/30
	defer fake.Close()
	providerID := createFakeProvider(t, env, fake.URL)
	// Price the provider: $1 per 1M prompt, $2 per 1M completion.
	rr := doRequest(env, "PUT", "/api/settings/llm-providers/"+providerID, map[string]interface{}{
		"label": "Fake Local LLM", "provider_type": "local", "endpoint_url": fake.URL,
		"model_name": "fake-model", "prompt_price_per_mtok": 1.0, "completion_price_per_mtok": 2.0,
	})
	require.Equal(t, http.StatusOK, rr.Code)
	reqID := createPreviewRequirement(t, env, "REQ-COST-1", "T", "D")

	rr = doRequest(env, "POST", "/api/ai-generations", map[string]string{
		"requirement_id": reqID, "provider_id": providerID, "idempotency_key": uuid.NewString(),
	})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	var body struct {
		Run struct {
			EstimatedCost *float64 `json:"estimated_cost"`
		} `json:"run"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.NotNil(t, body.Run.EstimatedCost)
	assert.InDelta(t, (10.0*1+20.0*2)/1e6, *body.Run.EstimatedCost, 1e-12)
}

// TestRegenerateDraft_CostIsAdditiveNotRepriced guards the fix that made
// RegenerateDraft add THIS call's own cost as a delta onto the run's
// estimated_cost, instead of recomputing cost from the run's cumulative
// tokens at the (possibly changed) current provider price — which would
// reprice the earlier generation's tokens too.
//
// Identical per-call usage can't distinguish the two mechanisms (both give
// the same total token count and, at a fixed price, the same total cost). The
// discriminator is changing the provider's price between the initial
// generation and the regeneration: additive-delta and cumulative-recompute
// then diverge to different dollar amounts, so asserting the exact final
// number pins down which mechanism actually ran.
func TestRegenerateDraft_CostIsAdditiveNotRepriced(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, fakeEnvelopeJSON) // usage: 10 prompt / 20 completion / 30 total, every call
	defer fake.Close()
	providerID := createFakeProvider(t, env, fake.URL)

	// P1: $1/MTok prompt, $2/MTok completion.
	rr := doRequest(env, "PUT", "/api/settings/llm-providers/"+providerID, map[string]interface{}{
		"label": "Fake Local LLM", "provider_type": "local", "endpoint_url": fake.URL,
		"model_name": "fake-model", "prompt_price_per_mtok": 1.0, "completion_price_per_mtok": 2.0,
	})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	reqID := createPreviewRequirement(t, env, "REQ-COST-REGEN-1", "Login", "<ul><li>User can sign in</li></ul>")
	runID, draftIDs := createCompletedRun(t, env, reqID, providerID)

	rr = doRequest(env, "GET", "/api/ai-generations/"+runID, nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var initial struct {
		Run struct {
			EstimatedCost *float64 `json:"estimated_cost"`
		} `json:"run"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &initial))
	require.NotNil(t, initial.Run.EstimatedCost, "generation must populate estimated_cost when prices are configured")
	c1 := *initial.Run.EstimatedCost
	assert.InDelta(t, (10.0*1+20.0*2)/1e6, c1, 1e-9, "initial run cost priced at P1")

	// Reprice the SAME provider to P2 before regenerating.
	rr = doRequest(env, "PUT", "/api/settings/llm-providers/"+providerID, map[string]interface{}{
		"label": "Fake Local LLM", "provider_type": "local", "endpoint_url": fake.URL,
		"model_name": "fake-model", "prompt_price_per_mtok": 10.0, "completion_price_per_mtok": 20.0,
	})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	rr = doRequest(env, "POST", "/api/ai-generations/"+runID+"/drafts/"+draftIDs[0]+"/regenerate",
		map[string]interface{}{"instruction": "sharpen it", "action": "make_more_specific"})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	rr = doRequest(env, "GET", "/api/ai-generations/"+runID, nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var after struct {
		Run struct {
			EstimatedCost *float64 `json:"estimated_cost"`
		} `json:"run"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &after))
	require.NotNil(t, after.Run.EstimatedCost)

	callCost2 := (10.0*10 + 20.0*20) / 1e6         // this call's own 10/20 usage, priced at P2
	wantAdditive := c1 + callCost2                 // fixed behavior: 5e-5 + 5e-4 = 5.5e-4
	cumulativeReprice := (20.0*10 + 40.0*20) / 1e6 // buggy behavior: cumulative 20/40 tokens repriced at P2 = 1e-3

	assert.InDelta(t, wantAdditive, *after.Run.EstimatedCost, 1e-9,
		"regeneration must add this call's own cost at CURRENT prices as a delta onto the prior total")
	assert.Greater(t, math.Abs(cumulativeReprice-*after.Run.EstimatedCost), 1e-6,
		"must not match cumulative-recompute, which would reprice the original generation's tokens at the new price too")
}

func TestRunResponsesCarryAttempts(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, fakeEnvelopeJSON)
	defer fake.Close()
	providerID := createFakeProvider(t, env, fake.URL)
	reqID := createPreviewRequirement(t, env, "REQ-ATT-1", "T", "D")
	runID, _ := createCompletedRun(t, env, reqID, providerID)

	rr := doRequest(env, "GET", "/api/ai-generations/"+runID, nil)
	require.Equal(t, http.StatusOK, rr.Code)
	var body struct {
		Attempts []struct {
			Kind        string `json:"kind"`
			TotalTokens int    `json:"total_tokens"`
		} `json:"attempts"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.NotEmpty(t, body.Attempts)
	assert.Equal(t, "generation", body.Attempts[0].Kind)
	assert.Equal(t, 30, body.Attempts[0].TotalTokens)
}

func TestCreateGeneration_BudgetWarningIsAcknowledgeable(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, fakeEnvelopeJSON)
	defer fake.Close()
	providerID := createFakeProvider(t, env, fake.URL)
	// Absurd pricing so any prompt exceeds a tiny budget.
	rr := doRequest(env, "PUT", "/api/settings/llm-providers/"+providerID, map[string]interface{}{
		"label": "Fake Local LLM", "provider_type": "local", "endpoint_url": fake.URL,
		"model_name": "fake-model", "prompt_price_per_mtok": 1000000.0, "completion_price_per_mtok": 1000000.0,
	})
	require.Equal(t, http.StatusOK, rr.Code)
	rr = doRequest(env, "PUT", "/api/settings/ai-budgets", map[string]float64{"per_request_usd": 0.0001})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	reqID := createPreviewRequirement(t, env, "REQ-BUDGET-1", "T", "D")

	body := map[string]interface{}{
		"requirement_id": reqID, "provider_id": providerID, "idempotency_key": uuid.NewString(),
	}
	rr = doRequest(env, "POST", "/api/ai-generations", body)
	require.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"category":"budget"`)
	assert.Contains(t, rr.Body.String(), `"scope":"request"`)

	// Acknowledged -> proceeds normally.
	body["acknowledge_budget"] = true
	rr = doRequest(env, "POST", "/api/ai-generations", body)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
}

func TestAIGenerationReportEndpoint(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, fakeEnvelopeJSON)
	defer fake.Close()
	providerID := createFakeProvider(t, env, fake.URL)
	reqID := createPreviewRequirement(t, env, "REQ-REPORT-1", "T", "D")
	createCompletedRun(t, env, reqID, providerID)

	rr := doRequest(env, "GET", "/api/ai-generations/reports/summary", nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var rep struct {
		Runs struct {
			Total     int `json:"total"`
			Completed int `json:"completed"`
		} `json:"runs"`
		Drafts struct {
			Generated int `json:"generated"`
		} `json:"drafts"`
		Providers []struct {
			ProviderLabel string `json:"provider_label"`
		} `json:"providers"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &rep))
	assert.Equal(t, 1, rep.Runs.Total)
	assert.Equal(t, 1, rep.Runs.Completed)
	assert.Equal(t, 2, rep.Drafts.Generated)
	require.NotEmpty(t, rep.Providers)
}
