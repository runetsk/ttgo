package ai_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"ttgo/pkg/tracker/models"
)

// ── Settings endpoints ───────────────────────────────────────────────────

func TestFailureAnalysisSettings_GetReturnsDefaults(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	rr := doRequest(env, "GET", "/api/settings/ai-failure-analysis", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var s models.AIFailureAnalysisSettings
	if err := json.NewDecoder(rr.Body).Decode(&s); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if s.MaxAnalysesPerRun < 1 {
		t.Errorf("expected positive MaxAnalysesPerRun, got %d", s.MaxAnalysesPerRun)
	}
	if s.PromptTemplate == "" {
		t.Error("expected non-empty PromptTemplate")
	}
}

func TestFailureAnalysisSettings_UpdateRejectsBadMax(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	body := map[string]interface{}{
		"enabled_on_completion": false,
		"max_analyses_per_run":  0,
		"dedup_enabled":         true,
		"redaction_enabled":     true,
		"prompt_template":       "x",
	}
	rr := doRequest(env, "PUT", "/api/settings/ai-failure-analysis", body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for max=0, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestFailureAnalysisSettings_UpdateRejectsEmptyPrompt(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	body := map[string]interface{}{
		"enabled_on_completion": false,
		"max_analyses_per_run":  10,
		"dedup_enabled":         true,
		"redaction_enabled":     true,
		"prompt_template":       "",
	}
	rr := doRequest(env, "PUT", "/api/settings/ai-failure-analysis", body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty prompt, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestFailureAnalysisSettings_UpdateValidPersists(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	body := map[string]interface{}{
		"enabled_on_completion": true,
		"max_analyses_per_run":  25,
		"dedup_enabled":         false,
		"redaction_enabled":     false,
		"prompt_template":       "Custom template",
	}
	rr := doRequest(env, "PUT", "/api/settings/ai-failure-analysis", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var got models.AIFailureAnalysisSettings
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.EnabledOnCompletion {
		t.Error("expected EnabledOnCompletion=true")
	}
	if got.MaxAnalysesPerRun != 25 {
		t.Errorf("expected MaxAnalysesPerRun=25, got %d", got.MaxAnalysesPerRun)
	}
	if got.PromptTemplate != "Custom template" {
		t.Errorf("unexpected template: %q", got.PromptTemplate)
	}

	// GET should reflect saved values.
	rr2 := doRequest(env, "GET", "/api/settings/ai-failure-analysis", nil)
	var reread models.AIFailureAnalysisSettings
	json.NewDecoder(rr2.Body).Decode(&reread)
	if reread.MaxAnalysesPerRun != 25 {
		t.Errorf("re-read expected 25, got %d", reread.MaxAnalysesPerRun)
	}
}

func TestFailureAnalysisSettings_ResetPromptRestoresDefault(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	// Mutate prompt
	doRequest(env, "PUT", "/api/settings/ai-failure-analysis", map[string]interface{}{
		"enabled_on_completion": false,
		"max_analyses_per_run":  10,
		"dedup_enabled":         true,
		"redaction_enabled":     true,
		"prompt_template":       "changed",
	})

	rr := doRequest(env, "POST", "/api/settings/ai-failure-analysis/prompt/reset", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var got models.AIFailureAnalysisSettings
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.PromptTemplate == "changed" || got.PromptTemplate == "" {
		t.Errorf("expected default template restored, got %q", got.PromptTemplate)
	}
}

// ── Not-found error paths ────────────────────────────────────────────────

func TestAnalyzeRunResult_404WhenMissing(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	rr := doRequest(env, "POST", "/api/run-results/does-not-exist/analyze", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestListRunResultAnalyses_404WhenMissing(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	rr := doRequest(env, "GET", "/api/run-results/does-not-exist/analyses", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestEnqueueRunAnalysis_404WhenRunMissing(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	rr := doRequest(env, "POST", "/api/runs/does-not-exist/analyze-failures", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGetRunAnalysisJob_404WhenNone(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	run := createTestRun(t, env, "Empty Run")
	rr := doRequest(env, "GET", "/api/runs/"+run+"/analysis-job", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCancelRunAnalysisJob_404WhenNone(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	run := createTestRun(t, env, "Empty Run")
	rr := doRequest(env, "POST", "/api/runs/"+run+"/analysis-job/cancel", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ── Valid-run happy paths ────────────────────────────────────────────────

func TestListCurrentAnalysesForEmptyRun_ReturnsEmptyMap(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	run := createTestRun(t, env, "Empty Run")
	rr := doRequest(env, "GET", "/api/runs/"+run+"/analyses/current", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var got map[string]*models.RunResultAnalysis
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %d entries", len(got))
	}
}

func TestEnqueueRunAnalysis_400WhenNoFailures(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	run := createTestRun(t, env, "No Failures Run")
	rr := doRequest(env, "POST", "/api/runs/"+run+"/analyze-failures", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestListCurrentAnalysesForRun_404WhenRunMissing(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	rr := doRequest(env, "GET", "/api/runs/does-not-exist/analyses/current", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

// createTestRun creates an empty run and returns its ID.
func createTestRun(t *testing.T, env *testEnv, name string) string {
	t.Helper()
	rr := doRequest(env, "POST", "/api/runs", map[string]interface{}{"name": name})
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		t.Fatalf("create run: got status %d, body: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		ID string `json:"id"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.ID == "" {
		t.Fatal("create run: empty ID")
	}
	return resp.ID
}
