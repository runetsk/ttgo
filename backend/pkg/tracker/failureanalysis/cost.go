package failureanalysis

import (
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"
)

// Pricing is what one job's calls cost, captured when its dependencies are resolved, so every
// cost event carries the prices in force when the call was made. The LLM prices are the
// default provider's (nil = unpriced: its events record tokens with no cost). TypeSafePerMTok
// is TypeSafeSettings.PricePerMTok, 0 when TypeSafe is not attached or is free.
type Pricing struct {
	LLMProviderID        string
	LLMPromptPerMTok     *float64
	LLMCompletionPerMTok *float64
	TypeSafePerMTok      float64
}

func (p Pricing) llmCost(prompt, completion int) *float64 {
	return llm.EstimateCostUSD(prompt, completion, p.LLMPromptPerMTok, p.LLMCompletionPerMTok)
}

func (p Pricing) typesafeCost(inputTokens int) *float64 {
	v := float64(inputTokens) * p.TypeSafePerMTok / 1e6
	return &v
}

// CostRefs says what a cost event belongs to: the test run, the job (nil for a single-result
// analysis) and the analysis row (nil when the row could not be stored; the call was still made).
type CostRefs struct {
	RunID      string
	JobID      *string
	AnalysisID *string
}

// RefsFor ties cost events to their run and job and, when it was stored, the analysis row.
func RefsFor(runID string, jobID *string, row *models.RunResultAnalysis) CostRefs {
	refs := CostRefs{RunID: runID, JobID: jobID}
	if row != nil {
		refs.AnalysisID = &row.ID
	}
	return refs
}

func costEvent(kind, engine, model string, refs CostRefs) *models.AIAnalysisCostEvent {
	return &models.AIAnalysisCostEvent{Kind: kind, Engine: engine, Model: model,
		RunID: refs.RunID, JobID: refs.JobID, AnalysisID: refs.AnalysisID}
}

// llmEvent is one LLM ledger row for tokens a call billed, priced at the job's LLM prices.
func llmEvent(kind, model string, prompt, completion int, deps JobDeps, refs CostRefs) *models.AIAnalysisCostEvent {
	ev := costEvent(kind, models.AnalysisCostEngineLLM, model, refs)
	ev.PromptTokens, ev.CompletionTokens = prompt, completion
	ev.EstimatedCost = deps.Pricing.llmCost(prompt, completion)
	if deps.Pricing.LLMProviderID != "" {
		id := deps.Pricing.LLMProviderID
		ev.ProviderID = &id
	}
	return ev
}

// CostEvents turns what one analysis or explanation spent into ledger rows: one TypeSafe
// event when TypeSafe billed input tokens, one LLM event when the LLM billed tokens. Pass
// representatives only; a clone made no call. kind is models.AnalysisCostKind*.
func CostEvents(kind string, res *AnalyzeResult, deps JobDeps, refs CostRefs) []*models.AIAnalysisCostEvent {
	if res == nil {
		return nil
	}
	var out []*models.AIAnalysisCostEvent
	if res.TypeSafeInputTokens > 0 {
		model := deps.DeciderModel
		if res.Engine == models.AnalysisEngineTypeSafe && res.ModelName != "" {
			model = res.ModelName // the versioned id TypeSafe answered with
		}
		ev := costEvent(kind, models.AnalysisCostEngineTypeSafe, model, refs)
		ev.TypeSafeInputTokens = res.TypeSafeInputTokens
		ev.EstimatedCost = deps.Pricing.typesafeCost(res.TypeSafeInputTokens)
		out = append(out, ev)
	}
	if res.TokenUsagePrompt > 0 || res.TokenUsageCompletion > 0 {
		model := deps.NarrativeModel
		if res.Engine == models.AnalysisEngineGenerative && res.ModelName != "" {
			model = res.ModelName // the model the provider says answered
		}
		out = append(out, llmEvent(kind, model, res.TokenUsagePrompt, res.TokenUsageCompletion, deps, refs))
	}
	return out
}

// DecisionCostEvents is what the decision phase billed (spec §A2): the TypeSafe event when
// TypeSafe answered, and one LLM event for LLM tokens spent inside Decide (takeover, fallback,
// generative-only, or what a failed attempt spent). A pending decision has spent no LLM tokens,
// so its explanation is billed once, by NarrationCostEvents. Pass representatives only.
func DecisionCostEvents(res *AnalyzeResult, deps JobDeps, refs CostRefs) []*models.AIAnalysisCostEvent {
	return CostEvents(models.AnalysisCostKindAnalysis, res, deps, refs)
}

// NarrationCostEvents is what one narration billed: a single LLM event from the delta's own
// usage, never a TypeSafe event (the decision phase recorded that). kind is
// models.AnalysisCostKindAnalysis for the worker's narration and AnalysisCostKindExplain for
// Explain. Nothing when the narration made no call (template error, no narrator). It is
// recorded whether or not the narration's apply wins: the call was billed.
func NarrationCostEvents(d NarrationDelta, deps JobDeps, refs CostRefs, kind string) []*models.AIAnalysisCostEvent {
	if d.PromptTokens <= 0 && d.CompletionTokens <= 0 {
		return nil
	}
	return []*models.AIAnalysisCostEvent{llmEvent(kind, deps.NarrativeModel, d.PromptTokens, d.CompletionTokens, deps, refs)}
}

// SemanticCostEvents is the one TypeSafe event for a job's semantic grouping pass, or none
// when it asked nothing. It is billed whether or not its merges were applied.
func SemanticCostEvents(rep SemanticReport, deps JobDeps, refs CostRefs) []*models.AIAnalysisCostEvent {
	if rep.InputTokens <= 0 {
		return nil
	}
	model := rep.Model
	if model == "" && deps.Semantic != nil {
		model = deps.Semantic.Model
	}
	ev := costEvent(models.AnalysisCostKindSemantic, models.AnalysisCostEngineTypeSafe, model, refs)
	ev.TypeSafeInputTokens = rep.InputTokens
	ev.EstimatedCost = deps.Pricing.typesafeCost(rep.InputTokens)
	return []*models.AIAnalysisCostEvent{ev}
}

// TransferCostEvents is the one TypeSafe event for a group's narrative transfer check, or none
// when it billed nothing. It is recorded whether or not the narration's apply wins, and also when
// the check failed after some requests were answered: those were billed. model "" = the
// configured transfer model.
func TransferCostEvents(tokens int, model string, deps JobDeps, refs CostRefs) []*models.AIAnalysisCostEvent {
	if tokens <= 0 {
		return nil
	}
	if model == "" && deps.Transfer != nil {
		model = deps.Transfer.Model
	}
	ev := costEvent(models.AnalysisCostKindTransfer, models.AnalysisCostEngineTypeSafe, model, refs)
	ev.TypeSafeInputTokens = tokens
	ev.EstimatedCost = deps.Pricing.typesafeCost(tokens)
	return []*models.AIAnalysisCostEvent{ev}
}

// TransferStateCharBound bounds one group's transfer-check state for the estimates (R1): every
// checked clone's member line at GroupMemberMsgCap plus about 2,000 characters of explanation.
const TransferStateCharBound = TransferChunk*TransferMaxChunks*GroupMemberMsgCap + 2000

// transferOnPath: the worker narrates this job's TypeSafe decisions, so a check may follow.
func transferOnPath(deps JobDeps) bool {
	return deps.Transfer != nil && deps.Decider != nil && deps.Narrative != nil && !deps.NarrativeSkipped
}

// transferCheckUSD is one priced transfer check at its state bound.
func transferCheckUSD(deps JobDeps) *float64 {
	return deps.Pricing.typesafeCost(TransferStateCharBound / 4)
}

// HedgeCostEvents is one hedge event per fired hedge whose call had a winner, priced at the
// winner's prompt tokens: the hedged request sent the same prompt, and the cancelled request's
// usage is never reported, so the event is an estimate (no completion tokens). A hedge whose
// winner reported no usage has nothing to price and adds no event. winnerPrompts comes from
// callstats (Counter.HedgePromptTokens).
func HedgeCostEvents(winnerPrompts []int, deps JobDeps, refs CostRefs) []*models.AIAnalysisCostEvent {
	var out []*models.AIAnalysisCostEvent
	for _, prompt := range winnerPrompts {
		if prompt <= 0 {
			continue
		}
		ev := costEvent(models.AnalysisCostKindHedge, models.AnalysisCostEngineLLM, deps.NarrativeModel, refs)
		ev.PromptTokens = prompt
		ev.EstimatedCost = deps.Pricing.llmCost(prompt, 0)
		if deps.Pricing.LLMProviderID != "" {
			id := deps.Pricing.LLMProviderID
			ev.ProviderID = &id
		}
		out = append(out, ev)
	}
	return out
}

// addCost sums known costs; nil stays nil only while every part is unknown.
func addCost(total, part *float64) *float64 {
	if part == nil {
		return total
	}
	if total == nil {
		v := *part
		return &v
	}
	v := *total + *part
	return &v
}

// llmStageUSD is the worst case of one LLM stage (spec §B): the first call and the JSON repair,
// each with its transient retry (4 attempts), each attempt the prompt bound in characters / 4
// plus the reply cap; doubled when hedging may send every attempt twice.
func llmStageUSD(deps JobDeps) *float64 {
	attempts := 2 * TransportAttempts
	if deps.HedgingOn {
		attempts *= 2
	}
	return deps.Pricing.llmCost(attempts*PromptCharCap/4, attempts*ReplyTokenCap)
}

// EstimateCallUSD is the worst-case cost of analyzing one group with these dependencies: one
// LLM stage (llmStageUSD) when the LLM is on the normal path (an LLM that is only the fallback
// does not count), plus the TypeSafe state bound / 4 at price_per_mtok when TypeSafe decides, plus
// one transfer check when the decision is narrated and the check is on (R1).
// nil when nothing on the path is priced.
func EstimateCallUSD(deps JobDeps) *float64 {
	var total *float64
	if deps.Decider != nil {
		total = addCost(total, deps.Pricing.typesafeCost(TypeSafeStateCharCap/4))
	}
	llmOnPath := deps.Narrative != nil && (deps.Decider == nil || !deps.NarrativeSkipped || deps.EscalateBelow > 0)
	if llmOnPath {
		total = addCost(total, llmStageUSD(deps))
	}
	if transferOnPath(deps) {
		total = addCost(total, transferCheckUSD(deps))
	}
	return total
}

// EstimateExplainUSD is the worst-case cost of one explanation: one LLM stage, plus one transfer
// check when the check is on (a group's Explain runs it).
func EstimateExplainUSD(deps JobDeps) *float64 {
	if deps.Narrative == nil {
		return nil
	}
	total := llmStageUSD(deps)
	if deps.Transfer != nil {
		total = addCost(total, transferCheckUSD(deps))
	}
	return total
}

// EstimateJobUSD is groups × EstimateCallUSD; nil when unpriced.
func EstimateJobUSD(deps JobDeps, groups int) *float64 {
	per := EstimateCallUSD(deps)
	if per == nil {
		return nil
	}
	v := *per * float64(groups)
	return &v
}

// PlannedGroups is how many analyses a job over these failures would run: signature groups
// when dedup is on (semantic merging can only lower it), capped at the per-run maximum.
func PlannedGroups(failures []*models.RunResult, dedup bool, maxPerRun int) int {
	n := len(failures)
	if dedup {
		n = len(GroupFailures(failures))
	}
	if maxPerRun > 0 && n > maxPerRun {
		n = maxPerRun
	}
	return n
}
