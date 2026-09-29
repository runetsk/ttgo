package failureanalysis

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"ttgo/pkg/tracker/typesafe"

	"github.com/stretchr/testify/require"
)

func answeringClient() *fakeClient {
	return &fakeClient{fn: func(req typesafe.Request) (*typesafe.Response, error) { return companionResponse(req, nil, "", 0), nil }}
}

func TestDecider_AsksCompanionsOnlyWhenTheirInputsExist(t *testing.T) {
	fc := answeringClient()
	dec := NewTypeSafeDecider(fc, "jev")
	asked := func(ev Evidence) []string {
		_, err := dec.Decide(context.Background(), ev)
		require.NoError(t, err)
		req := fc.calls[len(fc.calls)-1]
		keys := make([]string, 0, len(req.Questions))
		for k := range req.Questions {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return keys
	}
	always := []string{"defect_type", "injection", "outside_app", "verdict"}
	require.Equal(t, always, asked(BuildEvidenceWithBudget(baseContext(), TypeSafeBudget())))

	in := baseContext()
	in.RecentOutcomes = "PF"
	require.Equal(t, always, asked(BuildEvidenceWithBudget(in, TypeSafeBudget())), "two outcomes cannot alternate")

	in.RecentOutcomes = "PFP"
	in.SimilarFailures = []SimilarFailure{{Status: "FAIL", ErrorMessage: "expected 401, got 500"}}
	in.LinkedDefects = []LinkedDefect{{Key: "PAY-42", Status: "open", Summary: "Login answers 500"}}
	require.Equal(t, []string{"defect_type", "flaky_history", "injection", "known_defect", "outside_app", "recurring", "verdict"},
		asked(BuildEvidenceWithBudget(in, TypeSafeBudget())))

	// Inputs the ladder dropped are not asked about.
	ev := BuildEvidenceWithBudget(in, TypeSafeBudget())
	noHistory := ev
	noHistory.SimilarFailures, noHistory.SimilarFailuresRollup, noHistory.RecentOutcomes, noHistory.LogText = nil, "", "", ""
	b, _ := json.Marshal(stateObject(noHistory))
	ev.StateCap = len(b)
	require.Equal(t, []string{"defect_type", "injection", "known_defect", "outside_app", "verdict"}, asked(ev),
		"the linked defects were still sent; the history block was not")
}

func TestDecider_MapsCompanionAnswersIntoSignals(t *testing.T) {
	in := baseContext()
	in.RecentOutcomes = "PFPFP"
	in.SimilarFailures = []SimilarFailure{{Status: "FAIL", ErrorMessage: "expected 401, got 500"}}
	in.LinkedDefects = []LinkedDefect{{Key: "PAY-41", Summary: "Checkout slow"}, {Key: "PAY-42", Summary: "Login answers 500"}}
	fc := &fakeClient{fn: func(req typesafe.Request) (*typesafe.Response, error) {
		return companionResponse(req, map[string]float64{"injection": 0.03, "flaky_history": 0.91, "recurring": 0.97, "outside_app": 0.12},
			"defect_1", 0.88), nil
	}}
	d, err := NewTypeSafeDecider(fc, "jev").Decide(context.Background(), BuildEvidenceWithBudget(in, TypeSafeBudget()))
	require.NoError(t, err)
	require.InDelta(t, 0.03, *d.Signals.Injection, 1e-9)
	require.InDelta(t, 0.91, *d.Signals.FlakyHistory, 1e-9)
	require.InDelta(t, 0.97, *d.Signals.Recurring, 1e-9)
	require.InDelta(t, 0.12, *d.Signals.OutsideApp, 1e-9)
	require.Equal(t, &KnownDefectSignal{Key: "PAY-42", Confidence: 0.88}, d.Signals.KnownDefect)
	require.False(t, d.Signals.InjectionFlagged())
	require.NotEmpty(t, d.Signals.CheckedBlocks, "a guarded decision records what its injection question covered")

	d, err = NewTypeSafeDecider(answeringClient(), "jev").Decide(context.Background(), BuildEvidenceWithBudget(in, TypeSafeBudget()))
	require.NoError(t, err)
	require.Nil(t, d.Signals.KnownDefect, "`none` names no defect")
	require.NotNil(t, d.Signals.Recurring)
}

func TestDecider_KnownDefectMapsBackToTheKeyThatWasSent(t *testing.T) {
	in := baseContext() // redaction on
	in.LinkedDefects = []LinkedDefect{{Key: "PAY-41"}, {Key: "reported by alice@example.com"}}
	fc := &fakeClient{fn: func(req typesafe.Request) (*typesafe.Response, error) {
		return companionResponse(req, nil, "defect_1", 0.9), nil
	}}
	d, err := NewTypeSafeDecider(fc, "jev").Decide(context.Background(), BuildEvidenceWithBudget(in, TypeSafeBudget()))
	require.NoError(t, err)
	require.Equal(t, "reported by <REDACTED_EMAIL>", d.Signals.KnownDefect.Key, "the redacted key that was in the state (R5)")
	b, _ := json.Marshal(fc.calls[0].Questions["known_defect"])
	require.NotContains(t, string(b), "PAY-41", "raw keys are never option text")

	in.LinkedDefects = []LinkedDefect{{Key: "alice@example.com"}, {Key: "bob@example.com"}}
	_, err = NewTypeSafeDecider(fc, "jev").Decide(context.Background(), BuildEvidenceWithBudget(in, TypeSafeBudget()))
	require.NoError(t, err)
	require.NotContains(t, fc.calls[1].Questions, "known_defect", "two defects that render to one key: not asked")

	stray := &fakeClient{fn: func(req typesafe.Request) (*typesafe.Response, error) {
		resp := companionResponse(req, nil, "", 0)
		resp.Answers["known_defect"] = typesafe.Answer{Type: "choice", Choice: "defect_7", Confidence: 0.99}
		return resp, nil
	}}
	in.LinkedDefects = []LinkedDefect{{Key: "PAY-41"}}
	d, err = NewTypeSafeDecider(stray, "jev").Decide(context.Background(), BuildEvidenceWithBudget(in, TypeSafeBudget()))
	require.NoError(t, err)
	require.Nil(t, d.Signals.KnownDefect, "an option the request did not have maps to nothing")
}

func TestDecider_RecordsTheBlocksTheSentStateCarried(t *testing.T) {
	fc := answeringClient()
	dec := NewTypeSafeDecider(fc, "jev")
	in := baseContext()
	in.GroupMembers = []string{"socket hang up"}
	in.LinkedRequirements = []LinkedRequirement{{Key: "REQ-1", Title: "t"}}
	ev := BuildEvidenceWithBudget(in, TypeSafeBudget())
	d, err := dec.Decide(context.Background(), ev)
	require.NoError(t, err)
	require.Equal(t, []string{BlockTest, BlockSteps, BlockFailureError, BlockFailureStack, BlockFailureLog,
		BlockHistory, BlockExamples, BlockDefects, BlockRelatedFailures}, d.Signals.CheckedBlocks, "requirements are never checked")
	require.True(t, d.Signals.MembersChecked)
	require.Len(t, d.Signals.EvidenceHash, 64)
	require.Equal(t, d.Signals, d.Signals.VerifiedAgainst(ev), "hashed over what was sent, which is what was built")

	// The ladder dropped the related failures and the log: neither was checked.
	without := ev
	without.GroupMembers, without.LogText = nil, ""
	b, _ := json.Marshal(stateObject(without))
	ev.StateCap = len(b)
	d, err = dec.Decide(context.Background(), ev)
	require.NoError(t, err)
	require.NotContains(t, d.Signals.CheckedBlocks, BlockRelatedFailures)
	require.NotContains(t, d.Signals.CheckedBlocks, BlockFailureLog)
	require.Contains(t, d.Signals.CheckedBlocks, BlockFailureError)
	require.False(t, d.Signals.MembersChecked)
	require.NotContains(t, fc.calls[1].State.(map[string]any), "group")

	// The hard cap trimmed the error: the narrator's longer error text was not checked.
	in2 := baseContext()
	in2.Result.ErrorMessage = strings.Repeat("E", 5000)
	capped := BuildEvidenceWithBudget(in2, TypeSafeBudget())
	capped.StateCap = 2500
	d, err = dec.Decide(context.Background(), capped)
	require.NoError(t, err)
	require.NotContains(t, d.Signals.CheckedBlocks, BlockFailureError)

	bare := &fakeClient{fn: func(typesafe.Request) (*typesafe.Response, error) {
		return decisionResponse("product_bug", 0.95, 0.9, "product_bug", 0.9, 0.9), nil
	}}
	d, err = NewTypeSafeDecider(bare, "jev").Decide(context.Background(), BuildEvidenceWithBudget(in, TypeSafeBudget()))
	require.NoError(t, err)
	require.False(t, d.Signals.Guarded(), "no injection answer: unguarded")
	require.Nil(t, d.Signals.CheckedBlocks)
	require.False(t, d.Signals.MembersChecked)
}
