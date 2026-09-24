package failureanalysis

import (
	"context"
	"errors"
	"strings"
	"testing"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/typesafe"

	"github.com/stretchr/testify/require"
)

const goodVerdict = `{"verdict":"environment","confidence":"high","summary":"s","next_action":"n","rationale":"r"}`

func TestAnalyze_NoFallbackRecordsTypeSafeFailure(t *testing.T) {
	prov := &stubProvider{responses: []string{goodVerdict}}
	tsErr := &typesafe.Error{Category: typesafe.CategoryRateLimit, Status: 429, Message: "slow down"}
	deps := AnalyzeDeps{Narrative: prov, NarrativeModel: "gpt-test", Decider: fixedDecider{err: tsErr}, DeciderModel: "jev-latest", NoLLMFallback: true}
	_, err := Analyze(context.Background(), deps, baseContext())
	require.ErrorIs(t, err, ErrTypeSafeUnavailable)
	require.Equal(t, 0, prov.calls, "the fallback is off: failure data never reaches the LLM")

	res := FailedResult(err, deps)
	require.Equal(t, models.DecisionStatusFailed, res.DecisionStatus)
	require.Equal(t, models.AnalysisEngineTypeSafe, res.Engine)
	require.Equal(t, "jev-latest", res.ModelName)
	require.Equal(t, PolicyVersion, res.PolicyVersion, "lands in the same compare column as the decisions")
	require.Equal(t, "rate_limit", res.ErrorCategory)
	require.True(t, strings.HasPrefix(res.Summary, "analysis failed: "))
}

func TestAnalyze_NoNarratorAndTypeSafeDownIsAFailedAttempt(t *testing.T) {
	tsErr := &typesafe.Error{Category: typesafe.CategoryNetwork, Message: "dial"}
	_, err := Analyze(context.Background(), AnalyzeDeps{Decider: fixedDecider{err: tsErr}}, baseContext())
	require.ErrorIs(t, err, ErrTypeSafeUnavailable, "TypeSafe-only: an outage is a failed attempt, not an LLM call")
}

func TestAnalyze_TakeoverRecordsTypeSafeDecision(t *testing.T) {
	unsure := flakyDecision()
	unsure.Verdict, unsure.VerdictConfidence = models.VerdictUnknown, 0.48
	prov := &stubProvider{responses: []string{goodVerdict}}
	out, err := Analyze(context.Background(), AnalyzeDeps{Narrative: prov, NarrativeModel: "m", Decider: fixedDecider{d: unsure}, EscalateBelow: 0.90}, baseContext())
	require.NoError(t, err)
	require.Equal(t, models.AnalysisEngineGenerative, out.Engine)
	require.Equal(t, models.DecisionStatusOK, out.DecisionStatus)
	require.Equal(t, models.VerdictUnknown, out.TakeoverFromVerdict)
	require.InDelta(t, 0.48, *out.TakeoverFromConfidence, 1e-9)
	require.Equal(t, "automation_bug", out.TakeoverFromDefectType)
	require.Equal(t, 1, out.LLMCalls)
}

func TestAnalyze_GenerativeUnparseableIsAFailedDecision(t *testing.T) {
	prov := &stubProvider{responses: []string{"nope", "still nope"}}
	out, err := Analyze(context.Background(), AnalyzeDeps{Narrative: prov, NarrativeModel: "m"}, baseContext())
	require.NoError(t, err)
	require.Equal(t, models.DecisionStatusFailed, out.DecisionStatus, "two unreadable replies are not the model saying unknown")
	require.Equal(t, "unparseable", out.ErrorCategory)
	require.Equal(t, 2, out.LLMCalls)
}

func TestAnalyze_TemplateSystemBlockTravelsAsSystemMessage(t *testing.T) {
	prov := &recordingProvider{stubProvider: stubProvider{responses: []string{goodVerdict}}}
	_, err := Analyze(context.Background(), AnalyzeDeps{Narrative: prov, NarrativeModel: "m"}, baseContext())
	require.NoError(t, err)
	msgs := prov.reqs[0].Messages
	require.Len(t, msgs, 2)
	require.Equal(t, "system", msgs[0].Role)
	require.Contains(t, msgs[0].Content, "UNTRUSTED DATA")
	require.NotContains(t, msgs[0].Content, "expected 401, got 500", "no evidence in the system message")
	require.Equal(t, "user", msgs[1].Role)
	require.Contains(t, msgs[1].Content, "expected 401, got 500")
	require.NotContains(t, msgs[1].Content, "SYSTEM:")
}

func TestSplitSystemPrompt(t *testing.T) {
	sys, user := SplitSystemPrompt("SYSTEM:\nrules here\n\nUSER:\nthe data")
	require.Equal(t, "rules here", sys)
	require.Equal(t, "the data", user)

	for _, p := range []string{"no markers at all", "SYSTEM:\nonly rules", "USER:\nonly data", "SYSTEM:\n\nUSER:\ndata"} {
		sys, user := SplitSystemPrompt(p)
		require.Equal(t, "", sys, p)
		require.Equal(t, p, user, p)
	}
}

func TestAnalyze_UnavailableReasonReachesSummary(t *testing.T) {
	out, err := Analyze(context.Background(), AnalyzeDeps{Decider: fixedDecider{d: flakyDecision()},
		LLMUnavailableReason: "the default LLM provider is not approved for automatic analysis"}, baseContext())
	require.NoError(t, err)
	require.Equal(t, models.NarrativeStatusUnavailable, out.NarrativeStatus)
	require.Contains(t, out.Summary, "not approved for automatic analysis")
}

func TestAnalyze_RecordsDecisionAndCallTiming(t *testing.T) {
	prov := &finishProvider{replies: [][2]string{{`{"summary":"S","next_action":"N","rationale":"R"}`, "stop"}}}
	out, err := Analyze(context.Background(), AnalyzeDeps{Narrative: prov, NarrativeModel: "m", Decider: fixedDecider{d: flakyDecision()}}, baseContext())
	require.NoError(t, err)
	require.Equal(t, 1, out.LLMCalls)
	require.Equal(t, "stop", out.FinishReason)
	require.GreaterOrEqual(t, out.DecisionMs, 0)
}

func TestExplain_NarratesTheStoredDecision(t *testing.T) {
	score := 0.93
	stored := &models.RunResultAnalysis{Engine: models.AnalysisEngineTypeSafe, Verdict: models.VerdictFlakyTest,
		Confidence: models.ConfidenceHigh, ConfidenceScore: &score, SuggestedDefectType: "automation_bug",
		VerdictProbabilities: `{"flaky_test":0.52,"product_bug":0.45}`, NarrativeStatus: models.NarrativeStatusSkipped}
	prov := &recordingProvider{stubProvider: stubProvider{responses: []string{`{"summary":"S","next_action":"N","rationale":"R"}`}}}
	out, err := Explain(context.Background(), AnalyzeDeps{Narrative: prov, NarrativeModel: "m"}, baseContext(), stored)
	require.NoError(t, err)
	require.Equal(t, models.NarrativeStatusOK, out.NarrativeStatus)
	require.Equal(t, "S", out.Summary)
	sys := prov.reqs[0].Messages[0].Content
	require.Contains(t, sys, "`flaky_test` (confidence 0.93)")
	require.Contains(t, sys, "runner-up verdict was `product_bug`", "stored probabilities feed the runner-up hint")
	require.Contains(t, sys, "at most two pieces of evidence", "the short contract is used")

	_, err = Explain(context.Background(), AnalyzeDeps{}, baseContext(), stored)
	require.ErrorIs(t, err, ErrNoNarrator)
}

func TestBuildEvidence_RedactsEveryOutgoingField(t *testing.T) {
	in := baseContext()
	in.Result.TestNameSnapshot = "Login as jane.doe@example.com"
	in.Env = "staging token=abcdef123456"
	in.Categories = "owner qa.lead@example.com"
	in.Steps = []PromptStep{{Order: 1, Action: "fill password=hunter2hunter2", Expected: "mail to ops@example.com"}}
	in.LinkedDefects = []LinkedDefect{{Key: "BUG-1", Status: "open", Summary: "reported by dev@example.com"}}
	in.LinkedRequirements = []LinkedRequirement{{Key: "REQ-1", Title: "api_key=zzzzzzzzzzzz must rotate"}}
	ev := BuildEvidence(in)
	all := strings.Join([]string{ev.TestName, ev.Env, ev.Categories, ev.Steps[0].Action, ev.Steps[0].Expected,
		ev.LinkedDefects[0].Summary, ev.LinkedRequirements[0].Title}, " | ")
	for _, secret := range []string{"jane.doe@example.com", "abcdef123456", "qa.lead@example.com", "hunter2hunter2", "ops@example.com", "dev@example.com", "zzzzzzzzzzzz"} {
		require.NotContains(t, all, secret)
	}
	require.Equal(t, "Login as jane.doe@example.com", in.Result.TestNameSnapshot, "the caller's result is not mutated")

	in.RedactionEnabled = false
	require.Contains(t, BuildEvidence(in).TestName, "jane.doe@example.com", "redaction off leaves fields as they are")
}

func TestPipelineLabel(t *testing.T) {
	cases := map[string]Pipeline{
		"minimax":                            {Narrator: "minimax"},
		"TypeSafe jev, no LLM":               {Decider: "jev"},
		"TypeSafe jev, explained by minimax": {Decider: "jev", Narrator: "minimax", Explanations: true},
		"TypeSafe jev, decisions only, minimax below 90%": {Decider: "jev", Narrator: "minimax", TakeoverBelowPct: 90},
	}
	for want, p := range cases {
		require.Equal(t, want, p.Label())
	}
}

func TestErrCategory_DeadlineIsTimeout(t *testing.T) {
	require.Equal(t, "timeout", errCategory(context.DeadlineExceeded))
	require.Equal(t, "timeout", errCategory(errors.Join(errors.New("llm call"), context.DeadlineExceeded)))
	require.Equal(t, "rate_limit", errCategory(&llm.ProviderError{Category: llm.ErrCatRateLimit}))
}
