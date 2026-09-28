package failureanalysis

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/typesafe"

	"github.com/stretchr/testify/require"
)

func TestDecider_MapsAnswersAndSendsBothQuestions(t *testing.T) {
	fc := &fakeClient{fn: func(req typesafe.Request) (*typesafe.Response, error) {
		return decisionResponse(models.VerdictFlakyTest, 0.93, 0.95, "automation_bug", 0.88, 0.9), nil
	}}
	d, err := NewTypeSafeDecider(fc, "jev-1.13.0").Decide(context.Background(), BuildEvidence(baseContext()))
	require.NoError(t, err)
	require.Equal(t, models.VerdictFlakyTest, d.Verdict)
	require.InDelta(t, 0.93, d.VerdictConfidence, 1e-9)
	require.Equal(t, "automation_bug", d.SuggestedDefectType)
	require.InDelta(t, 0.88, d.DefectTypeConfidence, 1e-9)
	require.Equal(t, "jev-1.13.0", d.Model)
	require.Equal(t, 777, d.InputTokens)
	require.Equal(t, PolicyVersionNoExamples, d.PolicyVersion)
	require.Len(t, d.VerdictProbabilities, 6)
	require.Len(t, d.DefectTypeProbabilities, 4)

	require.Len(t, fc.calls, 1)
	req := fc.calls[0]
	require.Equal(t, "jev-1.13.0", req.Model)
	require.Contains(t, req.Questions, "verdict")
	require.Contains(t, req.Questions, "defect_type")
	b, _ := json.Marshal(req.State)
	require.Contains(t, string(b), "expected 401, got 500", "state carries the failure")
}

func TestDecider_AbstainsOnInsufficientEvidence(t *testing.T) {
	fc := &fakeClient{fn: func(req typesafe.Request) (*typesafe.Response, error) {
		return decisionResponse(models.VerdictUnknown, 0.6, 0.7, DefectTypeInsufficient, 0.95, 0.96), nil
	}}
	d, err := NewTypeSafeDecider(fc, "m").Decide(context.Background(), BuildEvidence(baseContext()))
	require.NoError(t, err)
	require.Equal(t, "", d.SuggestedDefectType)
	require.InDelta(t, 0.95, d.DefectTypeConfidence, 1e-9, "confidence is kept even when abstaining")
}

func TestDecider_AbstainsBelowThreshold(t *testing.T) {
	fc := &fakeClient{fn: func(req typesafe.Request) (*typesafe.Response, error) {
		return decisionResponse(models.VerdictProductBug, 0.7, 0.8, "product_bug", DefectTypeSuggestMin-0.01, 0.5), nil
	}}
	d, err := NewTypeSafeDecider(fc, "m").Decide(context.Background(), BuildEvidence(baseContext()))
	require.NoError(t, err)
	require.Equal(t, "", d.SuggestedDefectType)
	require.Equal(t, models.VerdictProductBug, d.Verdict, "verdict is independent of the abstention")
}

func TestDecider_UnknownVerdictWithSuggestionIsValid(t *testing.T) {
	fc := &fakeClient{fn: func(req typesafe.Request) (*typesafe.Response, error) {
		return decisionResponse(models.VerdictUnknown, 0.5, 0.6, "automation_bug", 0.9, 0.9), nil
	}}
	d, err := NewTypeSafeDecider(fc, "m").Decide(context.Background(), BuildEvidence(baseContext()))
	require.NoError(t, err)
	require.Equal(t, models.VerdictUnknown, d.Verdict)
	require.Equal(t, "automation_bug", d.SuggestedDefectType)
}

func TestDecider_PropagatesClientErrors(t *testing.T) {
	want := &typesafe.Error{Category: typesafe.CategoryRateLimit, Status: 429}
	fc := &fakeClient{fn: func(req typesafe.Request) (*typesafe.Response, error) { return nil, want }}
	_, err := NewTypeSafeDecider(fc, "m").Decide(context.Background(), BuildEvidence(baseContext()))
	require.True(t, errors.Is(err, want))
}

func TestDecider_ShrinksAndRetriesOnceOnOversized(t *testing.T) {
	fc := &fakeClient{}
	fc.fn = func(req typesafe.Request) (*typesafe.Response, error) {
		if len(fc.calls) == 1 {
			return nil, &typesafe.Error{Status: 422, Oversized: true, Message: "context limit exceeded"}
		}
		return decisionResponse("product_bug", 0.95, 0.9, "product_bug", 0.9, 0.9), nil
	}
	in := baseContext()
	in.Result.LogText = strings.Repeat("l", 40000)
	ev := BuildEvidenceWithBudget(in, TypeSafeBudget())
	d, err := NewTypeSafeDecider(fc, "jev").Decide(context.Background(), ev)
	require.NoError(t, err)
	require.Equal(t, "product_bug", d.Verdict)
	require.Len(t, fc.calls, 2, "one shrink-and-retry")
	first := fc.calls[0].State.(map[string]any)["failure"].(map[string]any)["log_tail"].(string)
	second := fc.calls[1].State.(map[string]any)["failure"].(map[string]any)["log_tail"].(string)
	require.Len(t, first, 40000)
	require.Less(t, len(second), len(first), "the retry sends a smaller state")

	always := &fakeClient{fn: func(typesafe.Request) (*typesafe.Response, error) {
		return nil, &typesafe.Error{Status: 422, Oversized: true, Message: "context limit exceeded"}
	}}
	_, err = NewTypeSafeDecider(always, "jev").Decide(context.Background(), ev)
	var te *typesafe.Error
	require.ErrorAs(t, err, &te)
	require.True(t, te.Oversized)
	require.Len(t, always.calls, 2, "no endless retry")
}

func TestDecider_DerivesTheSuggestionFromAConfidentVerdictWhenTheQuestionAbstains(t *testing.T) {
	decide := func(verdict string, vconf float64, defect string, dconf float64) *Decision {
		fc := &fakeClient{fn: func(typesafe.Request) (*typesafe.Response, error) {
			return decisionResponse(verdict, vconf, 0.9, defect, dconf, 0.8), nil
		}}
		d, err := NewTypeSafeDecider(fc, "jev").Decide(context.Background(), BuildEvidence(baseContext()))
		require.NoError(t, err)
		return d
	}
	// The question abstains, the verdict is confident: derive through the same mapping the LLM path uses.
	d := decide("flaky_test", 0.95, DefectTypeInsufficient, 0.85)
	require.Equal(t, "automation_bug", d.SuggestedDefectType)
	require.Equal(t, models.SuggestionSourceVerdict, d.SuggestionSource)
	require.Equal(t, 0.95, d.DefectTypeConfidence, "a derived suggestion carries the verdict's confidence")

	// Below the gate on the question, same rule.
	d = decide("environment", 0.92, "system_issue", 0.30)
	require.Equal(t, "system_issue", d.SuggestedDefectType)
	require.Equal(t, models.SuggestionSourceVerdict, d.SuggestionSource)

	// An unknown verdict derives nothing; neither does a verdict below VerdictDecidesSuggestionMin.
	d = decide("unknown", 0.95, DefectTypeInsufficient, 0.85)
	require.Empty(t, d.SuggestedDefectType)
	require.Empty(t, d.SuggestionSource)
	d = decide("product_bug", 0.80, DefectTypeInsufficient, 0.85)
	require.Empty(t, d.SuggestedDefectType)
	require.Empty(t, d.SuggestionSource)

	// A question that answered keeps its own answer and confidence, unmarked.
	d = decide("product_bug", 0.95, "product_bug", 0.7)
	require.Equal(t, "product_bug", d.SuggestedDefectType)
	require.Empty(t, d.SuggestionSource)
	require.Equal(t, 0.7, d.DefectTypeConfidence)
}

// TestSuggestion_VerdictDecidesWhereItIsSure replays the benchmark answers that motivated policy
// v5 (numbers from the 2026-09-22 decisions-only pass), plus the rule's other branches.
func TestSuggestion_VerdictDecidesWhereItIsSure(t *testing.T) {
	ans := func(choice string, conf float64) typesafe.Answer {
		return typesafe.Answer{Type: "choice", Choice: choice, Confidence: conf}
	}
	cases := []struct {
		name           string
		verdict        typesafe.Answer
		defect         typesafe.Answer
		want, wantFrom string
		wantConf       float64
	}{
		// Runner out of memory: v4 withheld (verdict under 0.90, question at 0.22); v5 derives.
		{"confident verdict fills a withheld question", ans("infrastructure", 0.88), ans("system_issue", 0.22), "system_issue", models.SuggestionSourceVerdict, 0.88},
		// WebSocket close: the question named a system source the verdict could not pick.
		{"unknown verdict drops a system source", ans("unknown", 0.52), ans("system_issue", 0.77), "", "", 0.77},
		{"unknown verdict drops a product source", ans("unknown", 0.60), ans("product_bug", 0.90), "", "", 0.90},
		{"unknown verdict keeps a test-code fault", ans("unknown", 0.60), ans("automation_bug", 0.80), "automation_bug", "", 0.80},
		// Rate-limit quota: verdict unsure between infrastructure and environment, question sure of the bucket.
		{"unsure verdict leaves the question's answer", ans("infrastructure", 0.51), ans("system_issue", 0.85), "system_issue", "", 0.85},
		{"unsure verdict does not override a disagreeing answer", ans("infrastructure", 0.60), ans("automation_bug", 0.70), "automation_bug", "", 0.70},
		{"confident verdict overrides a disagreeing answer", ans("flaky_test", 0.95), ans("product_bug", 0.80), "automation_bug", models.SuggestionSourceVerdict, 0.95},
		{"agreeing answer keeps its own confidence", ans("product_bug", 1.0), ans("product_bug", 0.94), "product_bug", "", 0.94},
		{"threshold is inclusive", ans("environment", VerdictDecidesSuggestionMin), ans(DefectTypeInsufficient, 0.9), "system_issue", models.SuggestionSourceVerdict, VerdictDecidesSuggestionMin},
		{"just under the threshold withholds as before", ans("environment", 0.84), ans(DefectTypeInsufficient, 0.9), "", "", 0.9},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, from, conf := suggestion(c.verdict, c.defect)
			require.Equal(t, c.want, got)
			require.Equal(t, c.wantFrom, from)
			require.InDelta(t, c.wantConf, conf, 1e-9)
		})
	}
	require.Less(t, VerdictDecidesSuggestionMin, VerdictHighMin, "v5 lets a verdict below the high bucket decide")
	require.Equal(t, "fa-verdict-v5", PolicyVersionNoExamples, "a rule change is a new policy, so calibration can be read per policy")
	require.Equal(t, "fa-verdict-v6", PolicyVersionWithExamples, "examples in the state are a different input, so a different policy")
	require.Equal(t, PolicyVersionNoExamples, PolicyVersionFor(0))
	require.Equal(t, PolicyVersionWithExamples, PolicyVersionFor(1))
}

func TestDecider_StampsThePolicyFromTheExamplesSent(t *testing.T) {
	var states []map[string]any
	fc := &fakeClient{fn: func(req typesafe.Request) (*typesafe.Response, error) {
		states = append(states, req.State.(map[string]any))
		return decisionResponse(models.VerdictFlakyTest, 0.93, 0.95, "automation_bug", 0.88, 0.9), nil
	}}
	dec := NewTypeSafeDecider(fc, "jev-1.13.0")

	d, err := dec.Decide(context.Background(), BuildEvidenceWithBudget(baseContext(), TypeSafeBudget()))
	require.NoError(t, err)
	require.Equal(t, PolicyVersionNoExamples, d.PolicyVersion)
	require.NotContains(t, states[0], "examples")

	in := baseContext()
	in.Examples = []TriageExample{{FailureType: "timeout", ErrorMessage: "spinner", SuggestedDefectType: "product_bug", HumanDefectType: "automation_bug", Corrected: true}}
	d, err = dec.Decide(context.Background(), BuildEvidenceWithBudget(in, TypeSafeBudget()))
	require.NoError(t, err)
	require.Equal(t, PolicyVersionWithExamples, d.PolicyVersion)
	require.Len(t, states[1]["examples"], 1)

	// The ladder dropped every example: the decision was made without them, so it is v5.
	ev := BuildEvidenceWithBudget(in, TypeSafeBudget())
	noExamples := ev
	noExamples.Examples = nil
	b, _ := json.Marshal(stateObject(noExamples))
	ev.StateCap = len(b)
	d, err = dec.Decide(context.Background(), ev)
	require.NoError(t, err)
	require.Equal(t, PolicyVersionNoExamples, d.PolicyVersion, "stamped from what was sent, not what was built")
	require.NotContains(t, states[2], "examples")
}
