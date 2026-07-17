package ai_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ttgo/internal/api/ai"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"

	"github.com/microcosm-cc/bluemonday"
	"github.com/stretchr/testify/require"
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

// ── Enrichment anti-regression (synchronous analyzeSync path) ────────────

// capturingHandlerProvider records the rendered prompt from the first chat
// message so the test can assert what enrichment reached the model.
type capturingHandlerProvider struct{ lastPrompt string }

func (c *capturingHandlerProvider) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if len(req.Messages) > 0 {
		c.lastPrompt = req.Messages[0].Content
	}
	return &llm.ChatResponse{
		Content: `{"verdict":"product_bug","confidence":"high","summary":"s","next_action":"n","rationale":"r"}`,
		Model:   "mock-model",
		Usage:   &llm.ChatUsage{PromptTokens: 100, CompletionTokens: 20, TotalTokens: 120},
	}, nil
}

// TestAnalyzeRunResult_SendsEnrichmentToProvider is the anti-regression guard
// for the synchronous handler call site (analyzeSync via AnalyzeRunResult): it
// seeds a test case with a linked defect plus a prior failure in another run,
// runs analysis with a fake provider, and asserts BOTH the linked defect and
// the historical-failure block reach the model. If handler.go ever reverts to a
// bare AnalyzeContext{Result: ...}, this fails.
func TestAnalyzeRunResult_SendsEnrichmentToProvider(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)

	// Test case with a linked defect.
	folder, err := s.CreateFolder("Checkout Suite", nil)
	require.NoError(t, err)
	tc := &models.TestCase{FolderID: folder.ID, Name: "Checkout flow"}
	require.NoError(t, s.CreateTestCase(tc))
	tcID := tc.ID

	defect := &models.Defect{Title: "Checkout returns 500", Status: "open", ExternalKey: "JIRA-909"}
	require.NoError(t, s.CreateDefect(defect))
	_, err = s.LinkDefectToTestCase(defect.ID, tcID)
	require.NoError(t, err)

	// A prior failure for the same test case in a DIFFERENT run, inside the
	// 30-day window (StartTime must be set explicitly — AddRunResult does not).
	priorRun := &models.TestRun{Name: "nightly"}
	require.NoError(t, s.CreateTestRun(priorRun))
	require.NoError(t, s.AddRunResult(&models.RunResult{
		TestRunID: priorRun.ID, TestCaseID: &tcID, TestNameSnapshot: "Checkout flow",
		AttemptNumber: 1, Status: models.StatusFail, FailureType: "assertion",
		ErrorMessage: "PRIOR_FAILURE_MARKER gateway timeout",
		DefectType:   "product_bug",
		StartTime:    time.Now().Add(-48 * time.Hour),
	}))

	// The target result analyzed synchronously.
	run := &models.TestRun{Name: "current"}
	require.NoError(t, s.CreateTestRun(run))
	target := &models.RunResult{
		TestRunID: run.ID, TestCaseID: &tcID, TestNameSnapshot: "Checkout flow",
		AttemptNumber: 1, Status: models.StatusFail, FailureType: "assertion",
		ErrorMessage: "current failure",
	}
	require.NoError(t, s.AddRunResult(target))

	// Redaction off so the historical marker passes through verbatim.
	_, err = s.UpdateFailureAnalysisSettings(&models.AIFailureAnalysisSettings{
		MaxAnalysesPerRun: 5, DedupEnabled: true, RedactionEnabled: false,
	})
	require.NoError(t, err)

	h := ai.NewHandler(s, bluemonday.UGCPolicy())
	prov := &capturingHandlerProvider{}
	h.SetFailureAnalysisDeps(func() (llm.Provider, string, error) {
		return prov, "mock-model", nil
	}, nil)

	req := httptest.NewRequest("POST", "/api/run-results/"+target.ID+"/analyze", nil)
	req.SetPathValue("id", target.ID)
	rr := httptest.NewRecorder()
	h.AnalyzeRunResult(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code, "unexpected status; body: %s", rr.Body.String())

	// Enrichment must reach the model: linked defect (key + title) and the
	// historical-failure block (its message + human triage label / rollup).
	require.NotEmpty(t, prov.lastPrompt, "provider captured no prompt")
	require.Contains(t, prov.lastPrompt, "JIRA-909", "linked defect key missing from prompt")
	require.Contains(t, prov.lastPrompt, "Checkout returns 500", "linked defect title missing from prompt")
	require.Contains(t, prov.lastPrompt, "PRIOR_FAILURE_MARKER", "historical failure message missing from prompt")
	require.Contains(t, prov.lastPrompt, "product_bug", "human triage label / rollup missing from prompt")
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
