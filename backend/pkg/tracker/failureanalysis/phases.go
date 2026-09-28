package failureanalysis

import (
	"context"
	"encoding/json"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"
)

// NarrationDelta is what one narration produced (spec §A1): the explanation fields and what
// its own LLM calls cost. It carries no decision field, so applying it can never change a
// decision. NarrativeStatus is ok, unavailable or unparseable; Summary is the text to store,
// including "AI narrative unavailable: <reason>" when the narration failed.
type NarrationDelta struct {
	Summary         string
	NextAction      string
	Rationale       string
	RawResponse     string
	FinishReason    string
	NarrativeStatus string
	// Reason is the short cause of an unavailable or unparseable narration ("timeout",
	// "truncated", "template error", "unparseable", narrationAbandoned, ...); "" when ok.
	Reason string

	PromptTokens     int
	CompletionTokens int
	LLMMs            int
	LLMCalls         int
}

// narrationAbandoned is the Reason of a narration whose context was cancelled mid-call. Such
// a delta is never stored: Analyze and Explain return the context error instead.
const narrationAbandoned = "cancelled"

// ApplyNarration writes a narration into a decided result: the explanation fields replace
// the pending ones and the narration's usage and timing are added to the decision's.
func (r *AnalyzeResult) ApplyNarration(d NarrationDelta) {
	r.NarrativeStatus = d.NarrativeStatus
	r.Summary, r.NextAction, r.Rationale = d.Summary, d.NextAction, d.Rationale
	r.RawResponse = d.RawResponse
	r.TokenUsagePrompt += d.PromptTokens
	r.TokenUsageCompletion += d.CompletionTokens
	r.LLMMs += d.LLMMs
	r.LLMCalls += d.LLMCalls
	if d.FinishReason != "" {
		r.FinishReason = d.FinishReason
	}
}

// Narrate asks the LLM to explain a decision Decide left pending. It runs only for
// NarrativeStatusPending (false otherwise, with no call) and never fails: a provider error, a
// cut-off or unreadable reply or a template error is a delta marked unavailable or
// unparseable, and the decision stands.
func Narrate(ctx context.Context, deps AnalyzeDeps, in AnalyzeContext, decided *AnalyzeResult) (NarrationDelta, bool) {
	if decided == nil || decided.NarrativeStatus != models.NarrativeStatusPending {
		return NarrationDelta{}, false
	}
	if deps.Narrative == nil {
		reason := firstNonEmpty(deps.LLMUnavailableReason, "no generative provider is configured")
		return unavailableDelta(reason, PromptMeta{}), true
	}
	decision := decisionOf(decided)
	pin := BuildEvidence(in).PromptInput(in.PromptTemplate)
	pin.DecidedVerdict, pin.DecidedConfidence, pin.DecidedDefectType = decision.Verdict, decided.Confidence, decision.SuggestedDefectType
	prompt, meta, err := BuildPrompt(pin)
	if err != nil {
		return unavailableDelta("template error", meta), true
	}
	_, user := SplitSystemPrompt(prompt)
	req := llm.ChatRequest{
		Model: firstNonEmpty(deps.NarrativeModel, in.ProviderModel),
		Messages: []llm.ChatMessage{
			{Role: "system", Content: narrativeSystemMessage(decision)},
			{Role: "user", Content: user},
		},
		Temperature: 0.2, MaxTokens: ReplyTokenCap, ResponseFormat: &llm.ResponseFormat{Type: "json_object"},
	}
	stats := &callStats{}
	prompts, completions := 0, 0
	done := func(d NarrationDelta) (NarrationDelta, bool) {
		d.PromptTokens, d.CompletionTokens = prompts, completions
		d.LLMCalls, d.LLMMs, d.FinishReason = stats.calls, stats.ms, stats.finish
		return d, true
	}
	resp, err := stats.chat(ctx, deps.Narrative, req)
	if abandoned(ctx) {
		return done(unavailableDelta(narrationAbandoned, meta))
	}
	if err != nil {
		return done(unavailableDelta(errCategory(err), meta))
	}
	prompts, completions = tokens(resp, true), tokens(resp, false)
	parsed, perr := parseNarrative(resp.Content)
	if perr != nil {
		retry := req
		retry.Messages = append(retry.Messages,
			llm.ChatMessage{Role: "assistant", Content: resp.Content},
			llm.ChatMessage{Role: "user", Content: repairMessage(resp)})
		resp2, err2 := stats.chat(ctx, deps.Narrative, retry)
		if abandoned(ctx) {
			return done(unavailableDelta(narrationAbandoned, meta))
		}
		if err2 != nil {
			return done(unavailableDelta(errCategory(err2), meta))
		}
		prompts += tokens(resp2, true)
		completions += tokens(resp2, false)
		parsed, perr = parseNarrative(resp2.Content)
		if perr != nil && replyTruncated(resp2) {
			return done(unavailableDelta("truncated", meta))
		}
		if perr != nil {
			return done(NarrationDelta{
				NarrativeStatus: models.NarrativeStatusUnparseable, Reason: "unparseable",
				Summary:     "AI narrative unavailable: unparseable response",
				Rationale:   meta.TruncationPrefix + headRunes(resp2.Content, 1400),
				RawResponse: resp2.Content,
			})
		}
		resp = resp2
	}
	return done(NarrationDelta{
		NarrativeStatus: models.NarrativeStatusOK,
		Summary:         clamp(parsed.Summary, 400),
		NextAction:      clamp(parsed.NextAction, 200),
		Rationale:       meta.TruncationPrefix + clamp(parsed.Rationale, 1500),
		RawResponse:     resp.Content,
	})
}

// unavailableDelta is a narration that produced no explanation; the rationale keeps only the
// note of what was trimmed from the prompt.
func unavailableDelta(reason string, meta PromptMeta) NarrationDelta {
	return NarrationDelta{NarrativeStatus: models.NarrativeStatusUnavailable, Reason: reason,
		Summary: "AI narrative unavailable: " + reason, Rationale: meta.TruncationPrefix}
}

// decisionOf rebuilds the TypeSafe decision a narration explains from a decided result (or a
// stored row mapped onto one). Probabilities survive the JSON round trip exactly, so the
// system message is the one the original decision would produce.
func decisionOf(r *AnalyzeResult) *Decision {
	d := &Decision{Verdict: r.Verdict, SuggestedDefectType: r.SuggestedDefectType}
	if r.ConfidenceScore != nil {
		d.VerdictConfidence = *r.ConfidenceScore
	}
	if r.VerdictProbabilities != "" {
		_ = json.Unmarshal([]byte(r.VerdictProbabilities), &d.VerdictProbabilities)
	}
	return d
}
