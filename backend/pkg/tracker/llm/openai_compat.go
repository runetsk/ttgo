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

// defaultEndpoints maps provider types to their default base URLs.
var defaultEndpoints = map[string]string{
	"openai": "https://api.openai.com",
	"gemini": "https://generativelanguage.googleapis.com/v1beta/openai",
	"local":  "http://localhost:11434",
}

// openAICompatClient implements the Provider interface for OpenAI-compatible APIs.
// Covers OpenAI GPT, Google Gemini (OpenAI-compat endpoint), and Local/Ollama.
type openAICompatClient struct {
	cfg        *models.LLMProviderConfig
	httpClient *http.Client
}

func newOpenAICompatClient(cfg *models.LLMProviderConfig) *openAICompatClient {
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 120 * time.Second
	}
	// "local" providers (Ollama/LAN) legitimately target private hosts, so they
	// use the integration-mode guard (allows loopback/private but STILL blocks
	// cloud-metadata/link-local); cloud providers get the strict guard that refuses
	// any internal host even if endpoint_url is hostile (F-002).
	var client *http.Client
	if cfg.ProviderType == "local" {
		client = safehttp.IntegrationClient(timeout)
	} else {
		client = safehttp.GuardedClient(timeout)
	}
	return &openAICompatClient{
		cfg:        cfg,
		httpClient: client,
	}
}

// openAIRequest is the request body for OpenAI-compatible chat completions.
type openAIRequest struct {
	Model          string          `json:"model"`
	Messages       []openAIMessage `json:"messages"`
	Temperature    float64         `json:"temperature,omitempty"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type openAIResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage *openAIUsage `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *openAICompatClient) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	endpointBase := c.cfg.EndpointURL
	if endpointBase == "" {
		if def, ok := defaultEndpoints[c.cfg.ProviderType]; ok {
			endpointBase = def
		}
	}
	url := endpointBase + "/v1/chat/completions"

	msgs := make([]openAIMessage, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = openAIMessage{Role: m.Role, Content: m.Content}
	}

	body := openAIRequest{
		Model:          req.Model,
		Messages:       msgs,
		Temperature:    req.Temperature,
		MaxTokens:      req.MaxTokens,
		ResponseFormat: &responseFormat{Type: "json_object"},
	}

	// Local/Ollama doesn't always support json_object response_format — only set for cloud providers.
	if c.cfg.ProviderType == "local" {
		body.ResponseFormat = nil
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
	if c.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}

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
		var errResp openAIResponse
		if jsonErr := json.Unmarshal(respBody, &errResp); jsonErr == nil && errResp.Error != nil {
			return nil, fmt.Errorf("LLM API error (HTTP %d): %s", resp.StatusCode, errResp.Error.Message)
		}
		bodyStr := string(respBody)
		if len(bodyStr) > 500 {
			bodyStr = bodyStr[:500] + "...(truncated)"
		}
		return nil, fmt.Errorf("LLM API error (HTTP %d): %s", resp.StatusCode, bodyStr)
	}

	var parsed openAIResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("LLM returned empty choices")
	}

	cr := &ChatResponse{
		Content:      parsed.Choices[0].Message.Content,
		Model:        parsed.Model,
		FinishReason: parsed.Choices[0].FinishReason,
	}
	if parsed.Usage != nil {
		cr.Usage = &ChatUsage{
			PromptTokens:     parsed.Usage.PromptTokens,
			CompletionTokens: parsed.Usage.CompletionTokens,
			TotalTokens:      parsed.Usage.TotalTokens,
		}
	}
	return cr, nil
}
