package failureanalysis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"
)

// AnalyzeContext bundles the target RunResult with enrichments and active settings.
type AnalyzeContext struct {
	Result             *models.RunResult
	Steps              []PromptStep
	SimilarFailures    []SimilarFailure
	LinkedDefects      []LinkedDefect
	LinkedRequirements []LinkedRequirement
	Env                string
	Browser            string
	OS                 string
	AppVersion         string
	Categories         string

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
}

// Analyze builds the prompt, calls the provider (with one retry on invalid
// JSON), and returns a parsed verdict. If parsing fails twice, returns an
// "unknown" verdict with the raw response preserved in Rationale.
func Analyze(ctx context.Context, provider llm.Provider, in AnalyzeContext) (*AnalyzeResult, error) {
	errMsg := in.Result.ErrorMessage
	stack := in.Result.StackTrace
	logs := in.Result.LogText
	if in.RedactionEnabled {
		errMsg = Redact(errMsg)
		stack = Redact(stack)
		logs = Redact(logs)
	}

	prompt, meta, err := BuildPrompt(PromptInput{
		Template:           in.PromptTemplate,
		TestName:           in.Result.TestNameSnapshot,
		Categories:         in.Categories,
		Env:                in.Env,
		Browser:            in.Browser,
		OS:                 in.OS,
		AppVersion:         in.AppVersion,
		Steps:              in.Steps,
		FailureType:        in.Result.FailureType,
		ErrorMessage:       errMsg,
		StackTrace:         stack,
		LogText:            logs,
		SimilarFailures:    in.SimilarFailures,
		LinkedDefects:      in.LinkedDefects,
		LinkedRequirements: in.LinkedRequirements,
	})
	if err != nil {
		return nil, fmt.Errorf("build prompt: %w", err)
	}

	req := llm.ChatRequest{
		Model:       in.ProviderModel,
		Messages:    []llm.ChatMessage{{Role: "user", Content: prompt}},
		Temperature: 0.2,
		MaxTokens:   1024,
	}

	resp, err := provider.Chat(ctx, req)
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
		resp2, err2 := provider.Chat(ctx, retryReq)
		if err2 != nil {
			return nil, fmt.Errorf("llm retry: %w", err2)
		}
		totalPrompt += tokens(resp2, true)
		totalCompletion += tokens(resp2, false)
		parsed, parseErr = parseVerdict(resp2.Content)
		if parseErr != nil {
			raw := resp2.Content
			if len(raw) > 1400 {
				raw = raw[:1400]
			}
			return &AnalyzeResult{
				Verdict:              models.VerdictUnknown,
				Confidence:           models.ConfidenceLow,
				Summary:              "AI returned unparseable response — see rationale",
				NextAction:           "Review raw response manually",
				Rationale:            meta.TruncationPrefix + raw,
				RawResponse:          resp2.Content,
				ModelName:            resp2.Model,
				TokenUsagePrompt:     totalPrompt,
				TokenUsageCompletion: totalCompletion,
			}, nil
		}
		resp = resp2
	}

	return &AnalyzeResult{
		Verdict:              parsed.Verdict,
		Confidence:           parsed.Confidence,
		Summary:              clamp(parsed.Summary, 400),
		NextAction:           clamp(parsed.NextAction, 200),
		Rationale:            meta.TruncationPrefix + clamp(parsed.Rationale, 1500),
		RawResponse:          resp.Content,
		ModelName:            firstNonEmpty(resp.Model, in.ProviderModel),
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

func clamp(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
