package failureanalysis

import (
	"context"
	"errors"
	"testing"
	"time"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

type stubProvider struct {
	responses []string
	errs      []error
	calls     int
}

func (s *stubProvider) Chat(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	i := s.calls
	s.calls++
	if i < len(s.errs) && s.errs[i] != nil {
		return nil, s.errs[i]
	}
	return &llm.ChatResponse{
		Content: s.responses[i],
		Usage:   &llm.ChatUsage{PromptTokens: 100, CompletionTokens: 20, TotalTokens: 120},
	}, nil
}

func baseContext() AnalyzeContext {
	return AnalyzeContext{
		Result: &models.RunResult{
			ID: "rr1", TestNameSnapshot: "Login", Status: "FAIL",
			FailureType: "assertion", ErrorMessage: "expected 401, got 500",
			StackTrace: "stack", LogText: "log",
		},
		RedactionEnabled: true,
		PromptTemplate:   DefaultPromptTemplate,
		ProviderModel:    "gpt-test",
	}
}

func TestAnalyzeHappyPath(t *testing.T) {
	prov := &stubProvider{responses: []string{
		`{"verdict":"product_bug","confidence":"high","summary":"s","next_action":"n","rationale":"r"}`,
	}}
	out, err := Analyze(context.Background(), prov, baseContext())
	require.NoError(t, err)
	require.Equal(t, models.VerdictProductBug, out.Verdict)
	require.Equal(t, models.ConfidenceHigh, out.Confidence)
	require.Equal(t, 100, out.TokenUsagePrompt)
}

func TestAnalyzeRetriesOnInvalidJSON(t *testing.T) {
	prov := &stubProvider{responses: []string{
		`this is not json`,
		`{"verdict":"flaky_test","confidence":"medium","summary":"s","next_action":"n","rationale":"r"}`,
	}}
	out, err := Analyze(context.Background(), prov, baseContext())
	require.NoError(t, err)
	require.Equal(t, 2, prov.calls)
	require.Equal(t, models.VerdictFlakyTest, out.Verdict)
}

func TestAnalyzeFallsBackOnTwoInvalidResponses(t *testing.T) {
	prov := &stubProvider{responses: []string{"nope", "still nope"}}
	out, err := Analyze(context.Background(), prov, baseContext())
	require.NoError(t, err)
	require.Equal(t, models.VerdictUnknown, out.Verdict)
	require.Equal(t, models.ConfidenceLow, out.Confidence)
	require.Contains(t, out.Rationale, "still nope")
}

func TestAnalyzeAcceptsMixedCaseVerdictAndNumericConfidence(t *testing.T) {
	prov := &stubProvider{responses: []string{
		`{"verdict":"Infrastructure","confidence":0.95,"summary":"s","next_action":"n","rationale":"r"}`,
	}}
	out, err := Analyze(context.Background(), prov, baseContext())
	require.NoError(t, err)
	require.Equal(t, models.VerdictInfrastructure, out.Verdict)
	require.Equal(t, models.ConfidenceHigh, out.Confidence)
}

func TestParseVerdictNormalizesVerdictFormatting(t *testing.T) {
	cases := map[string]string{
		`{"verdict":"Product Bug","confidence":"Medium","summary":"","next_action":"","rationale":""}`: models.VerdictProductBug,
		`{"verdict":"flaky-test","confidence":"low","summary":"","next_action":"","rationale":""}`:     models.VerdictFlakyTest,
		`{"verdict":"  Test Data  ","confidence":"high","summary":"","next_action":"","rationale":""}`: models.VerdictTestData,
	}
	for input, expected := range cases {
		v, err := parseVerdict(input)
		require.NoError(t, err, input)
		require.Equal(t, expected, v.Verdict, input)
	}
}

func TestParseVerdictNumericConfidenceBuckets(t *testing.T) {
	cases := map[string]string{
		`{"verdict":"product_bug","confidence":0.95,"summary":"","next_action":"","rationale":""}`:   models.ConfidenceHigh,
		`{"verdict":"product_bug","confidence":0.5,"summary":"","next_action":"","rationale":""}`:    models.ConfidenceMedium,
		`{"verdict":"product_bug","confidence":0.1,"summary":"","next_action":"","rationale":""}`:    models.ConfidenceLow,
		`{"verdict":"product_bug","confidence":"HIGH","summary":"","next_action":"","rationale":""}`: models.ConfidenceHigh,
	}
	for input, expected := range cases {
		v, err := parseVerdict(input)
		require.NoError(t, err, input)
		require.Equal(t, expected, v.Confidence, input)
	}
}

func TestAnalyzeReturnsProviderErrorDirectly(t *testing.T) {
	prov := &stubProvider{
		responses: []string{""},
		errs:      []error{errors.New("provider unavailable")},
	}
	_, err := Analyze(context.Background(), prov, baseContext())
	require.Error(t, err)
	require.Contains(t, err.Error(), "provider unavailable")
}

// capturingProvider records the rendered prompt from the first chat message so
// tests can assert what actually reached the model.
type capturingProvider struct {
	lastPrompt string
}

func (c *capturingProvider) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if len(req.Messages) > 0 {
		c.lastPrompt = req.Messages[0].Content
	}
	return &llm.ChatResponse{
		Content: `{"verdict":"product_bug","confidence":"high","summary":"s","next_action":"n","rationale":"r"}`,
		Usage:   &llm.ChatUsage{PromptTokens: 100, CompletionTokens: 20, TotalTokens: 120},
	}, nil
}

func TestAnalyzeRedactsSimilarFailureMessages(t *testing.T) {
	const raw = "auth failed: Bearer abcdefghijklmnopqrstuvwxyz0123456789"
	prov := &capturingProvider{}
	in := baseContext() // RedactionEnabled: true
	in.SimilarFailures = []SimilarFailure{
		{RunStartedAt: time.Now(), Status: "FAIL", ErrorMessage: raw},
	}

	_, err := Analyze(context.Background(), prov, in)
	require.NoError(t, err)

	// The enriched history secret must be scrubbed in the rendered prompt.
	require.Contains(t, prov.lastPrompt, "Bearer <REDACTED_TOKEN>")
	require.NotContains(t, prov.lastPrompt, "abcdefghijklmnopqrstuvwxyz0123456789")

	// Redaction must not mutate the caller's shared backing array.
	require.Equal(t, raw, in.SimilarFailures[0].ErrorMessage,
		"caller's SimilarFailures slice must not be mutated by redaction")
}

func TestAnalyzeLeavesSimilarFailuresRawWhenRedactionDisabled(t *testing.T) {
	const raw = "auth failed: Bearer abcdefghijklmnopqrstuvwxyz0123456789"
	prov := &capturingProvider{}
	in := baseContext()
	in.RedactionEnabled = false
	in.SimilarFailures = []SimilarFailure{
		{RunStartedAt: time.Now(), Status: "FAIL", ErrorMessage: raw},
	}

	_, err := Analyze(context.Background(), prov, in)
	require.NoError(t, err)

	require.Contains(t, prov.lastPrompt, "abcdefghijklmnopqrstuvwxyz0123456789")
	require.NotContains(t, prov.lastPrompt, "<REDACTED_TOKEN>")
}
