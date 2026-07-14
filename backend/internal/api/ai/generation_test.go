package ai_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeLLMCapture records the last OpenAI-compatible chat request the fake
// provider received.
type fakeLLMCapture struct {
	mu   sync.Mutex
	path string
	body struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		MaxTokens int `json:"max_tokens"`
	}
}

// newFakeLLMServer returns an httptest server that speaks the OpenAI-compatible
// chat-completions wire format and always answers with the given content.
// GenerateTests reaches it via a real "local"-type provider config, so the full
// production path (provider construction, SSRF-guarded client, JSON parsing)
// is exercised — no test seams.
func newFakeLLMServer(t *testing.T, capture *fakeLLMCapture, content string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("fake LLM: read body: %v", err)
		}
		capture.mu.Lock()
		capture.path = r.URL.Path
		if err := json.Unmarshal(raw, &capture.body); err != nil {
			t.Errorf("fake LLM: parse body: %v (raw: %s)", err, raw)
		}
		capture.mu.Unlock()

		resp := map[string]interface{}{
			"model": "fake-model",
			"choices": []map[string]interface{}{
				{"finish_reason": "stop", "message": map[string]string{"content": content}},
			},
			"usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 20, "total_tokens": 30},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

// createFakeProvider registers the fake LLM server as a real "local" provider
// through the admin API and returns its id.
func createFakeProvider(t *testing.T, env *testEnv, endpointURL string) string {
	t.Helper()
	rr := doRequest(env, "POST", "/api/settings/llm-providers", map[string]interface{}{
		"label":           "Fake Local LLM",
		"provider_type":   "local",
		"endpoint_url":    endpointURL,
		"model_name":      "fake-model",
		"timeout_seconds": 10,
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create provider: %d %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		ID string `json:"id"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.ID == "" {
		t.Fatal("no provider id in response")
	}
	return resp.ID
}

const fakeDraftsJSON = `[{"name":"[Functional] Login with valid credentials","category":"Functional","description":"Verify login","steps":[{"action":"Enter valid email","expected_result":"Field accepts input"},{"action":"Click Sign In","expected_result":"Dashboard loads"}]}]`

func TestGenerateTests_EndToEnd(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, fakeDraftsJSON)
	defer fake.Close()

	providerID := createFakeProvider(t, env, fake.URL)
	reqID := createPreviewRequirement(t, env, "REQ-GEN-1", "Login flow", "Users must be able to log in.")

	genParams := map[string]string{
		"provider_id":             providerID,
		"coverage_level":          "essential",
		"detail_level":            "Detailed",
		"additional_instructions": "focus on login",
	}

	// The feature's core promise: the preview IS the prompt. Fetch the preview
	// for the identical parameters first, then assert the LLM received exactly
	// its system message and joined segment texts.
	prevRR := doRequest(env, "POST", "/api/ai-gen/prompt-preview", map[string]string{
		"requirement_id":          reqID,
		"coverage_level":          genParams["coverage_level"],
		"detail_level":            genParams["detail_level"],
		"additional_instructions": genParams["additional_instructions"],
	})
	if prevRR.Code != http.StatusOK {
		t.Fatalf("prompt-preview: %d %s", prevRR.Code, prevRR.Body.String())
	}
	var preview previewResponse
	json.NewDecoder(prevRR.Body).Decode(&preview)
	var joined strings.Builder
	for _, s := range preview.Segments {
		joined.WriteString(s.Text)
	}

	rr := doRequest(env, "POST", fmt.Sprintf("/api/requirements/%s/generate-tests", reqID), genParams)
	if rr.Code != http.StatusOK {
		t.Fatalf("generate-tests: %d %s", rr.Code, rr.Body.String())
	}

	var out struct {
		Drafts []struct {
			TempID   string `json:"temp_id"`
			Name     string `json:"name"`
			Category string `json:"category"`
			Steps    []struct {
				Action         string `json:"action"`
				ExpectedResult string `json:"expected_result"`
			} `json:"steps"`
		} `json:"drafts"`
		Debug struct {
			MaxTokensBudget int    `json:"max_tokens_budget"`
			TemplateType    string `json:"template_type"`
			RequestContext  string `json:"request_context"`
			Retried         bool   `json:"retried"`
		} `json:"debug"`
		TemplateWarning string `json:"template_warning"`
	}
	json.NewDecoder(rr.Body).Decode(&out)

	// Drafts parsed from the canned LLM JSON.
	if len(out.Drafts) != 1 {
		t.Fatalf("drafts: want 1, got %d (%s)", len(out.Drafts), rr.Body.String())
	}
	d := out.Drafts[0]
	if d.Name != "[Functional] Login with valid credentials" || d.Category != "Functional" {
		t.Fatalf("draft fields: %+v", d)
	}
	if len(d.Steps) != 2 || d.Steps[1].ExpectedResult != "Dashboard loads" {
		t.Fatalf("draft steps: %+v", d.Steps)
	}
	if d.TempID == "" {
		t.Fatal("draft temp_id empty")
	}
	if out.Debug.Retried {
		t.Fatal("unexpected retry on clean response")
	}
	if out.TemplateWarning != "" {
		t.Fatalf("unexpected template_warning: %q", out.TemplateWarning)
	}

	// The chat request the provider actually received.
	captured.mu.Lock()
	defer captured.mu.Unlock()
	if captured.path != "/v1/chat/completions" {
		t.Fatalf("LLM path: %q", captured.path)
	}
	if captured.body.Model != "fake-model" {
		t.Fatalf("LLM model: %q", captured.body.Model)
	}
	if captured.body.MaxTokens != 4096 { // essential default budget
		t.Fatalf("LLM max_tokens: %d", captured.body.MaxTokens)
	}
	if out.Debug.MaxTokensBudget != 4096 {
		t.Fatalf("debug max_tokens_budget: %d", out.Debug.MaxTokensBudget)
	}
	if out.Debug.TemplateType != "standard" {
		t.Fatalf("debug template_type: %q", out.Debug.TemplateType)
	}
	if len(captured.body.Messages) != 2 {
		t.Fatalf("LLM messages: want 2, got %d", len(captured.body.Messages))
	}
	sys, user := captured.body.Messages[0], captured.body.Messages[1]
	if sys.Role != "system" || user.Role != "user" {
		t.Fatalf("message roles: %q, %q", sys.Role, user.Role)
	}
	if sys.Content != preview.SystemMessage {
		t.Fatalf("system message sent to LLM differs from preview:\n got: %q\nwant: %q", sys.Content, preview.SystemMessage)
	}
	if user.Content != joined.String() {
		t.Fatalf("prompt sent to LLM differs from joined preview segments:\n got: %q\nwant: %q", user.Content, joined.String())
	}
	if out.Debug.RequestContext != user.Content {
		t.Fatal("debug request_context differs from the prompt actually sent")
	}

	// Audit log records the resolved coverage level and success status.
	activity, err := env.store.GetRecentActivity(time.Time{}, time.Time{}, 30)
	if err != nil {
		t.Fatalf("GetRecentActivity: %v", err)
	}
	found := false
	for _, row := range activity {
		action, _ := row["action"].(string)
		if strings.HasPrefix(action, "ai_generation:requirement:"+reqID+":") {
			found = true
			if !strings.Contains(action, ":provider:"+providerID+":") ||
				!strings.Contains(action, ":status:success:") ||
				!strings.Contains(action, ":coverage:essential:") {
				t.Fatalf("audit action fields: %q", action)
			}
		}
	}
	if !found {
		t.Fatalf("no ai_generation audit row for requirement %s", reqID)
	}
}

func TestGenerateTests_TemplateWarningPropagates(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	// Blank out the stored template content — GenerateTests must fall back to
	// the built-in template and surface the warning.
	if _, err := env.store.UpdateTemplateContent("   "); err != nil {
		t.Fatalf("blank template: %v", err)
	}

	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, fakeDraftsJSON)
	defer fake.Close()
	providerID := createFakeProvider(t, env, fake.URL)
	reqID := createPreviewRequirement(t, env, "REQ-GEN-2", "Signup flow", "Users can sign up.")

	rr := doRequest(env, "POST", fmt.Sprintf("/api/requirements/%s/generate-tests", reqID), map[string]string{
		"provider_id": providerID,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("generate-tests: %d %s", rr.Code, rr.Body.String())
	}
	var out struct {
		TemplateWarning string `json:"template_warning"`
	}
	json.NewDecoder(rr.Body).Decode(&out)
	if out.TemplateWarning != "Using built-in default template (custom template unavailable)" {
		t.Fatalf("template_warning: %q", out.TemplateWarning)
	}
}

func TestGenerateTests_UnknownCoverage(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	reqID := createPreviewRequirement(t, env, "REQ-GEN-3", "Some flow", "Desc.")

	rr := doRequest(env, "POST", fmt.Sprintf("/api/requirements/%s/generate-tests", reqID), map[string]string{
		"coverage_level": "maximal",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "coverage_level must be one of: essential, thorough, comprehensive") {
		t.Fatalf("error body: %s", rr.Body.String())
	}
}

func TestGenerateTests_RequirementNotFound(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	rr := doRequest(env, "POST", "/api/requirements/does-not-exist/generate-tests", map[string]string{})
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: %d %s", rr.Code, rr.Body.String())
	}
}

func TestProviderPricingRoundTrip(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	rr := doRequest(env, "POST", "/api/settings/llm-providers", map[string]interface{}{
		"label": "Priced", "provider_type": "openai", "model_name": "gpt-test",
		"api_key": "sk-test", "prompt_price_per_mtok": 2.5, "completion_price_per_mtok": 10.0,
	})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	var created struct {
		ID                     string   `json:"id"`
		PromptPricePerMTok     *float64 `json:"prompt_price_per_mtok"`
		CompletionPricePerMTok *float64 `json:"completion_price_per_mtok"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &created))
	require.NotNil(t, created.PromptPricePerMTok)
	assert.Equal(t, 2.5, *created.PromptPricePerMTok)

	// Clearing via PUT with nulls.
	rr = doRequest(env, "PUT", "/api/settings/llm-providers/"+created.ID, map[string]interface{}{
		"label": "Priced", "provider_type": "openai", "model_name": "gpt-test",
		"prompt_price_per_mtok": nil, "completion_price_per_mtok": nil,
	})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var updated struct {
		PromptPricePerMTok *float64 `json:"prompt_price_per_mtok"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &updated))
	assert.Nil(t, updated.PromptPricePerMTok)
}
