package failureanalysis

import (
	"context"
	"fmt"
	"log/slog"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/typesafe"
)

// TransferDeps is what the narrative transfer check needs: the TypeSafe client semantic grouping
// uses (same rate limiter) and its model.
type TransferDeps struct {
	Client typesafe.Client
	Model  string
}

// TransferMember is one failure the check shows TypeSafe.
type TransferMember struct {
	ResultID     string
	TestName     string
	ErrorMessage string
}

// TransferInput is a group's explanation and the failures it is checked against: the
// representative it was written from (context only) and the semantic clones to ask about.
type TransferInput struct {
	Summary, NextAction, Rationale string
	Representative                 TransferMember
	Clones                         []TransferMember
	Redact                         bool
}

// TransferResult: Fits maps a checked clone's result id to P(the explanation fits it), nil when
// the check failed. Unchecked counts clones past TransferMaxChunks × TransferChunk. InputTokens is
// what the answered requests billed; Model the id TypeSafe answered with.
type TransferResult struct {
	Fits        map[string]float64
	Unchecked   int
	InputTokens int
	Model       string
}

func transferQuestionID(j int) string { return fmt.Sprintf("fits_%d", j) }

// CheckTransfer asks TypeSafe whether a group's explanation describes each semantic clone's
// failure (spec §2): TransferChunk clones per request, at most TransferMaxChunks requests.
// All-or-nothing like semantic grouping: a failed request or a context that ended before the
// next request returns the error with nil Fits, keeping the tokens the answered requests billed.
// No client or no clones = no call.
func CheckTransfer(ctx context.Context, deps TransferDeps, in TransferInput) (TransferResult, error) {
	var res TransferResult
	if deps.Client == nil || len(in.Clones) == 0 {
		return res, nil
	}
	clones := in.Clones
	if limit := TransferChunk * TransferMaxChunks; len(clones) > limit {
		res.Unchecked = len(clones) - limit
		clones = clones[:limit]
	}
	fits := make(map[string]float64, len(clones))
	for start := 0; start < len(clones); start += TransferChunk {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		chunk := clones[start:min(start+TransferChunk, len(clones))]
		resp, err := deps.Client.Evaluate(ctx, transferRequest(deps.Model, in, chunk))
		if err != nil {
			return res, err
		}
		res.InputTokens += resp.Usage.InputTokens
		if resp.Model != "" {
			res.Model = resp.Model
		}
		for j, c := range chunk {
			if a, ok := resp.Answers[transferQuestionID(j+1)]; ok {
				fits[c.ResultID] = a.Noul
			}
		}
	}
	res.Fits = fits
	return res, nil
}

// transferRequest builds one chunk's request: the explanation, the representative at index 0 and
// the chunk's clones after it, each error one line capped at GroupMemberMsgCap and redacted when
// redaction is on (as the narrator's member lines are).
func transferRequest(model string, in TransferInput, chunk []TransferMember) typesafe.Request {
	red := func(s string) string {
		if in.Redact {
			return Redact(s)
		}
		return s
	}
	member := func(i int, m TransferMember) map[string]any {
		return map[string]any{
			"index":     i,
			"test_name": headRunes(red(m.TestName), TestNameCap),
			"error":     oneline(headRunes(red(m.ErrorMessage), GroupMemberMsgCap)),
		}
	}
	failures := []map[string]any{member(0, in.Representative)}
	qs := make(map[string]typesafe.Question, len(chunk))
	for j, c := range chunk {
		failures = append(failures, member(j+1, c))
		qs[transferQuestionID(j+1)] = transferQuestion(j + 1)
	}
	state := map[string]any{
		"explanation": map[string]any{"summary": red(in.Summary), "next_action": red(in.NextAction), "rationale": red(in.Rationale)},
		"failures":    failures,
	}
	return typesafe.Request{State: state, Model: model, Questions: qs}
}

// TransferMemberFrom is a result as the check shows it.
func TransferMemberFrom(r *models.RunResult) TransferMember {
	return TransferMember{ResultID: r.ID, TestName: r.TestNameSnapshot, ErrorMessage: r.ErrorMessage}
}

// SemanticClones are the group's members merged in by semantic grouping, in member order.
func SemanticClones(g *FailureGroup) []*models.RunResult {
	var out []*models.RunResult
	for _, m := range g.Members {
		if _, ok := g.SemanticMembers[m.ID]; ok && m.ID != g.Representative.ID {
			out = append(out, m)
		}
	}
	return out
}

// TransferInputFor pairs a narration with the failures it is checked against.
func TransferInputFor(d NarrationDelta, rep *models.RunResult, clones []*models.RunResult, redact bool) TransferInput {
	in := TransferInput{Summary: d.Summary, NextAction: d.NextAction, Rationale: d.Rationale,
		Representative: TransferMemberFrom(rep), Redact: redact}
	for _, c := range clones {
		in.Clones = append(in.Clones, TransferMemberFrom(c))
	}
	return in
}

// RunTransferCheck runs CheckTransfer for an ok narration and records the outcome on the delta:
// Fits on success; TransferFailed with nil Fits on any error, including a context that ended
// during the check. Tokens, model and the unchecked count are kept either way. A delta that is
// not ok, or an input without clones, comes back unchanged and costs nothing.
func RunTransferCheck(ctx context.Context, deps TransferDeps, in TransferInput, d NarrationDelta) NarrationDelta {
	if d.NarrativeStatus != models.NarrativeStatusOK || len(in.Clones) == 0 {
		return d
	}
	res, err := CheckTransfer(ctx, deps, in)
	d.TransferTokens, d.TransferModel, d.TransferUnchecked = res.InputTokens, res.Model, res.Unchecked
	if err != nil {
		slog.Warn("failure-analysis: narrative transfer check failed; fits left unset",
			"representative_result_id", in.Representative.ResultID, "clones", len(in.Clones), "err", err)
		d.TransferFailed, d.Fits = true, nil
		return d
	}
	d.Fits = res.Fits
	return d
}
