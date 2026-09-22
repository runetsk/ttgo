package failureanalysis

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/typesafe"

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
	out, err := Analyze(context.Background(), AnalyzeDeps{Narrative: prov, NarrativeModel: "gpt-test"}, baseContext())
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
	out, err := Analyze(context.Background(), AnalyzeDeps{Narrative: prov, NarrativeModel: "gpt-test"}, baseContext())
	require.NoError(t, err)
	require.Equal(t, 2, prov.calls)
	require.Equal(t, models.VerdictFlakyTest, out.Verdict)
}

func TestAnalyzeFallsBackOnTwoInvalidResponses(t *testing.T) {
	prov := &stubProvider{responses: []string{"nope", "still nope"}}
	out, err := Analyze(context.Background(), AnalyzeDeps{Narrative: prov, NarrativeModel: "gpt-test"}, baseContext())
	require.NoError(t, err)
	require.Equal(t, models.VerdictUnknown, out.Verdict)
	require.Equal(t, models.ConfidenceLow, out.Confidence)
	require.Contains(t, out.Rationale, "still nope")
}

func TestAnalyzeAcceptsMixedCaseVerdictAndNumericConfidence(t *testing.T) {
	prov := &stubProvider{responses: []string{
		`{"verdict":"Infrastructure","confidence":0.95,"summary":"s","next_action":"n","rationale":"r"}`,
	}}
	out, err := Analyze(context.Background(), AnalyzeDeps{Narrative: prov, NarrativeModel: "gpt-test"}, baseContext())
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
	_, err := Analyze(context.Background(), AnalyzeDeps{Narrative: prov, NarrativeModel: "gpt-test"}, baseContext())
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

	_, err := Analyze(context.Background(), AnalyzeDeps{Narrative: prov, NarrativeModel: "gpt-test"}, in)
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

	_, err := Analyze(context.Background(), AnalyzeDeps{Narrative: prov, NarrativeModel: "gpt-test"}, in)
	require.NoError(t, err)

	require.Contains(t, prov.lastPrompt, "abcdefghijklmnopqrstuvwxyz0123456789")
	require.NotContains(t, prov.lastPrompt, "<REDACTED_TOKEN>")
}

func TestAnalyzeSendsRollupToProvider(t *testing.T) {
	// Guards the SimilarFailuresRollup mapping in Analyze -> BuildPrompt. The
	// per-row "(human: ...)" clause repeats each label, so only the "×N"
	// distribution line proves the rollup itself reached the model; drop the
	// mapping and this is the sole test that fails.
	prov := &capturingProvider{}
	in := baseContext()
	in.RedactionEnabled = false
	in.SimilarFailures = []SimilarFailure{
		{Status: "FAIL", ErrorMessage: "boom", DefectType: "product_bug"},
		{Status: "FAIL", ErrorMessage: "bang", DefectType: "product_bug"},
		{Status: "ERROR", ErrorMessage: "thud", DefectType: "flaky"},
	}
	in.SimilarFailuresRollup = "product_bug " + timesGlyph + "2, flaky " + timesGlyph + "1"

	_, err := Analyze(context.Background(), AnalyzeDeps{Narrative: prov, NarrativeModel: "gpt-test"}, in)
	require.NoError(t, err)

	// "×N" appears only in the rollup, never in a per-row clause.
	require.Contains(t, prov.lastPrompt, "product_bug "+timesGlyph+"2, flaky "+timesGlyph+"1")
}

type recordingProvider struct {
	stubProvider
	reqs []llm.ChatRequest
}

func (r *recordingProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	r.reqs = append(r.reqs, req)
	return r.stubProvider.Chat(ctx, req)
}

type fixedDecider struct {
	d   *Decision
	err error
}

func (f fixedDecider) Decide(context.Context, Evidence) (*Decision, error) { return f.d, f.err }

func flakyDecision() *Decision {
	return &Decision{Verdict: models.VerdictFlakyTest, VerdictConfidence: 0.93,
		VerdictProbabilities: map[string]float64{"flaky_test": 0.95, "product_bug": 0.03, "environment": 0.02},
		SuggestedDefectType:  "automation_bug", DefectTypeConfidence: 0.88,
		DefectTypeProbabilities: map[string]float64{"automation_bug": 0.9, "product_bug": 0.1},
		Model:                   "jev-1.13.0", InputTokens: 777, PolicyVersion: PolicyVersion}
}

func TestAnalyze_TypeSafeDecidesGenerativeExplains(t *testing.T) {
	prov := &recordingProvider{stubProvider: stubProvider{responses: []string{`{"summary":"S","next_action":"N","rationale":"R"}`}}}
	out, err := Analyze(context.Background(), AnalyzeDeps{Narrative: prov, NarrativeModel: "gpt-test", Decider: fixedDecider{d: flakyDecision()}}, baseContext())
	require.NoError(t, err)
	require.Equal(t, models.AnalysisEngineTypeSafe, out.Engine)
	require.Equal(t, models.VerdictFlakyTest, out.Verdict)
	require.Equal(t, models.ConfidenceHigh, out.Confidence)
	require.InDelta(t, 0.93, *out.ConfidenceScore, 1e-9)
	require.Equal(t, "automation_bug", out.SuggestedDefectType)
	require.InDelta(t, 0.88, *out.SuggestedDefectTypeConfidence, 1e-9)
	require.Equal(t, "S", out.Summary)
	require.Equal(t, "N", out.NextAction)
	require.Equal(t, "R", out.Rationale)
	require.Equal(t, models.NarrativeStatusOK, out.NarrativeStatus)
	require.Equal(t, PolicyVersion, out.PolicyVersion)
	require.Equal(t, 777, out.TypeSafeInputTokens)
	require.Contains(t, out.VerdictProbabilities, `"flaky_test":0.95`)

	require.Len(t, prov.reqs, 1)
	msgs := prov.reqs[0].Messages
	require.Equal(t, "system", msgs[0].Role)
	require.Contains(t, msgs[0].Content, "must not be changed")
	require.Contains(t, msgs[0].Content, "`flaky_test`")
	require.Contains(t, msgs[0].Content, "`automation_bug`")
	require.Contains(t, msgs[0].Content, `{"summary"`)
	require.NotContains(t, msgs[0].Content, "runner-up", "gap 0.95-0.03 is above RunnerUpMargin")
	require.Equal(t, "user", msgs[1].Role)
	require.Contains(t, msgs[1].Content, "expected 401, got 500")
}

func TestAnalyze_RunnerUpMentionedOnlyUnderMargin(t *testing.T) {
	d := flakyDecision()
	d.VerdictProbabilities = map[string]float64{"flaky_test": 0.5, "product_bug": 0.42, "environment": 0.08}
	prov := &recordingProvider{stubProvider: stubProvider{responses: []string{`{"summary":"S","next_action":"N","rationale":"R"}`}}}
	_, err := Analyze(context.Background(), AnalyzeDeps{Narrative: prov, Decider: fixedDecider{d: d}}, baseContext())
	require.NoError(t, err)
	require.Contains(t, prov.reqs[0].Messages[0].Content, "runner-up verdict was `product_bug`")
}

func TestAnalyze_CustomizedTemplateExtraFieldsIgnored(t *testing.T) {
	prov := &stubProvider{responses: []string{`{"verdict":"product_bug","confidence":"high","summary":"S","next_action":"N","rationale":"R"}`}}
	out, err := Analyze(context.Background(), AnalyzeDeps{Narrative: prov, Decider: fixedDecider{d: flakyDecision()}}, baseContext())
	require.NoError(t, err)
	require.Equal(t, models.VerdictFlakyTest, out.Verdict, "the LLM's verdict is ignored")
	require.Equal(t, "S", out.Summary)
}

func TestAnalyze_UnparseableNarrativeKeepsDecision(t *testing.T) {
	prov := &stubProvider{responses: []string{"nope", "still nope"}}
	out, err := Analyze(context.Background(), AnalyzeDeps{Narrative: prov, Decider: fixedDecider{d: flakyDecision()}}, baseContext())
	require.NoError(t, err)
	require.Equal(t, models.VerdictFlakyTest, out.Verdict)
	require.Equal(t, models.NarrativeStatusUnparseable, out.NarrativeStatus)
	require.Contains(t, out.Summary, "unparseable")
	require.Contains(t, out.Rationale, "still nope")
}

func TestAnalyze_NarrativeProviderErrorsKeepDecision(t *testing.T) {
	cases := map[string]*stubProvider{
		"first call":  {errs: []error{errors.New("boom")}},
		"repair call": {responses: []string{"nope", ""}, errs: []error{nil, errors.New("boom")}},
	}
	for name, prov := range cases {
		out, err := Analyze(context.Background(), AnalyzeDeps{Narrative: prov, Decider: fixedDecider{d: flakyDecision()}}, baseContext())
		require.NoError(t, err, name)
		require.Equal(t, models.VerdictFlakyTest, out.Verdict, name)
		require.Equal(t, models.NarrativeStatusUnavailable, out.NarrativeStatus, name)
		require.Contains(t, out.Summary, "AI narrative unavailable", name)
	}
}

func TestAnalyze_NarrativeProviderErrorCategoryReachesSummary(t *testing.T) {
	prov := &stubProvider{errs: []error{&llm.ProviderError{Category: llm.ErrCatTimeout, Message: "deadline"}}}
	out, err := Analyze(context.Background(), AnalyzeDeps{Narrative: prov, Decider: fixedDecider{d: flakyDecision()}}, baseContext())
	require.NoError(t, err)
	require.Equal(t, models.VerdictFlakyTest, out.Verdict)
	require.Equal(t, models.NarrativeStatusUnavailable, out.NarrativeStatus)
	require.Equal(t, "AI narrative unavailable: timeout", out.Summary)
}

func TestAnalyze_TemplateErrorKeepsDecision(t *testing.T) {
	in := baseContext()
	in.PromptTemplate = "{{ .Broken"
	prov := &stubProvider{responses: []string{`{"summary":"S","next_action":"N","rationale":"R"}`}}
	out, err := Analyze(context.Background(), AnalyzeDeps{Narrative: prov, Decider: fixedDecider{d: flakyDecision()}}, in)
	require.NoError(t, err)
	require.Equal(t, models.NarrativeStatusUnavailable, out.NarrativeStatus)
	require.Equal(t, models.VerdictFlakyTest, out.Verdict)
}

func TestAnalyze_CancelledContextPropagates(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	prov := &stubProvider{errs: []error{context.Canceled}}
	_, err := Analyze(ctx, AnalyzeDeps{Narrative: prov, Decider: fixedDecider{d: flakyDecision()}}, baseContext())
	require.ErrorIs(t, err, context.Canceled)

	_, err = Analyze(ctx, AnalyzeDeps{Narrative: prov, Decider: fixedDecider{err: errors.New("typesafe down")}}, baseContext())
	require.ErrorIs(t, err, context.Canceled, "decider error with a cancelled context must not fall back")
}

func TestAnalyze_DeciderErrorFallsBackToGenerative(t *testing.T) {
	prov := &stubProvider{responses: []string{`{"verdict":"product_bug","confidence":"high","summary":"s","next_action":"n","rationale":"r"}`}}
	deciderErr := &typesafe.Error{Category: typesafe.CategoryRateLimit, Status: 429, Message: "slow down"}
	out, err := Analyze(context.Background(), AnalyzeDeps{Narrative: prov, Decider: fixedDecider{err: deciderErr}}, baseContext())
	require.NoError(t, err)
	require.Equal(t, models.AnalysisEngineGenerative, out.Engine)
	require.Equal(t, models.VerdictProductBug, out.Verdict)
	require.Equal(t, "product_bug", out.SuggestedDefectType, "generative rows persist the legacy mapping")
	require.Nil(t, out.ConfidenceScore)
	require.True(t, strings.HasPrefix(out.Rationale, "[verdict engine: TypeSafe unavailable (rate_limit); used generative] "), out.Rationale)
}

func TestErrCategory_ExtractsRealCategories(t *testing.T) {
	require.Equal(t, "rate_limit", errCategory(&typesafe.Error{Category: typesafe.CategoryRateLimit, Status: 429}))
	require.Equal(t, "timeout", errCategory(&llm.ProviderError{Category: llm.ErrCatTimeout, Message: "t"}))
	require.Equal(t, "error", errCategory(errors.New("boom")))
	require.Equal(t, "overloaded", errCategory(fmt.Errorf("wrap: %w", &typesafe.Error{Category: typesafe.CategoryOverloaded})))
}

func TestAnalyze_DecisionWithoutNarrativeProvider(t *testing.T) {
	out, err := Analyze(context.Background(), AnalyzeDeps{Decider: fixedDecider{d: flakyDecision()}}, baseContext())
	require.NoError(t, err)
	require.Equal(t, models.NarrativeStatusUnavailable, out.NarrativeStatus)
	require.Equal(t, models.VerdictFlakyTest, out.Verdict)
	require.Contains(t, out.Summary, "no generative provider")

	_, err = Analyze(context.Background(), AnalyzeDeps{}, baseContext())
	require.Error(t, err, "generative path with no provider is still an error")
}

// TestAnalyze_EmptyNarrativeTriggersRepair covers F1: a reply that unmarshals cleanly but
// carries no narrative content ({}, {"error":"quota"}, or a customized template's
// verdict/confidence-only body) must not be accepted as a valid narrative — it has to fire
// the same repair retry as invalid JSON.
func TestAnalyze_EmptyNarrativeTriggersRepair(t *testing.T) {
	prov := &stubProvider{responses: []string{
		`{}`,
		`{"summary":"S","next_action":"N","rationale":"R"}`,
	}}
	out, err := Analyze(context.Background(), AnalyzeDeps{Narrative: prov, Decider: fixedDecider{d: flakyDecision()}}, baseContext())
	require.NoError(t, err)
	require.Equal(t, 2, prov.calls, "the empty first reply must trigger the repair call")
	require.Equal(t, models.NarrativeStatusOK, out.NarrativeStatus)
	require.Equal(t, "S", out.Summary)
	require.Equal(t, "N", out.NextAction)
	require.Equal(t, "R", out.Rationale)
	require.Equal(t, models.VerdictFlakyTest, out.Verdict, "decision is preserved")
}

func TestAnalyze_BothRepliesEmptyIsUnparseable(t *testing.T) {
	prov := &stubProvider{responses: []string{`{}`, `{}`}}
	out, err := Analyze(context.Background(), AnalyzeDeps{Narrative: prov, Decider: fixedDecider{d: flakyDecision()}}, baseContext())
	require.NoError(t, err)
	require.Equal(t, 2, prov.calls)
	require.Equal(t, models.NarrativeStatusUnparseable, out.NarrativeStatus)
	require.Equal(t, models.VerdictFlakyTest, out.Verdict, "decision is preserved")
}

func TestAnalyze_VerdictOnlyReplyIsUnparseableOnBothTries(t *testing.T) {
	prov := &stubProvider{responses: []string{
		`{"verdict":"x","confidence":"high"}`,
		`{"verdict":"x","confidence":"high"}`,
	}}
	out, err := Analyze(context.Background(), AnalyzeDeps{Narrative: prov, Decider: fixedDecider{d: flakyDecision()}}, baseContext())
	require.NoError(t, err)
	require.Equal(t, 2, prov.calls)
	require.Equal(t, models.NarrativeStatusUnparseable, out.NarrativeStatus)
	require.Equal(t, models.VerdictFlakyTest, out.Verdict, "decision is preserved")
}

func TestAnalyze_GenerativePathPersistsLegacySuggestion(t *testing.T) {
	prov := &stubProvider{responses: []string{`{"verdict":"environment","confidence":"medium","summary":"s","next_action":"n","rationale":"r"}`}}
	out, err := Analyze(context.Background(), AnalyzeDeps{Narrative: prov}, baseContext())
	require.NoError(t, err)
	require.Equal(t, models.AnalysisEngineGenerative, out.Engine)
	require.Equal(t, "system_issue", out.SuggestedDefectType)
	require.Equal(t, models.NarrativeStatusOK, out.NarrativeStatus)
}

func TestAnalyze_TypeSafeGetsTheFullLog(t *testing.T) {
	marker := "UNIQUE-LOG-START-MARKER"
	in := baseContext()
	in.Result.LogText = marker + " " + strings.Repeat("l", 30000)
	fc := &fakeClient{fn: func(typesafe.Request) (*typesafe.Response, error) {
		return decisionResponse("product_bug", 0.95, 0.9, "product_bug", 0.9, 0.9), nil
	}}
	prov := &stubProvider{responses: []string{`{"summary":"s","next_action":"n","rationale":"r"}`}}
	out, err := Analyze(context.Background(), AnalyzeDeps{Decider: NewTypeSafeDecider(fc, "jev"), Narrative: prov, NarrativeModel: "m"}, in)
	require.NoError(t, err)
	require.Equal(t, "typesafe", string(out.Engine))
	require.Len(t, fc.calls, 1)
	logTail := fc.calls[0].State.(map[string]any)["failure"].(map[string]any)["log_tail"].(string)
	require.True(t, strings.HasPrefix(logTail, marker), "Jev is given the whole log, not the 2,000-character tail")
}
