package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	"ttgo/internal/safehttp"
	"ttgo/pkg/tracker/models"
)

const anthropicDefaultEndpoint = "https://api.anthropic.com"
const anthropicAPIVersion = "2023-06-01"
const maxLLMResponseSize = 10 << 20 // 10 MB

// anthropicClient implements the Provider interface for the Anthropic Claude Messages API.
type anthropicClient struct {
	cfg        *models.LLMProviderConfig
	httpClient *http.Client
}

func newAnthropicClient(cfg *models.LLMProviderConfig) *anthropicClient {
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 120 * time.Second
	}
	return &anthropicClient{
		cfg:        cfg,
		httpClient: safehttp.GuardedClient(timeout), // SSRF guard: refuses internal/metadata hosts (F-002)
	}
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicResponse struct {
	Model      string `json:"model"`
	StopReason string `json:"stop_reason"`
	Content    []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage *anthropicUsage `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *anthropicClient) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	endpointBase := c.cfg.EndpointURL
	if endpointBase == "" {
		endpointBase = anthropicDefaultEndpoint
	}
	url := endpointBase + "/v1/messages"

	// Separate system prompt from user/assistant messages.
	var systemPrompt string
	var msgs []anthropicMessage
	for _, m := range req.Messages {
		if m.Role == "system" {
			systemPrompt = m.Content
		} else {
			msgs = append(msgs, anthropicMessage{Role: m.Role, Content: m.Content})
		}
	}

	maxTok := req.MaxTokens
	if maxTok == 0 {
		maxTok = 8192
	}

	body := anthropicRequest{
		Model:     req.Model,
		MaxTokens: maxTok,
		System:    systemPrompt,
		Messages:  msgs,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.cfg.APIKey)
	httpReq.Header.Set("anthropic-version", anthropicAPIVersion)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxLLMResponseSize))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp anthropicResponse
		if jsonErr := json.Unmarshal(respBody, &errResp); jsonErr == nil && errResp.Error != nil {
			return nil, fmt.Errorf("Anthropic API error (HTTP %d): %s", resp.StatusCode, errResp.Error.Message)
		}
		bodyStr := string(respBody)
		if len(bodyStr) > 500 {
			bodyStr = bodyStr[:500] + "...(truncated)"
		}
		return nil, fmt.Errorf("Anthropic API error (HTTP %d): %s", resp.StatusCode, bodyStr)
	}

	var parsed anthropicResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	if len(parsed.Content) == 0 {
		return nil, fmt.Errorf("Anthropic returned empty content")
	}

	// Find first text block.
	for _, block := range parsed.Content {
		if block.Type == "text" {
			cr := &ChatResponse{
				Content:      block.Text,
				Model:        parsed.Model,
				FinishReason: parsed.StopReason,
			}
			if parsed.Usage != nil {
				cr.Usage = &ChatUsage{
					PromptTokens:     parsed.Usage.InputTokens,
					CompletionTokens: parsed.Usage.OutputTokens,
					TotalTokens:      parsed.Usage.InputTokens + parsed.Usage.OutputTokens,
				}
			}
			return cr, nil
		}
	}

	return nil, fmt.Errorf("Anthropic response contained no text block")
}
