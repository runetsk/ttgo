package failureanalysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
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

	PromptTemplate   string
	RedactionEnabled bool
	ProviderModel    string
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
	DefectTypeProbabilities       string // JSON
	NarrativeStatus               string
	PolicyVersion                 string
	TypeSafeInputTokens           int
}

func jsonOrEmpty(m map[string]float64) string {
	if len(m) == 0 {
		return ""
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func f64ptr(v float64) *float64 { return &v }

// Analyze runs the cascade (spec §6): TypeSafe decides when a Decider is present, the generative
// provider narrates; without a decider (or when it fails) the generative provider decides as today.
func Analyze(ctx context.Context, deps AnalyzeDeps, in AnalyzeContext) (*AnalyzeResult, error) {
	ev := BuildEvidence(in)

	var decision *Decision
	fallbackPrefix := ""
	if deps.Decider != nil {
		d, err := deps.Decider.Decide(ctx, ev)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if err != nil {
			slog.Warn("failure-analysis: TypeSafe decision failed, using generative verdict", "err", err)
			fallbackPrefix = "[verdict engine: TypeSafe unavailable (" + errCategory(err) + "); used generative] "
		} else {
			decision = d
		}
	}
	if decision == nil {
		return analyzeGenerative(ctx, deps, in, ev, fallbackPrefix)
	}

	out := &AnalyzeResult{
		Engine:                        models.AnalysisEngineTypeSafe,
		Verdict:                       decision.Verdict,
		Confidence:                    confidenceBucket(decision.VerdictConfidence),
		ConfidenceScore:               f64ptr(decision.VerdictConfidence),
		VerdictProbabilities:          jsonOrEmpty(decision.VerdictProbabilities),
		SuggestedDefectType:           decision.SuggestedDefectType,
		SuggestedDefectTypeConfidence: f64ptr(decision.DefectTypeConfidence),
		DefectTypeProbabilities:       jsonOrEmpty(decision.DefectTypeProbabilities),
		PolicyVersion:                 decision.PolicyVersion,
		TypeSafeInputTokens:           decision.InputTokens,
		ModelName:                     decision.Model,
		NarrativeStatus:               models.NarrativeStatusOK,
	}
	if deps.Narrative == nil {
		out.NarrativeStatus = models.NarrativeStatusUnavailable
		out.Summary = "Narrative unavailable: no generative provider is configured."
		return out, nil
	}

	pin := ev.PromptInput(in.PromptTemplate)
	pin.DecidedVerdict, pin.DecidedConfidence, pin.DecidedDefectType = decision.Verdict, out.Confidence, decision.SuggestedDefectType
	prompt, meta, err := BuildPrompt(pin)
	if err != nil {
		return narrativeUnavailable(out, "template error", meta), nil
	}
	req := llm.ChatRequest{
		Model: firstNonEmpty(deps.NarrativeModel, in.ProviderModel),
		Messages: []llm.ChatMessage{
			{Role: "system", Content: narrativeSystemMessage(decision)},
			{Role: "user", Content: prompt},
		},
		Temperature: 0.2, MaxTokens: 1024, ResponseFormat: &llm.ResponseFormat{Type: "json_object"},
	}
	resp, err := deps.Narrative.Chat(ctx, req)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return narrativeUnavailable(out, errCategory(err), meta), nil
	}
	out.TokenUsagePrompt, out.TokenUsageCompletion = tokens(resp, true), tokens(resp, false)
	parsed, perr := parseNarrative(resp.Content)
	if perr != nil {
		retry := req
		retry.Messages = append(retry.Messages,
			llm.ChatMessage{Role: "assistant", Content: resp.Content},
			llm.ChatMessage{Role: "user", Content: "Your previous response was not valid JSON. Return only the JSON object."})
		resp2, err2 := deps.Narrative.Chat(ctx, retry)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if err2 != nil {
			return narrativeUnavailable(out, errCategory(err2), meta), nil
		}
		out.TokenUsagePrompt += tokens(resp2, true)
		out.TokenUsageCompletion += tokens(resp2, false)
		parsed, perr = parseNarrative(resp2.Content)
		if perr != nil {
			raw := headRunes(resp2.Content, 1400)
			out.NarrativeStatus = models.NarrativeStatusUnparseable
			out.Summary = "AI narrative unavailable: unparseable response"
			out.Rationale = meta.TruncationPrefix + raw
			out.RawResponse = resp2.Content
			return out, nil
		}
		resp = resp2
	}
	out.Summary = clamp(parsed.Summary, 400)
	out.NextAction = clamp(parsed.NextAction, 200)
	out.Rationale = meta.TruncationPrefix + clamp(parsed.Rationale, 1500)
	out.RawResponse = resp.Content
	return out, nil
}

func narrativeUnavailable(out *AnalyzeResult, reason string, meta PromptMeta) *AnalyzeResult {
	out.NarrativeStatus = models.NarrativeStatusUnavailable
	out.Summary = "AI narrative unavailable: " + reason
	out.NextAction = ""
	out.Rationale = meta.TruncationPrefix
	return out
}

// errCategory extracts a short category label from provider/typesafe errors for prefixes.
func errCategory(err error) string {
	var pe *llm.ProviderError
	if errors.As(err, &pe) {
		return string(pe.Category)
	}
	var te *typesafe.Error
	if errors.As(err, &te) {
		return string(te.Category)
	}
	return "error"
}

// analyzeGenerative is the pre-TypeSafe path, unchanged except that it runs on the capped
// evidence and persists the legacy suggestion mapping.
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
		Messages:       []llm.ChatMessage{{Role: "user", Content: prompt}},
		Temperature:    0.2,
		MaxTokens:      1024,
		ResponseFormat: &llm.ResponseFormat{Type: "json_object"},
	}
	resp, err := deps.Narrative.Chat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("llm call: %w", err)
	}
	parsed, parseErr := parseVerdict(resp.Content)
	totalPrompt := tokens(resp, true)
	totalCompletion := tokens(resp, false)
	if parseErr != nil {
		retryReq := req
		retryReq.Messages = append(retryReq.Messages,
			llm.ChatMessage{Role: "assistant", Content: resp.Content},
			llm.ChatMessage{Role: "user", Content: "Your previous response was not valid JSON. Return only the JSON object."})
		resp2, err2 := deps.Narrative.Chat(ctx, retryReq)
		if err2 != nil {
			return nil, fmt.Errorf("llm retry: %w", err2)
		}
		totalPrompt += tokens(resp2, true)
		totalCompletion += tokens(resp2, false)
		parsed, parseErr = parseVerdict(resp2.Content)
		if parseErr != nil {
			raw := headRunes(resp2.Content, 1400)
			return &AnalyzeResult{
				Engine: models.AnalysisEngineGenerative, NarrativeStatus: models.NarrativeStatusOK,
				Verdict: models.VerdictUnknown, Confidence: models.ConfidenceLow,
				Summary: "AI returned unparseable response — see rationale", NextAction: "Review raw response manually",
				Rationale: prefix + meta.TruncationPrefix + raw, RawResponse: resp2.Content,
				ModelName:            firstNonEmpty(resp2.Model, deps.NarrativeModel, in.ProviderModel),
				TokenUsagePrompt:     totalPrompt,
				TokenUsageCompletion: totalCompletion,
			}, nil
		}
		resp = resp2
	}
	return &AnalyzeResult{
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
	}, nil
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
