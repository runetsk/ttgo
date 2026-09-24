package failureanalysis

import (
	"context"
	"strings"
	"testing"
	"time"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/typesafe"

	"github.com/stretchr/testify/require"
)

// waitProvider answers only when its context ends, like an LLM that outlasts the deadline.
type waitProvider struct{ calls int }

func (p *waitProvider) Chat(ctx context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	p.calls++
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestAnalyze_DeadlineDuringTakeoverKeepsTypeSafesDecision(t *testing.T) {
	unsure := flakyDecision()
	unsure.Verdict, unsure.VerdictConfidence = models.VerdictUnknown, 0.48
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	out, err := Analyze(ctx, AnalyzeDeps{Narrative: &waitProvider{}, NarrativeModel: "m", Decider: fixedDecider{d: unsure}, EscalateBelow: 0.90}, baseContext())
	require.NoError(t, err, "running out of time after TypeSafe decided is not a failed attempt")
	require.Equal(t, models.AnalysisEngineTypeSafe, out.Engine)
	require.Equal(t, models.DecisionStatusOK, out.DecisionStatus)
	require.Equal(t, models.VerdictUnknown, out.Verdict)
	require.Equal(t, "timeout", out.ErrorCategory)
	require.Equal(t, models.NarrativeStatusUnavailable, out.NarrativeStatus)
	require.Equal(t, 777, out.TypeSafeInputTokens, "TypeSafe's accounting is kept")
}

func TestAnalyze_DeadlineDuringExplanationKeepsTheDecision(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	out, err := Analyze(ctx, AnalyzeDeps{Narrative: &waitProvider{}, NarrativeModel: "m", Decider: fixedDecider{d: flakyDecision()}}, baseContext())
	require.NoError(t, err)
	require.Equal(t, models.AnalysisEngineTypeSafe, out.Engine)
	require.Equal(t, models.VerdictFlakyTest, out.Verdict)
	require.Equal(t, models.NarrativeStatusUnavailable, out.NarrativeStatus)
	require.Contains(t, out.Summary, "timeout")
}

func TestAnalyze_CancelStillAbandonsTheAnalysis(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(30 * time.Millisecond); cancel() }()
	_, err := Analyze(ctx, AnalyzeDeps{Narrative: &waitProvider{}, NarrativeModel: "m", Decider: fixedDecider{d: flakyDecision()}}, baseContext())
	require.ErrorIs(t, err, context.Canceled, "a cancelled job or a closed request stores nothing")
}

func TestUnavailableDecider_AppliesTheFallbackSwitch(t *testing.T) {
	cfgErr := &typesafe.Error{Category: typesafe.CategoryConfiguration, Message: "no key"}
	deps := AnalyzeDeps{Decider: NewUnavailableDecider(cfgErr), DeciderModel: "jev-latest", NoLLMFallback: true}
	_, err := Analyze(context.Background(), deps, baseContext())
	require.ErrorIs(t, err, ErrTypeSafeUnavailable)
	res := FailedResult(err, deps)
	require.Equal(t, models.AnalysisEngineTypeSafe, res.Engine)
	require.Equal(t, "configuration", res.ErrorCategory)

	prov := &stubProvider{responses: []string{`{"verdict":"environment","confidence":"high","summary":"s","next_action":"n","rationale":"r"}`}}
	out, err := Analyze(context.Background(), AnalyzeDeps{Narrative: prov, NarrativeModel: "m", Decider: NewUnavailableDecider(cfgErr)}, baseContext())
	require.NoError(t, err)
	require.Equal(t, models.AnalysisEngineGenerative, out.Engine)
	require.True(t, strings.HasPrefix(out.Rationale, "[verdict engine: TypeSafe unavailable (configuration); used generative]"),
		"with the fallback on the analysis says TypeSafe was unavailable: %q", out.Rationale)
}

func TestExcerpt_RedactsFailureType(t *testing.T) {
	g := &FailureGroup{Key: Signature("assertion", "boom"), Representative: &models.RunResult{
		TestNameSnapshot: "Login as jane.doe@example.com", FailureType: "token=abcdef123456abcdef",
		ErrorMessage: "password=hunter2hunter2", StackTrace: "api_key=zzzzzzzzzzzz",
	}}
	ex := excerpt(g, true)
	for _, field := range []string{"test_name", "failure_type", "error_message", "stack_head"} {
		v := ex[field].(string)
		for _, secret := range []string{"jane.doe@example.com", "abcdef123456abcdef", "hunter2hunter2", "zzzzzzzzzzzz"} {
			require.NotContains(t, v, secret, field)
		}
	}
	require.Equal(t, g.Key, ex["id"], "the id is the signature hash, not failure text")
	require.Contains(t, excerpt(g, false)["failure_type"], "abcdef123456abcdef", "redaction off leaves it as recorded")
}
