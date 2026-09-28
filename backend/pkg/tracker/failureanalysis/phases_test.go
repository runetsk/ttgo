package failureanalysis

import (
	"context"
	"errors"
	"testing"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/typesafe"

	"github.com/stretchr/testify/require"
)

// TestDecideNarrate_RouteTable walks every row of spec §A1's route table that ends in a
// decision through Decide and then Narrate. The rows that end in an error are
// TestDecide_FailedRoutesReturnAStoreableResult.
func TestDecideNarrate_RouteTable(t *testing.T) {
	tsDown := &typesafe.Error{Category: typesafe.CategoryRateLimit, Status: 429, Message: "slow down"}
	unsure := func(verdict string, conf float64) *Decision {
		d := flakyDecision()
		d.Verdict, d.VerdictConfidence = verdict, conf
		return d
	}
	cases := []struct {
		name            string
		prov            *stubProvider // nil = no narrator attached
		deps            AnalyzeDeps   // Narrative and NarrativeModel are filled in below
		engine          string
		narrative       string
		category        string
		decideCalls     int
		narrates        bool
		summary         string // substring of the decided Summary; "" = empty
		rationalePrefix string
	}{
		{name: "TypeSafe decides, explanations on, narrator present",
			prov: &stubProvider{responses: []string{narrJSON}}, deps: AnalyzeDeps{Decider: fixedDecider{d: flakyDecision()}},
			engine: models.AnalysisEngineTypeSafe, narrative: models.NarrativeStatusPending, narrates: true},
		{name: "TypeSafe decides, explanations off",
			prov: &stubProvider{responses: []string{narrJSON}}, deps: AnalyzeDeps{Decider: fixedDecider{d: flakyDecision()}, NarrativeSkipped: true},
			engine: models.AnalysisEngineTypeSafe, narrative: models.NarrativeStatusSkipped, summary: "switched off"},
		{name: "TypeSafe decides, no narrator",
			deps:   AnalyzeDeps{Decider: fixedDecider{d: flakyDecision()}, LLMUnavailableReason: "the LLM key cannot be decrypted"},
			engine: models.AnalysisEngineTypeSafe, narrative: models.NarrativeStatusUnavailable,
			summary: "Narrative unavailable: the LLM key cannot be decrypted."},
		{name: "takeover succeeded",
			prov: &stubProvider{responses: []string{goodVerdict}}, deps: AnalyzeDeps{Decider: fixedDecider{d: unsure(models.VerdictUnknown, 0.48)}, EscalateBelow: 0.90},
			engine: models.AnalysisEngineGenerative, narrative: models.NarrativeStatusOK, decideCalls: 1, summary: "s",
			rationalePrefix: "[verdict engine: TypeSafe unsure (unknown at 0.48, below 0.90); the LLM decided] "},
		{name: "takeover failed",
			prov:   &stubProvider{responses: []string{""}, errs: []error{&llm.ProviderError{Category: llm.ErrCatTimeout, Message: "t"}}},
			deps:   AnalyzeDeps{Decider: fixedDecider{d: unsure(models.VerdictFlakyTest, 0.60)}, EscalateBelow: 0.90},
			engine: models.AnalysisEngineTypeSafe, narrative: models.NarrativeStatusUnavailable, category: "timeout",
			decideCalls: 1, summary: "(timeout)"},
		{name: "takeover unparseable",
			prov:   &stubProvider{responses: []string{"nope", "still nope"}},
			deps:   AnalyzeDeps{Decider: fixedDecider{d: unsure(models.VerdictFlakyTest, 0.60)}, EscalateBelow: 0.90},
			engine: models.AnalysisEngineTypeSafe, narrative: models.NarrativeStatusUnavailable, category: "unparseable",
			decideCalls: 2, summary: "(unparseable)"},
		{name: "TypeSafe unavailable, fallback on",
			prov: &stubProvider{responses: []string{goodVerdict}}, deps: AnalyzeDeps{Decider: fixedDecider{err: tsDown}},
			engine: models.AnalysisEngineGenerative, narrative: models.NarrativeStatusOK, decideCalls: 1, summary: "s",
			rationalePrefix: "[verdict engine: TypeSafe unavailable (rate_limit); used generative] "},
		{name: "generative-only",
			prov:   &stubProvider{responses: []string{goodVerdict}},
			engine: models.AnalysisEngineGenerative, narrative: models.NarrativeStatusOK, decideCalls: 1, summary: "s"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			deps := c.deps
			deps.NarrativeModel = "gpt-test"
			if c.prov != nil {
				deps.Narrative = c.prov
			}
			in := parityContext()
			res, err := Decide(context.Background(), deps, in)
			require.NoError(t, err)
			require.Equal(t, c.engine, res.Engine)
			require.Equal(t, c.narrative, res.NarrativeStatus)
			require.Equal(t, models.DecisionStatusOK, res.DecisionStatus)
			require.Equal(t, c.category, res.ErrorCategory)
			require.True(t, res.HistoryAvailable)
			if c.summary == "" {
				require.Empty(t, res.Summary)
			} else {
				require.Contains(t, res.Summary, c.summary)
			}
			if c.rationalePrefix != "" {
				require.Contains(t, res.Rationale, c.rationalePrefix)
			}
			calls := 0
			if c.prov != nil {
				calls = c.prov.calls
			}
			require.Equal(t, c.decideCalls, calls, "LLM calls made by Decide")

			before := *res
			d, ok := Narrate(context.Background(), deps, in, res)
			require.Equal(t, c.narrates, ok)
			require.Equal(t, before, *res, "Narrate never changes the decided result")
			if !c.narrates {
				require.Equal(t, NarrationDelta{}, d)
				if c.prov != nil {
					require.Equal(t, c.decideCalls, c.prov.calls, "a final decision is never narrated (no second LLM call)")
				}
				return
			}
			require.Equal(t, 1, c.prov.calls)
			require.Equal(t, models.NarrativeStatusOK, d.NarrativeStatus)
			require.Equal(t, "S", d.Summary)
			require.Equal(t, "N", d.NextAction)
			require.Equal(t, "R", d.Rationale)
			require.Equal(t, narrJSON, d.RawResponse)
			require.Equal(t, 100, d.PromptTokens)
			require.Equal(t, 20, d.CompletionTokens)
			require.Equal(t, 1, d.LLMCalls)
			require.Empty(t, d.Reason)
		})
	}
}

// pendingDecision is a TypeSafe decision waiting for its explanation.
func pendingDecision() *AnalyzeResult {
	res := decisionResult(flakyDecision())
	res.NarrativeStatus = models.NarrativeStatusPending
	res.DecisionMs, res.TypeSafeInputTokens = 7, 777
	return res
}

func TestNarrate_OnlyForPending(t *testing.T) {
	for _, status := range []string{models.NarrativeStatusOK, models.NarrativeStatusSkipped,
		models.NarrativeStatusUnavailable, models.NarrativeStatusUnparseable, ""} {
		prov := &stubProvider{responses: []string{narrJSON}}
		res := pendingDecision()
		res.NarrativeStatus = status
		d, ok := Narrate(context.Background(), AnalyzeDeps{Narrative: prov}, baseContext(), res)
		require.False(t, ok, status)
		require.Equal(t, NarrationDelta{}, d, status)
		require.Equal(t, 0, prov.calls, status)
	}
	_, ok := Narrate(context.Background(), AnalyzeDeps{Narrative: &stubProvider{}}, baseContext(), nil)
	require.False(t, ok)
}

func TestNarrate_DeltaCarriesOnlyNarration(t *testing.T) {
	prov := &finishProvider{replies: [][2]string{{narrJSON, "stop"}}}
	d, ok := Narrate(context.Background(), AnalyzeDeps{Narrative: prov, NarrativeModel: "m"}, baseContext(), pendingDecision())
	require.True(t, ok)
	require.Equal(t, NarrationDelta{Summary: "S", NextAction: "N", Rationale: "R", RawResponse: narrJSON, FinishReason: "stop",
		NarrativeStatus: models.NarrativeStatusOK, PromptTokens: 100, CompletionTokens: ReplyTokenCap,
		LLMCalls: 1, LLMMs: d.LLMMs}, d)
	sys := prov.reqs[0].Messages[0].Content
	require.Contains(t, sys, "`flaky_test` (confidence 0.93)", "the decision is rebuilt from the decided result")
	require.Contains(t, sys, "`automation_bug`")
}

func TestNarrate_FailuresAreDeltasNotErrors(t *testing.T) {
	boom := errors.New("boom")
	cases := []struct {
		name, tmpl              string
		prov                    llm.Provider
		status, reason, summary string
		calls                   int
	}{
		{name: "first call fails", prov: &stubProvider{responses: []string{""}, errs: []error{boom}},
			status: models.NarrativeStatusUnavailable, reason: "error", summary: "AI narrative unavailable: error", calls: 1},
		{name: "timeout", prov: &stubProvider{responses: []string{""}, errs: []error{&llm.ProviderError{Category: llm.ErrCatTimeout, Message: "t"}}},
			status: models.NarrativeStatusUnavailable, reason: "timeout", summary: "AI narrative unavailable: timeout", calls: 1},
		{name: "repair call fails", prov: &stubProvider{responses: []string{"nope", ""}, errs: []error{nil, boom}},
			status: models.NarrativeStatusUnavailable, reason: "error", summary: "AI narrative unavailable: error", calls: 2},
		{name: "unparseable twice", prov: &stubProvider{responses: []string{"nope", "still nope"}},
			status: models.NarrativeStatusUnparseable, reason: "unparseable", summary: "AI narrative unavailable: unparseable response", calls: 2},
		{name: "cut off twice", prov: &finishProvider{replies: [][2]string{{`{"summary":"The`, "length"}, {`{"summary":"The`, "length"}}},
			status: models.NarrativeStatusUnavailable, reason: "truncated", summary: "AI narrative unavailable: truncated", calls: 2},
		{name: "template error", tmpl: "{{ .Broken", prov: &stubProvider{responses: []string{narrJSON}},
			status: models.NarrativeStatusUnavailable, reason: "template error", summary: "AI narrative unavailable: template error"},
		{name: "no narrator",
			status: models.NarrativeStatusUnavailable, reason: "no generative provider is configured",
			summary: "AI narrative unavailable: no generative provider is configured"},
	}
	for _, c := range cases {
		in := baseContext()
		if c.tmpl != "" {
			in.PromptTemplate = c.tmpl
		}
		deps := AnalyzeDeps{NarrativeModel: "m"}
		if c.prov != nil {
			deps.Narrative = c.prov
		}
		d, ok := Narrate(context.Background(), deps, in, pendingDecision())
		require.True(t, ok, c.name)
		require.Equal(t, c.status, d.NarrativeStatus, c.name)
		require.Equal(t, c.reason, d.Reason, c.name)
		require.Equal(t, c.summary, d.Summary, c.name)
		require.Equal(t, c.calls, d.LLMCalls, c.name)
		require.Empty(t, d.NextAction, c.name)
	}
}

func TestApplyNarration_AddsUsageAndKeepsTheDecision(t *testing.T) {
	res := pendingDecision()
	res.TokenUsagePrompt, res.LLMCalls, res.LLMMs, res.FinishReason = 50, 1, 10, "stop"
	res.ApplyNarration(NarrationDelta{Summary: "S", NextAction: "N", Rationale: "R", RawResponse: narrJSON,
		NarrativeStatus: models.NarrativeStatusOK, PromptTokens: 100, CompletionTokens: 20, LLMMs: 30, LLMCalls: 2})
	require.Equal(t, models.VerdictFlakyTest, res.Verdict)
	require.Equal(t, models.AnalysisEngineTypeSafe, res.Engine)
	require.Equal(t, "automation_bug", res.SuggestedDefectType)
	require.Equal(t, 777, res.TypeSafeInputTokens)
	require.Equal(t, 7, res.DecisionMs)
	require.Equal(t, models.NarrativeStatusOK, res.NarrativeStatus)
	require.Equal(t, "S", res.Summary)
	require.Equal(t, "N", res.NextAction)
	require.Equal(t, "R", res.Rationale)
	require.Equal(t, narrJSON, res.RawResponse)
	require.Equal(t, 150, res.TokenUsagePrompt)
	require.Equal(t, 20, res.TokenUsageCompletion)
	require.Equal(t, 3, res.LLMCalls)
	require.Equal(t, 40, res.LLMMs)
	require.Equal(t, "stop", res.FinishReason, "an empty finish reason keeps the last one")

	res.ApplyNarration(NarrationDelta{NarrativeStatus: models.NarrativeStatusUnavailable,
		Summary: "AI narrative unavailable: timeout", FinishReason: "length"})
	require.Empty(t, res.NextAction)
	require.Empty(t, res.Rationale)
	require.Empty(t, res.RawResponse)
	require.Equal(t, "length", res.FinishReason)
}
