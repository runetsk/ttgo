package failureanalysis

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/typesafe"

	"github.com/stretchr/testify/require"
)

func transferClones(n int) []TransferMember {
	out := make([]TransferMember, n)
	for i := range out {
		out[i] = TransferMember{ResultID: fmt.Sprintf("c%d", i), TestName: "t", ErrorMessage: fmt.Sprintf("err %d", i)}
	}
	return out
}

func transferIn(clones []TransferMember) TransferInput {
	return TransferInput{Summary: "Checkout button timed out", NextAction: "Wait for #checkout", Rationale: "R",
		Representative: TransferMember{ResultID: "rep", TestName: "checkout", ErrorMessage: "Timeout waiting for #checkout after 5000ms"},
		Clones:         clones}
}

// allFit answers every fits_ question with p and bills tokens per request.
func allFit(p float64, tokens int) func(req typesafe.Request) (*typesafe.Response, error) {
	return func(req typesafe.Request) (*typesafe.Response, error) {
		resp := &typesafe.Response{Model: "jev-1.13.0", Answers: map[string]typesafe.Answer{}, Usage: typesafe.Usage{InputTokens: tokens}}
		for id := range req.Questions {
			resp.Answers[id] = typesafe.Answer{Type: "noul", Noul: p}
		}
		return resp, nil
	}
}

func stateFailures(req typesafe.Request) []map[string]any {
	return req.State.(map[string]any)["failures"].([]map[string]any)
}

func TestTransferConstants(t *testing.T) {
	require.Equal(t, 0.50, TransferFitMin)
	require.Equal(t, 40, TransferChunk)
	require.Equal(t, 1, TransferMaxChunks, "R10: one request of 40 clones per group")
}

func TestTransferQuestion_IsANoulAboutTheNamedFailure(t *testing.T) {
	q := transferQuestion(3)
	require.Equal(t, "noul", q.Type)
	require.Equal(t, []string{"false", "true"}, criteriaKeys(t, q.Criteria))
	ins := q.Instructions.(map[string]any)
	require.Contains(t, ins["question"], "`failures[3]`", "question ids are never shown to the model, so the index is in the text")
	require.Equal(t, 3, ins["failure"])
	crit := q.Criteria.(map[string]any)
	require.Contains(t, crit["false"], "redaction placeholder carries no information")
}

func TestCheckTransfer_NoClonesOrNoClientMakesNoCall(t *testing.T) {
	fc := &fakeClient{fn: allFit(0.9, 10)}
	res, err := CheckTransfer(context.Background(), TransferDeps{Client: fc, Model: "jev"}, transferIn(nil))
	require.NoError(t, err)
	require.Nil(t, res.Fits)
	require.Empty(t, fc.calls)

	res, err = CheckTransfer(context.Background(), TransferDeps{}, transferIn(transferClones(2)))
	require.NoError(t, err)
	require.Nil(t, res.Fits)
}

func TestCheckTransfer_AsksEachCloneAgainstTheRepresentativesExplanation(t *testing.T) {
	fc := &fakeClient{fn: func(req typesafe.Request) (*typesafe.Response, error) {
		failures := stateFailures(req)
		resp := &typesafe.Response{Model: "jev-1.13.0", Answers: map[string]typesafe.Answer{}, Usage: typesafe.Usage{InputTokens: 120}}
		for id := range req.Questions {
			var j int
			_, _ = fmt.Sscanf(id, "fits_%d", &j)
			p := 0.9
			if strings.Contains(failures[j]["error"].(string), "503") {
				p = 0.1
			}
			resp.Answers[id] = typesafe.Answer{Type: "noul", Noul: p}
		}
		return resp, nil
	}}
	clones := []TransferMember{
		{ResultID: "a", TestName: "checkout A", ErrorMessage: "Timeout waiting for #checkout after 7000ms"},
		{ResultID: "b", TestName: "checkout B", ErrorMessage: "POST /api/cart returned 503"},
	}
	res, err := CheckTransfer(context.Background(), TransferDeps{Client: fc, Model: "jev-1.13.0"}, transferIn(clones))
	require.NoError(t, err)
	require.Equal(t, map[string]float64{"a": 0.9, "b": 0.1}, res.Fits)
	require.Equal(t, 120, res.InputTokens)
	require.Equal(t, "jev-1.13.0", res.Model)
	require.Zero(t, res.Unchecked)

	require.Len(t, fc.calls, 1)
	req := fc.calls[0]
	require.Equal(t, "jev-1.13.0", req.Model)
	state := req.State.(map[string]any)
	require.Equal(t, map[string]any{"summary": "Checkout button timed out", "next_action": "Wait for #checkout", "rationale": "R"}, state["explanation"])
	failures := stateFailures(req)
	require.Len(t, failures, 3)
	require.Equal(t, 0, failures[0]["index"])
	require.Equal(t, "checkout", failures[0]["test_name"], "the representative the explanation was written from comes first")
	require.Equal(t, 2, failures[2]["index"])
	require.Equal(t, "POST /api/cart returned 503", failures[2]["error"])
	require.Len(t, req.Questions, 2)
	require.Contains(t, req.Questions, "fits_1")
	require.Contains(t, req.Questions, "fits_2")
	require.NotContains(t, req.Questions, "fits_0", "the representative is context, not a question")
}

func TestCheckTransfer_OneRequestOf40PerGroup(t *testing.T) {
	fc := &fakeClient{fn: allFit(0.8, 100)}
	res, err := CheckTransfer(context.Background(), TransferDeps{Client: fc, Model: "jev"}, transferIn(transferClones(85)))
	require.NoError(t, err)
	require.Len(t, fc.calls, 1, "TransferMaxChunks = 1")
	require.Len(t, fc.calls[0].Questions, TransferChunk)
	require.Len(t, stateFailures(fc.calls[0]), TransferChunk+1)
	require.Equal(t, "err 39", stateFailures(fc.calls[0])[40]["error"])
	require.Len(t, res.Fits, 40)
	require.Contains(t, res.Fits, "c39")
	require.NotContains(t, res.Fits, "c40")
	require.Equal(t, 45, res.Unchecked, "clones past TransferMaxChunks × TransferChunk stay unchecked")
	require.Equal(t, 100, res.InputTokens)
}

func TestCheckTransfer_RedactsAndCapsWhatItSends(t *testing.T) {
	fc := &fakeClient{fn: allFit(0.9, 10)}
	in := transferIn([]TransferMember{{ResultID: "a", TestName: "login as qa@example.com",
		ErrorMessage: "login failed password=hunter22\n" + strings.Repeat("x", 500)}})
	in.Summary = "Login rejected for qa@example.com"
	in.Redact = true
	_, err := CheckTransfer(context.Background(), TransferDeps{Client: fc}, in)
	require.NoError(t, err)
	body := fmt.Sprintf("%v", fc.calls[0].State)
	require.NotContains(t, body, "hunter22")
	require.NotContains(t, body, "qa@example.com")
	require.Contains(t, body, "<REDACTED_EMAIL>")
	errLine := stateFailures(fc.calls[0])[1]["error"].(string)
	require.Len(t, []rune(errLine), GroupMemberMsgCap)
	require.NotContains(t, errLine, "\n", "one line per failure")

	in.Redact = false
	fc.calls = nil
	_, err = CheckTransfer(context.Background(), TransferDeps{Client: fc}, in)
	require.NoError(t, err)
	require.Contains(t, fmt.Sprintf("%v", fc.calls[0].State), "hunter22", "redaction off sends the text as stored")
}

func TestCheckTransfer_AFailedRequestReturnsNoFits(t *testing.T) {
	fc := &fakeClient{fn: func(typesafe.Request) (*typesafe.Response, error) {
		return nil, &typesafe.Error{Category: typesafe.CategoryInternal, Status: 500, Message: "boom"}
	}}
	res, err := CheckTransfer(context.Background(), TransferDeps{Client: fc}, transferIn(transferClones(45)))
	require.Error(t, err)
	require.Nil(t, res.Fits, "no fits from a failed check")
	require.Zero(t, res.InputTokens, "a failed request reports no usage")
	require.Equal(t, 5, res.Unchecked, "the cap is reported either way")
}

func TestCheckTransfer_AnEndedContextSendsNothing(t *testing.T) {
	fc := &fakeClient{fn: allFit(0.9, 10)}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res, err := CheckTransfer(ctx, TransferDeps{Client: fc}, transferIn(transferClones(3)))
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, res.Fits)
	require.Empty(t, fc.calls)
}

func TestRunTransferCheck_RecordsTheOutcomeOnTheDelta(t *testing.T) {
	ok := NarrationDelta{Summary: "S", NarrativeStatus: models.NarrativeStatusOK}

	fc := &fakeClient{fn: allFit(0.3, 90)}
	d := RunTransferCheck(context.Background(), TransferDeps{Client: fc, Model: "jev"}, transferIn(transferClones(1)), ok)
	require.Equal(t, map[string]float64{"c0": 0.3}, d.Fits)
	require.False(t, d.TransferFailed)
	require.Equal(t, 90, d.TransferTokens)
	require.Equal(t, "jev-1.13.0", d.TransferModel)
	require.Equal(t, "S", d.Summary, "the explanation is untouched")

	failing := &fakeClient{fn: func(typesafe.Request) (*typesafe.Response, error) { return nil, errors.New("down") }}
	d = RunTransferCheck(context.Background(), TransferDeps{Client: failing}, transferIn(transferClones(1)), ok)
	require.True(t, d.TransferFailed)
	require.Nil(t, d.Fits)
	require.Equal(t, models.NarrativeStatusOK, d.NarrativeStatus, "a failed check never withholds the explanation")

	unavailable := NarrationDelta{NarrativeStatus: models.NarrativeStatusUnavailable, Reason: "timeout"}
	fc.calls = nil
	d = RunTransferCheck(context.Background(), TransferDeps{Client: fc}, transferIn(transferClones(1)), unavailable)
	require.Equal(t, unavailable, d)
	require.Empty(t, fc.calls, "only a written explanation is checked")
}

func TestSemanticClonesAndTransferInputFor(t *testing.T) {
	rep := &models.RunResult{ID: "r0", TestNameSnapshot: "rep", ErrorMessage: "e0"}
	sig := &models.RunResult{ID: "r1", ErrorMessage: "e0"}
	sem := &models.RunResult{ID: "r2", TestNameSnapshot: "sem", ErrorMessage: "e2"}
	g := &FailureGroup{Key: "k", Representative: rep, Members: []*models.RunResult{rep, sig, sem},
		SemanticMembers: map[string]float64{"r2": 0.9}}
	clones := SemanticClones(g)
	require.Equal(t, []*models.RunResult{sem}, clones)
	require.Empty(t, SemanticClones(&FailureGroup{Representative: rep, Members: []*models.RunResult{rep, sig}}))

	in := TransferInputFor(NarrationDelta{Summary: "S", NextAction: "N", Rationale: "R"}, rep, clones, true)
	require.Equal(t, TransferInput{Summary: "S", NextAction: "N", Rationale: "R",
		Representative: TransferMember{ResultID: "r0", TestName: "rep", ErrorMessage: "e0"},
		Clones:         []TransferMember{{ResultID: "r2", TestName: "sem", ErrorMessage: "e2"}}, Redact: true}, in)
}

// Backlog #33: execution values read as placeholders on both sides of the transfer check.
func TestTransferRequest_NormalizesExecutionValues(t *testing.T) {
	in := TransferInput{Summary: "The gateway circuit is open (incident PAY-7200).",
		Representative: TransferMember{ErrorMessage: "POST /api/pay returned 503 (incident PAY-7200)"}}
	req := transferRequest("m", in, []TransferMember{{ErrorMessage: "POST /api/pay returned 503 (incident PAY-3549)"}})
	st := req.State.(map[string]any)
	require.Equal(t, "The gateway circuit is open (incident PAY-<N>).", st["explanation"].(map[string]any)["summary"])
	fs := st["failures"].([]map[string]any)
	require.Equal(t, fs[0]["error"], fs[1]["error"], "the two errors read the same")
	require.Equal(t, "POST /api/pay returned 503 (incident PAY-<N>)", fs[1]["error"])
}
