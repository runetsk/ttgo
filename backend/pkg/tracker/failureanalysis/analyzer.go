package failureanalysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/typesafe"
)

// AnalyzeContext bundles the target RunResult with enrichments and active settings.
type AnalyzeContext struct {
	Result                *models.RunResult
	Steps                 []PromptStep
	SimilarFailures       []SimilarFailure
	SimilarFailuresRollup string
	LinkedDefects         []LinkedDefect
	LinkedRequirements    []LinkedRequirement
	Env                   string
	Browser               string
	OS                    string
	AppVersion            string
	Categories            string
	// RecentOutcomes is this test's last RecentOutcomesLimit results before the analyzed one,
	// oldest first, one letter each (P pass, F fail, E error, S other), from
	// ListRecentOutcomesByTestCase. It goes to the TypeSafe state only
	// (history.recent_outcomes); the LLM prompt never carries it. "" when unknown.
	RecentOutcomes string
	// Examples are past human triage decisions (few-shot), best first; BuildEvidence caps and
	// redacts them. Empty when few-shot examples are off or none qualify.
	Examples []TriageExample
	// GroupMembers are the raw error messages of the other failures analyzed with this one as
	// one group (the worker fills it). BuildEvidence redacts, caps and filters them for the
	// "Related failures in this group" prompt block; the TypeSafe decision never sees them.
	GroupMembers []string

	PromptTemplate   string
	RedactionEnabled bool
	ProviderModel    string
}

// HistoryAvailable reports whether the context carries this test's history: its recent
// failures or the rollup of the labels people gave them.
func (c AnalyzeContext) HistoryAvailable() bool {
	return c.SimilarFailuresRollup != "" || len(c.SimilarFailures) > 0
}

// AnalyzeResult maps 1:1 onto models.RunResultAnalysis (minus IDs/versioning/timestamps).
type AnalyzeResult struct {
	Verdict              string
	Confidence           string
	Summary              string
	NextAction           string
	Rationale            string
	RawResponse          string
	ModelName            string
	TokenUsagePrompt     int
	TokenUsageCompletion int

	Engine                        string
	ConfidenceScore               *float64
	VerdictProbabilities          string // JSON
	SuggestedDefectType           string
	SuggestedDefectTypeConfidence *float64
	SuggestionSource              string
	DefectTypeProbabilities       string // JSON
	NarrativeStatus               string
	PolicyVersion                 string
	TypeSafeInputTokens           int
	// HistoryAvailable: the context carried this test's history (see AnalyzeContext.HistoryAvailable).
	HistoryAvailable bool

	// DecisionStatus is models.DecisionStatusOK for a decision and DecisionStatusFailed for an
	// attempt that produced none; ErrorCategory says why (or why a takeover did not happen).
	DecisionStatus string
	ErrorCategory  string

	// Takeover provenance: TypeSafe's own decision when the LLM decided below the threshold.
	TakeoverFromVerdict    string
	TakeoverFromConfidence *float64
	TakeoverFromDefectType string

	// Stage timing and call accounting.
	DecisionMs   int
	LLMMs        int
	LLMCalls     int
	FinishReason string
}

func jsonOrEmpty(m map[string]float64) string {
	if len(m) == 0 {
		return ""
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func f64ptr(v float64) *float64 { return &v }

// ReplyTokenCap bounds every failure-analysis LLM reply (the generative verdict and the
// TypeSafe narrative). Providers bill generated tokens only, so the cap costs nothing on short
// replies. It was 1,024, which verbose models such as deepseek-v4.1-flash overran twice in a
// row, leaving JSON that could not be parsed. It is not higher than 2,048 because a slow model
// writing 4,096 tokens outlasts the default 90-second provider timeout; a reply cut off at
// 2,048 gets a retry that asks for brevity instead.
const ReplyTokenCap = 2048

// TransportAttempts bounds each LLM call: one retry on a transient provider failure (rate
// limit, 5xx, network), honouring Retry-After. JSON repair is a separate, single follow-up.
const TransportAttempts = 2

// ErrReplyTruncated: the LLM's reply stopped at ReplyTokenCap on the first call and on the
// repair retry, so no complete JSON came back. It is a failed call, not an "unknown" verdict.
var ErrReplyTruncated = errors.New("LLM reply cut off at the length limit twice")

// ErrTypeSafeUnavailable wraps a TypeSafe failure that was not handed to the LLM, because the
// LLM fallback is switched off or no LLM is available. The attempt is recorded as failed.
var ErrTypeSafeUnavailable = errors.New("TypeSafe.ai unavailable and the LLM fallback is off")

// ErrNoNarrator: an explanation was requested but no LLM provider is available.
var ErrNoNarrator = errors.New("no LLM provider is available to write the explanation")

// usageError carries what an LLM exchange spent before it failed — tokens, calls, time and the
// last finish reason — so a failed analysis still records what it cost. errors.Is and
// errors.As see through it; its text is the wrapped error's.
type usageError struct {
	err                error
	prompt, completion int
	calls, ms          int
	finish             string
}

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }

// UsageFromError returns the prompt and completion tokens an analysis consumed before it
// failed with err, or zeros when err carries none.
func UsageFromError(err error) (prompt, completion int) {
	var ue *usageError
	if errors.As(err, &ue) {
		return ue.prompt, ue.completion
	}
	return 0, 0
}

// statsFromError returns the LLM calls, milliseconds and last finish reason an analysis spent
// before it failed with err, or zeros when err carries none.
func statsFromError(err error) (calls, ms int, finish string) {
	var ue *usageError
	if errors.As(err, &ue) {
		return ue.calls, ue.ms, ue.finish
	}
	return 0, 0, ""
}

// callStats accumulates what the LLM calls of one analysis cost in time and attempts.
type callStats struct {
	calls, ms int
	finish    string
}

// chat sends one request with bounded transient retries and records the attempts.
func (s *callStats) chat(ctx context.Context, p llm.Provider, req llm.ChatRequest) (*llm.ChatResponse, error) {
	start := time.Now()
	resp, retries, err := llm.ChatWithRetry(ctx, p, req, llmRetryOptions())
	s.calls += 1 + retries
	s.ms += int(time.Since(start).Milliseconds())
	if resp != nil {
		s.finish = resp.FinishReason
	}
	return resp, err
}

func (s *callStats) apply(out *AnalyzeResult) {
	out.LLMCalls += s.calls
	out.LLMMs += s.ms
	if s.finish != "" {
		out.FinishReason = s.finish
	}
}

// FailedResult is the record of an attempt that produced no decision: the engine and model
// that were tried, why it failed, what it spent and whether the context carried this test's
// history. Decide builds it for its own errors; callers use it only for an error raised before
// Decide returned (a group deadline), passing the context they built, or AnalyzeContext{}.
func FailedResult(err error, deps AnalyzeDeps, in AnalyzeContext) *AnalyzeResult {
	p, c := UsageFromError(err)
	calls, ms, finish := statsFromError(err)
	res := &AnalyzeResult{
		Engine: models.AnalysisEngineGenerative, ModelName: deps.NarrativeModel,
		Verdict: models.VerdictUnknown, Confidence: models.ConfidenceLow,
		Summary:         "analysis failed: " + err.Error(),
		NarrativeStatus: models.NarrativeStatusUnavailable,
		DecisionStatus:  models.DecisionStatusFailed, ErrorCategory: errCategory(err),
		TokenUsagePrompt: p, TokenUsageCompletion: c,
		LLMCalls: calls, LLMMs: ms, FinishReason: finish,
		HistoryAvailable: in.HistoryAvailable(),
	}
	if errors.Is(err, ErrTypeSafeUnavailable) {
		// Nothing was decided; the attempted request carried the context's examples.
		res.Engine, res.ModelName, res.PolicyVersion = models.AnalysisEngineTypeSafe, deps.DeciderModel, PolicyVersionFor(len(in.Examples))
	}
	return res
}

// Analyze runs the cascade (spec §6) in one call: Decide, then Narrate when the decision is
// waiting for its explanation, with the narration applied to the result. Its output is the
// pre-Wave-2 output (testdata/analyze_parity.golden.json). On an error it returns the
// storeable failed result Decide built, or nil when the context was cancelled.
func Analyze(ctx context.Context, deps AnalyzeDeps, in AnalyzeContext) (*AnalyzeResult, error) {
	res, err := Decide(ctx, deps, in)
	if err != nil {
		return res, err
	}
	if d, ok := Narrate(ctx, deps, in, res); ok {
		if d.Reason == narrationAbandoned {
			return nil, ctx.Err()
		}
		res.ApplyNarration(d)
	}
	return res, nil
}

// Decide runs everything up to and including the decision (spec §A1): TypeSafe decides when a
// Decider is present; the LLM decides without one, when TypeSafe fails and the fallback is on,
// or below the takeover threshold. A TypeSafe decision that should be explained comes back
// NarrativeStatusPending, for Narrate; every other route comes back final.
// The settings page draws these branches (frontend/src/utils/analysisFlow.js); change both together.
// On an error it returns the storeable failed attempt with the error (spec §A1), or nil when the context was cancelled.
func Decide(ctx context.Context, deps AnalyzeDeps, in AnalyzeContext) (*AnalyzeResult, error) {
	res, decisionMs, err := decide(ctx, deps, in)
	if err != nil {
		if abandoned(ctx) {
			return nil, err // a cancelled job or a closed request stores nothing
		}
		failed := FailedResult(err, deps, in)
		failed.DecisionMs = decisionMs
		return failed, err
	}
	res.HistoryAvailable = in.HistoryAvailable()
	return res, nil
}

// decide is Decide without the result's context bookkeeping. It also reports how long
// TypeSafe took, so a failed attempt can record it.
func decide(ctx context.Context, deps AnalyzeDeps, in AnalyzeContext) (*AnalyzeResult, int, error) {
	ev := BuildEvidence(in)

	var decision *Decision
	fallbackPrefix := ""
	decisionMs := 0
	if deps.Decider != nil {
		start := time.Now()
		d, err := deps.Decider.Decide(ctx, BuildEvidenceWithBudget(in, TypeSafeBudget()))
		decisionMs = int(time.Since(start).Milliseconds())
		if ctx.Err() != nil {
			return nil, decisionMs, ctx.Err()
		}
		if err != nil {
			if deps.NoLLMFallback || deps.Narrative == nil {
				slog.Warn("failure-analysis: TypeSafe decision failed and the LLM fallback is off", "err", err)
				return nil, decisionMs, fmt.Errorf("%w: %w", ErrTypeSafeUnavailable, err)
			}
			slog.Warn("failure-analysis: TypeSafe decision failed, using generative verdict", "err", err)
			fallbackPrefix = "[verdict engine: TypeSafe unavailable (" + errCategory(err) + "); used generative] "
		} else {
			decision = d
		}
	}
	if decision == nil {
		res, err := analyzeGenerative(ctx, deps, in, ev, fallbackPrefix)
		if res != nil {
			res.DecisionMs = decisionMs
		}
		return res, decisionMs, err
	}

	// Takeover: TypeSafe is not confident enough, so the LLM decides as the generative
	// pipeline would. The row is an LLM analysis (graded on the LLM ladder); it records
	// TypeSafe's own decision and carries TypeSafe's tokens because both ran.
	escalationFailed := ""
	esc := &AnalyzeResult{}
	if deps.EscalateBelow > 0 && deps.Narrative != nil && decision.VerdictConfidence < deps.EscalateBelow {
		prefix := fmt.Sprintf("[verdict engine: TypeSafe unsure (%s at %.2f, below %.2f); the LLM decided] ",
			decision.Verdict, decision.VerdictConfidence, deps.EscalateBelow)
		res, err := analyzeGenerative(ctx, deps, in, ev, prefix)
		if abandoned(ctx) {
			return nil, decisionMs, ctx.Err()
		}
		switch {
		case err == nil && res.DecisionStatus != models.DecisionStatusFailed:
			res.TypeSafeInputTokens = decision.InputTokens
			res.DecisionMs = decisionMs
			res.TakeoverFromVerdict = decision.Verdict
			res.TakeoverFromConfidence = f64ptr(decision.VerdictConfidence)
			res.TakeoverFromDefectType = decision.SuggestedDefectType
			return res, decisionMs, nil
		case err == nil:
			// Two unreadable replies are not an LLM verdict; TypeSafe's decision stands.
			slog.Warn("failure-analysis: LLM takeover reply unparseable, keeping TypeSafe's decision")
			escalationFailed = "unparseable"
			esc = res
		default:
			slog.Warn("failure-analysis: LLM takeover failed, keeping TypeSafe's decision", "err", err)
			escalationFailed = errCategory(err)
			esc.TokenUsagePrompt, esc.TokenUsageCompletion = UsageFromError(err)
			esc.LLMCalls, esc.LLMMs, esc.FinishReason = statsFromError(err)
		}
	}

	out := decisionResult(decision)
	out.DecisionMs = decisionMs
	if escalationFailed != "" {
		// The LLM was just asked and failed; do not call it again to narrate. Its tokens and
		// time were still spent, so they are recorded.
		out.TokenUsagePrompt, out.TokenUsageCompletion = esc.TokenUsagePrompt, esc.TokenUsageCompletion
		out.LLMMs, out.LLMCalls, out.FinishReason = esc.LLMMs, esc.LLMCalls, esc.FinishReason
		out.ErrorCategory = escalationFailed
		out.NarrativeStatus = models.NarrativeStatusUnavailable
		out.Summary = "The LLM could not be asked to decide (" + escalationFailed + "); TypeSafe's low-confidence decision is kept."
		return out, decisionMs, nil
	}
	if deps.NarrativeSkipped {
		out.NarrativeStatus = models.NarrativeStatusSkipped
		out.Summary = "No explanation: explanations are switched off in the TypeSafe.ai settings; the classification above is TypeSafe's decision."
		return out, decisionMs, nil
	}
	if deps.Narrative == nil {
		reason := deps.LLMUnavailableReason
		if reason == "" {
			reason = "no generative provider is configured"
		}
		out.NarrativeStatus = models.NarrativeStatusUnavailable
		out.Summary = "Narrative unavailable: " + reason + "."
		return out, decisionMs, nil
	}
	// Explanations are on and a narrator is attached: the decision is published first and
	// Narrate writes its explanation.
	out.NarrativeStatus = models.NarrativeStatusPending
	return out, decisionMs, nil
}

// decisionResult is a TypeSafe decision as a stored analysis, before any explanation.
func decisionResult(decision *Decision) *AnalyzeResult {
	return &AnalyzeResult{
		Engine:                        models.AnalysisEngineTypeSafe,
		Verdict:                       decision.Verdict,
		Confidence:                    confidenceBucket(decision.VerdictConfidence),
		ConfidenceScore:               f64ptr(decision.VerdictConfidence),
		VerdictProbabilities:          jsonOrEmpty(decision.VerdictProbabilities),
		SuggestedDefectType:           decision.SuggestedDefectType,
		SuggestedDefectTypeConfidence: f64ptr(decision.DefectTypeConfidence),
		SuggestionSource:              decision.SuggestionSource,
		DefectTypeProbabilities:       jsonOrEmpty(decision.DefectTypeProbabilities),
		PolicyVersion:                 decision.PolicyVersion,
		TypeSafeInputTokens:           decision.InputTokens,
		ModelName:                     decision.Model,
		NarrativeStatus:               models.NarrativeStatusOK,
		DecisionStatus:                models.DecisionStatusOK,
	}
}

// replyTruncated reports whether the provider stopped the reply at the token limit
// ("length" for OpenAI-compatible APIs, "max_tokens" for Anthropic).
func replyTruncated(r *llm.ChatResponse) bool {
	if r == nil {
		return false
	}
	return strings.EqualFold(r.FinishReason, "length") || strings.EqualFold(r.FinishReason, "max_tokens")
}

// repairMessage is the follow-up for a reply that could not be parsed: a cut-off reply is
// asked to shorten, anything else to return only the JSON object.
func repairMessage(first *llm.ChatResponse) string {
	if replyTruncated(first) {
		return "Your previous response was cut off at the length limit. Return the complete JSON object again with every field brief."
	}
	return "Your previous response was not valid JSON. Return only the JSON object."
}

// abandoned reports whether ctx was cancelled (the job was cancelled, the server is stopping,
// or the caller went away), as opposed to running out of its deadline. An abandoned analysis
// stores nothing. A deadline hit after TypeSafe decided is not abandonment: the decision is
// kept and the takeover or explanation is recorded as unavailable (category timeout).
func abandoned(ctx context.Context) bool { return errors.Is(ctx.Err(), context.Canceled) }

// errCategory extracts a short category label from provider/typesafe errors for prefixes.
func errCategory(err error) string {
	if errors.Is(err, ErrReplyTruncated) {
		return "truncated"
	}
	var pe *llm.ProviderError
	if errors.As(err, &pe) {
		return string(pe.Category)
	}
	var te *typesafe.Error
	if errors.As(err, &te) {
		return string(te.Category)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	return "error"
}

// analyzeGenerative is the pre-TypeSafe path: the LLM decides and explains in one call.
// It runs on the capped evidence and persists the legacy suggestion mapping.
func analyzeGenerative(ctx context.Context, deps AnalyzeDeps, in AnalyzeContext, ev Evidence, prefix string) (*AnalyzeResult, error) {
	if deps.Narrative == nil {
		return nil, fmt.Errorf("no LLM provider configured")
	}
	prompt, meta, err := BuildPrompt(ev.PromptInput(in.PromptTemplate))
	if err != nil {
		return nil, fmt.Errorf("build prompt: %w", err)
	}
	req := llm.ChatRequest{
		Model:          firstNonEmpty(deps.NarrativeModel, in.ProviderModel),
		Messages:       generativeMessages(prompt),
		Temperature:    0.2,
		MaxTokens:      ReplyTokenCap,
		ResponseFormat: &llm.ResponseFormat{Type: "json_object"},
	}
	stats := &callStats{}
	// spent wraps an error with what this exchange cost so far; the error text is unchanged.
	spent := func(err error, prompt, completion int) error {
		return &usageError{err: err, prompt: prompt, completion: completion,
			calls: stats.calls, ms: stats.ms, finish: stats.finish}
	}
	resp, err := stats.chat(ctx, deps.Narrative, req)
	if err != nil {
		return nil, spent(fmt.Errorf("llm call: %w", err), 0, 0)
	}
	parsed, parseErr := parseVerdict(resp.Content)
	totalPrompt := tokens(resp, true)
	totalCompletion := tokens(resp, false)
	if parseErr != nil {
		retryReq := req
		retryReq.Messages = append(append([]llm.ChatMessage(nil), req.Messages...),
			llm.ChatMessage{Role: "assistant", Content: resp.Content},
			llm.ChatMessage{Role: "user", Content: repairMessage(resp)})
		resp2, err2 := stats.chat(ctx, deps.Narrative, retryReq)
		if err2 != nil {
			return nil, spent(fmt.Errorf("llm retry: %w", err2), totalPrompt, totalCompletion)
		}
		totalPrompt += tokens(resp2, true)
		totalCompletion += tokens(resp2, false)
		parsed, parseErr = parseVerdict(resp2.Content)
		if parseErr != nil && replyTruncated(resp2) {
			// No complete answer came back: a failed call, not the model saying "unknown".
			return nil, fmt.Errorf("llm call: %w", spent(ErrReplyTruncated, totalPrompt, totalCompletion))
		}
		if parseErr != nil {
			// Two complete but unreadable replies: no decision. The row keeps the raw text for
			// inspection and is marked failed, never counted as the model's "unknown".
			raw := headRunes(resp2.Content, 1400)
			out := &AnalyzeResult{
				Engine: models.AnalysisEngineGenerative, NarrativeStatus: models.NarrativeStatusOK,
				Verdict: models.VerdictUnknown, Confidence: models.ConfidenceLow,
				Summary: "AI returned unparseable response — see rationale", NextAction: "Review raw response manually",
				Rationale: prefix + meta.TruncationPrefix + raw, RawResponse: resp2.Content,
				ModelName:            firstNonEmpty(resp2.Model, deps.NarrativeModel, in.ProviderModel),
				TokenUsagePrompt:     totalPrompt,
				TokenUsageCompletion: totalCompletion,
				DecisionStatus:       models.DecisionStatusFailed,
				ErrorCategory:        "unparseable",
			}
			stats.apply(out)
			return out, nil
		}
		resp = resp2
	}
	out := &AnalyzeResult{
		Engine: models.AnalysisEngineGenerative, NarrativeStatus: models.NarrativeStatusOK,
		Verdict: parsed.Verdict, Confidence: parsed.Confidence,
		SuggestedDefectType:  models.SuggestedDefectType(parsed.Verdict),
		Summary:              clamp(parsed.Summary, 400),
		NextAction:           clamp(parsed.NextAction, 200),
		Rationale:            prefix + meta.TruncationPrefix + clamp(parsed.Rationale, 1500),
		RawResponse:          resp.Content,
		ModelName:            firstNonEmpty(resp.Model, deps.NarrativeModel, in.ProviderModel),
		TokenUsagePrompt:     totalPrompt,
		TokenUsageCompletion: totalCompletion,
		DecisionStatus:       models.DecisionStatusOK,
	}
	stats.apply(out)
	return out, nil
}

// generativeMessages sends the template's SYSTEM block as a real system message and the
// rest, which carries the untrusted evidence, as the user message. A template without the
// SYSTEM:/USER: markers is sent as a single user message, as before.
func generativeMessages(prompt string) []llm.ChatMessage {
	system, user := SplitSystemPrompt(prompt)
	if system == "" {
		return []llm.ChatMessage{{Role: "user", Content: user}}
	}
	return []llm.ChatMessage{{Role: "system", Content: system}, {Role: "user", Content: user}}
}

type verdictJSON struct {
	Verdict    string `json:"verdict"`
	Confidence string `json:"confidence"`
	Summary    string `json:"summary"`
	NextAction string `json:"next_action"`
	Rationale  string `json:"rationale"`
}

type rawVerdictJSON struct {
	Verdict    string          `json:"verdict"`
	Confidence json.RawMessage `json:"confidence"`
	Summary    string          `json:"summary"`
	NextAction string          `json:"next_action"`
	Rationale  string          `json:"rationale"`
}

func parseVerdict(raw string) (*verdictJSON, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var rv rawVerdictJSON
	if err := json.Unmarshal([]byte(raw), &rv); err != nil {
		return nil, err
	}
	verdict := normalizeVerdict(rv.Verdict)
	if !models.ValidVerdicts[verdict] {
		return nil, fmt.Errorf("invalid verdict: %q", rv.Verdict)
	}
	confidence, err := normalizeConfidence(rv.Confidence)
	if err != nil {
		return nil, err
	}
	return &verdictJSON{
		Verdict:    verdict,
		Confidence: confidence,
		Summary:    rv.Summary,
		NextAction: rv.NextAction,
		Rationale:  rv.Rationale,
	}, nil
}

func normalizeVerdict(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.Join(strings.Fields(s), "_")
	return s
}

func normalizeConfidence(raw json.RawMessage) (string, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return "", fmt.Errorf("missing confidence")
	}
	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return "", fmt.Errorf("invalid confidence: %w", err)
		}
		s = strings.ToLower(strings.TrimSpace(s))
		if models.ValidConfidences[s] {
			return s, nil
		}
		return "", fmt.Errorf("invalid confidence: %q", s)
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err != nil {
		return "", fmt.Errorf("invalid confidence: %w", err)
	}
	switch {
	case f >= 0.8:
		return models.ConfidenceHigh, nil
	case f >= 0.4:
		return models.ConfidenceMedium, nil
	case f >= 0:
		return models.ConfidenceLow, nil
	default:
		return "", fmt.Errorf("invalid confidence: %v", f)
	}
}

func tokens(r *llm.ChatResponse, prompt bool) int {
	if r == nil || r.Usage == nil {
		return 0
	}
	if prompt {
		return r.Usage.PromptTokens
	}
	return r.Usage.CompletionTokens
}

// clamp caps s at n runes, never cutting inside a multi-byte sequence.
func clamp(s string, n int) string {
	return headRunes(s, n)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
