// Package llm provides a thin abstraction over LLM provider HTTP clients.
// All LLM API calls are proxied through the backend; frontend never contacts providers directly.
package llm

import (
	"context"
	"fmt"
	"ttgo/pkg/tracker/models"
)

// ChatMessage is a single message in a chat conversation.
type ChatMessage struct {
	Role    string `json:"role"` // "system" | "user" | "assistant"
	Content string `json:"content"`
}

// ChatRequest is the unified input for a chat completion.
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

// ChatUsage holds token counts returned by the provider.
type ChatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatResponse is the unified output from a chat completion.
type ChatResponse struct {
	Content      string     `json:"content"`
	Model        string     `json:"model,omitempty"`
	FinishReason string     `json:"finish_reason,omitempty"`
	Usage        *ChatUsage `json:"usage,omitempty"`
}

// Provider is the interface that every LLM client must implement.
type Provider interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}

// NewProvider returns the appropriate Provider implementation for the given config.
func NewProvider(cfg *models.LLMProviderConfig) (Provider, error) {
	switch cfg.ProviderType {
	case "openai", "gemini", "local":
		return newOpenAICompatClient(cfg), nil
	case "anthropic":
		return newAnthropicClient(cfg), nil
	default:
		return nil, fmt.Errorf("unknown provider type: %s", cfg.ProviderType)
	}
}
