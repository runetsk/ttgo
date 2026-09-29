package failureanalysis

import (
	"context"
	"errors"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/typesafe"
)

// ErrOwnCheckUnavailable: there is no TypeSafe client to check a result's own evidence with.
var ErrOwnCheckUnavailable = errors.New("TypeSafe.ai is not available to check this result's evidence")

// OwnEvidenceCheck is the injection-only TypeSafe check of one result's own evidence before it is
// explained on its own (spec R9): the injection answer plus the blocks (and their hashes) the sent
// state carried, what the request billed and the model that answered.
type OwnEvidenceCheck struct {
	Signals     Signals
	InputTokens int
	Model       string
}

// CheckOwnEvidence sends one result's own evidence — the state a decision would send for it, with
// no group members — to TypeSafe with the injection question only. Narrate then sends the LLM only
// the blocks in Signals.CheckedBlocks. An error means nothing was checked; InputTokens may still
// be set when TypeSafe answered without an injection answer (it billed).
func CheckOwnEvidence(ctx context.Context, deps TransferDeps, in AnalyzeContext) (OwnEvidenceCheck, error) {
	var out OwnEvidenceCheck
	if deps.Client == nil {
		return out, ErrOwnCheckUnavailable
	}
	in.GroupMembers = nil
	built := BuildEvidenceWithBudget(in, TypeSafeBudget())
	state, _, sent := renderState(built)
	resp, err := deps.Client.Evaluate(ctx, typesafe.Request{State: state, Model: deps.Model,
		Questions: map[string]typesafe.Question{questionInjection: injectionQuestion()}})
	if err != nil {
		return out, err
	}
	out.InputTokens, out.Model = resp.Usage.InputTokens, resp.Model
	a, ok := resp.Answers[questionInjection]
	if !ok {
		return out, errors.New("typesafe: the injection question was not answered")
	}
	p := a.Noul
	out.Signals.Injection = &p
	out.Signals.RecordChecked(built, sent)
	return out, nil
}

// OwnCheckCostEvents is the TypeSafe event of an own-evidence check, billed as part of the
// explanation it guards (kind explain), or none when it billed nothing.
func OwnCheckCostEvents(check OwnEvidenceCheck, deps JobDeps, refs CostRefs) []*models.AIAnalysisCostEvent {
	if check.InputTokens <= 0 {
		return nil
	}
	model := check.Model
	if model == "" && deps.Transfer != nil {
		model = deps.Transfer.Model
	}
	ev := costEvent(models.AnalysisCostKindExplain, models.AnalysisCostEngineTypeSafe, model, refs)
	ev.TypeSafeInputTokens = check.InputTokens
	ev.EstimatedCost = deps.Pricing.typesafeCost(check.InputTokens)
	return []*models.AIAnalysisCostEvent{ev}
}

// EstimateOwnExplainUSD is the worst case of Explain ?scope=result: one LLM stage, plus the
// injection-only TypeSafe request at the state bound when TypeSafe is attached.
func EstimateOwnExplainUSD(deps JobDeps) *float64 {
	if deps.Narrative == nil {
		return nil
	}
	total := llmStageUSD(deps)
	if deps.Transfer != nil {
		total = addCost(total, deps.Pricing.typesafeCost(TypeSafeStateCharCap/4))
	}
	return total
}
