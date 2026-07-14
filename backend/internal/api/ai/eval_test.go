package ai_test

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestRealProviderEvaluation exercises the full generation pipeline against a
// real (or locally hosted) provider and logs quality metrics. It is opt-in
// and excluded from CI (spec: real-provider evaluation is opt-in):
//
//	TTGO_EVAL_LLM_ENDPOINT=http://localhost:11434 \
//	TTGO_EVAL_LLM_MODEL=llama3 \
//	go test -tags sqlite_fts5 ./internal/api/ai/ -run TestRealProviderEvaluation -v
func TestRealProviderEvaluation(t *testing.T) {
	endpoint := os.Getenv("TTGO_EVAL_LLM_ENDPOINT")
	model := os.Getenv("TTGO_EVAL_LLM_MODEL")
	if endpoint == "" || model == "" {
		t.Skip("set TTGO_EVAL_LLM_ENDPOINT and TTGO_EVAL_LLM_MODEL to run the real-provider evaluation")
	}

	env, cleanup := testServer(t)
	defer cleanup()

	rr := doRequest(env, "POST", "/api/settings/llm-providers", map[string]interface{}{
		"label": "eval-provider", "provider_type": "local",
		"endpoint_url": endpoint, "model_name": model, "timeout_seconds": 600,
	})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	var provider struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &provider))

	reqID := createPreviewRequirement(t, env, "REQ-EVAL-1", "Login form",
		`<h2>Acceptance Criteria</h2><ul><li>User can sign in with a valid email and password</li><li>Wrong password shows an inline error</li><li>Account locks after 5 failed attempts</li></ul>`)

	rr = doRequest(env, "POST", "/api/ai-generations", map[string]string{
		"requirement_id": reqID, "provider_id": provider.ID, "idempotency_key": uuid.NewString(),
	})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	var body struct {
		Run struct {
			Status      string `json:"status"`
			TotalTokens int    `json:"total_tokens"`
			DurationMs  int64  `json:"duration_ms"`
		} `json:"run"`
		Drafts []struct {
			Findings   json.RawMessage `json:"findings"`
			Quality    json.RawMessage `json:"quality"`
			SourceRefs []string        `json:"source_refs"`
		} `json:"drafts"`
		Coverage struct {
			CoveredCount   int `json:"covered_count"`
			UncoveredCount int `json:"uncovered_count"`
		} `json:"coverage"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))

	withFindings, withQuality, withRefs := 0, 0, 0
	for _, d := range body.Drafts {
		if len(d.Findings) > 0 {
			withFindings++
		}
		if len(d.Quality) > 0 {
			withQuality++
		}
		if len(d.SourceRefs) > 0 {
			withRefs++
		}
	}
	t.Logf("EVAL model=%s drafts=%d structural_findings=%d quality_flagged=%d with_source_refs=%d covered=%d uncovered=%d tokens=%d duration_ms=%d",
		model, len(body.Drafts), withFindings, withQuality, withRefs,
		body.Coverage.CoveredCount, body.Coverage.UncoveredCount,
		body.Run.TotalTokens, body.Run.DurationMs)
	require.NotEmpty(t, body.Drafts, "real provider returned zero drafts")
}
