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

// ── Suggested defect_type exposure ───────────────────────────────────────

// seedAnalyzedResult builds a store holding one FAIL result plus a handler whose
// provider always returns the given verdict. Returns the handler and the run /
// result IDs so each analysis endpoint can be driven directly.
func seedAnalyzedResult(t *testing.T, verdict string) (h *ai.Handler, runID, resultID string) {
	t.Helper()
	s, err := store.New(":memory:")
	require.NoError(t, err)

	run := &models.TestRun{Name: "suggestion"}
	require.NoError(t, s.CreateTestRun(run))
	result := &models.RunResult{
		TestRunID:        run.ID,
		TestNameSnapshot: "t",
		AttemptNumber:    1,
		Status:           models.StatusFail,
		FailureType:      "assertion",
		ErrorMessage:     "boom",
	}
	require.NoError(t, s.AddRunResult(result))

	h = ai.NewHandler(s, bluemonday.UGCPolicy())
	h.SetFailureAnalysisDeps(func() (llm.Provider, string, error) {
		return &verdictProvider{verdict: verdict}, "mock-model", nil
	}, nil)
	return h, run.ID, result.ID
}

// callAnalyze drives POST /run-results/{id}/analyze and decodes the body into out.
func callAnalyze(t *testing.T, h *ai.Handler, resultID string, out interface{}) {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/run-results/"+resultID+"/analyze", nil)
	req.SetPathValue("id", resultID)
	rr := httptest.NewRecorder()
	h.AnalyzeRunResult(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code, "analyze failed: %s", rr.Body.String())
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), out))
}

// callListAnalyses drives GET /run-results/{id}/analyses and decodes the body into out.
func callListAnalyses(t *testing.T, h *ai.Handler, resultID string, out interface{}) {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/run-results/"+resultID+"/analyses", nil)
	req.SetPathValue("id", resultID)
	rr := httptest.NewRecorder()
	h.ListRunResultAnalyses(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, "list analyses failed: %s", rr.Body.String())
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), out))
}

// callCurrentAnalyses drives GET /runs/{id}/analyses/current and decodes the body into out.
func callCurrentAnalyses(t *testing.T, h *ai.Handler, runID string, out interface{}) {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/runs/"+runID+"/analyses/current", nil)
	req.SetPathValue("id", runID)
	rr := httptest.NewRecorder()
	h.ListCurrentAnalysesForRun(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, "current analyses failed: %s", rr.Body.String())
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), out))
}

// TestAnalysisEndpoints_ExposeSuggestedDefectType asserts every verdict surfaces
// the documented suggested defect_type on ALL THREE analysis endpoints: the
// synchronous analyze (a single object — no loop, so it needs its own
// assignment), the per-result version list, and the per-run current map.
// The mapping is lossy (6 verdicts → 4 defect types) and lives only in
// models.SuggestedDefectType; the frontend must never re-derive it.
func TestAnalysisEndpoints_ExposeSuggestedDefectType(t *testing.T) {
	tests := []struct {
		verdict string
		want    string
	}{
		{models.VerdictProductBug, "product_bug"},
		{models.VerdictFlakyTest, "automation_bug"},
		{models.VerdictTestData, "automation_bug"},
		{models.VerdictEnvironment, "system_issue"},
		{models.VerdictInfrastructure, "system_issue"},
		{models.VerdictUnknown, ""},
	}
	for _, tt := range tests {
		t.Run(tt.verdict, func(t *testing.T) {
			h, runID, resultID := seedAnalyzedResult(t, tt.verdict)

			var created models.RunResultAnalysis
			callAnalyze(t, h, resultID, &created)
			require.Equal(t, tt.verdict, created.Verdict)
			require.Equal(t, tt.want, created.SuggestedDefectType, "AnalyzeRunResult")

			var list []*models.RunResultAnalysis
			callListAnalyses(t, h, resultID, &list)
			require.Len(t, list, 1)
			require.Equal(t, tt.want, list[0].SuggestedDefectType, "ListRunResultAnalyses")

			var current map[string]*models.RunResultAnalysis
			callCurrentAnalyses(t, h, runID, &current)
			require.Contains(t, current, resultID)
			require.Equal(t, tt.want, current[resultID].SuggestedDefectType, "ListCurrentAnalysesForRun")
		})
	}
}

// TestAnalysisEndpoints_RawResponseStrippedFromListsOnly locks in the exact scope
// of F-068: the raw LLM output is omitted from the two LIST responses but stays
// on the synchronous analyze response. Decoded as raw maps because RawResponse is
// `omitempty` — stripping it drops the key entirely rather than emptying it.
func TestAnalysisEndpoints_RawResponseStrippedFromListsOnly(t *testing.T) {
	h, runID, resultID := seedAnalyzedResult(t, models.VerdictFlakyTest)

	var created map[string]interface{}
	callAnalyze(t, h, resultID, &created)
	raw, ok := created["raw_response"]
	require.True(t, ok, "AnalyzeRunResult must still return raw_response — F-068 covers list responses only")
	require.NotEmpty(t, raw)
	require.Equal(t, "automation_bug", created["suggested_defect_type"])

	var list []map[string]interface{}
	callListAnalyses(t, h, resultID, &list)
	require.Len(t, list, 1)
	require.NotContains(t, list[0], "raw_response", "raw_response must stay stripped from ListRunResultAnalyses (F-068)")
	require.Equal(t, "automation_bug", list[0]["suggested_defect_type"])

	var current map[string]map[string]interface{}
	callCurrentAnalyses(t, h, runID, &current)
	require.Contains(t, current, resultID)
	require.NotContains(t, current[resultID], "raw_response", "raw_response must stay stripped from ListCurrentAnalysesForRun (F-068)")
	require.Equal(t, "automation_bug", current[resultID]["suggested_defect_type"])
}

// ── Accuracy endpoint ────────────────────────────────────────────────────

// TestFailureAnalysisAccuracy_EndpointShapeAndExclusions checks the wiring end to end: the route
// resolves, the documented JSON shape comes back, and the untriaged auto-default is excluded
// rather than counted against the AI.
func TestFailureAnalysisAccuracy_EndpointShapeAndExclusions(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	s := env.store

	run := &models.TestRun{Name: "triaged"}
	require.NoError(t, s.CreateTestRun(run))
	add := func(attempt int, verdict, suggested, confidence, defectType string) {
		require.NoError(t, s.AddRunResult(&models.RunResult{
			TestRunID: run.ID, TestNameSnapshot: "t", AttemptNumber: attempt,
			Status: models.StatusFail, ErrorMessage: "boom",
			DefectType:          defectType,
			SuggestedVerdict:    verdict,
			SuggestedDefectType: suggested,
			SuggestedConfidence: confidence,
		}))
	}
	add(1, models.VerdictProductBug, "product_bug", models.ConfidenceHigh, "product_bug")    // agreed
	add(2, models.VerdictFlakyTest, "automation_bug", models.ConfidenceLow, "product_bug")   // overridden
	add(3, models.VerdictProductBug, "product_bug", models.ConfidenceHigh, "to_investigate") // untriaged
	add(4, models.VerdictUnknown, "", models.ConfidenceLow, "product_bug")                   // no suggestion

	rr := doRequest(env, "GET", "/api/ai/failure-analysis/accuracy", nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var got struct {
		Total         int     `json:"total"`
		Agreed        int     `json:"agreed"`
		AgreementRate float64 `json:"agreement_rate"`
		ByVerdict     []struct {
			Verdict string  `json:"verdict"`
			Total   int     `json:"total"`
			Agreed  int     `json:"agreed"`
			Rate    float64 `json:"rate"`
		} `json:"by_verdict"`
		ByConfidence []struct {
			Confidence string  `json:"confidence"`
			Total      int     `json:"total"`
			Agreed     int     `json:"agreed"`
			Rate       float64 `json:"rate"`
		} `json:"by_confidence"`
	}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))

	require.Equal(t, 2, got.Total, "untriaged and no-suggestion rows are not part of the calibration set")
	require.Equal(t, 1, got.Agreed)
	require.InDelta(t, 0.5, got.AgreementRate, 1e-9)
	require.Len(t, got.ByVerdict, 2)
	require.Len(t, got.ByConfidence, 2)
}

// TestFailureAnalysisAccuracy_DaysParam covers the window parameter: a junk or non-positive
// value falls back to the 30-day default, and an absurd value is clamped instead of rejected.
func TestFailureAnalysisAccuracy_DaysParam(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	for _, q := range []string{"", "?days=7", "?days=abc", "?days=0", "?days=-5", "?days=99999"} {
		rr := doRequest(env, "GET", "/api/ai/failure-analysis/accuracy"+q, nil)
		require.Equal(t, http.StatusOK, rr.Code, "days=%q: %s", q, rr.Body.String())
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
