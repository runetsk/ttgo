package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Ported from the legacy tree. Current routes openai/anthropic providers through
// safehttp.GuardedClient, which refuses loopback (127.0.0.1) — exactly where
// httptest.NewServer binds. Each server-backed test therefore swaps in
// srv.Client() after construction so the loopback guard doesn't reject the test
// server; the request-building and response-parsing logic stays under test.

func TestNewProvider(t *testing.T) {
	cases := []struct {
		name       string
		provider   string
		wantErr    bool
		wantClient string
	}{
		{"openai", "openai", false, "*llm.openAICompatClient"},
		{"gemini", "gemini", false, "*llm.openAICompatClient"},
		{"local", "local", false, "*llm.openAICompatClient"},
		{"anthropic", "anthropic", false, "*llm.anthropicClient"},
		{"unknown", "foobar", true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := NewProvider(&models.LLMProviderConfig{ProviderType: tc.provider})
			if tc.wantErr {
				require.Error(t, err)
				require.Nil(t, p)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, p)
		})
	}
}

func TestOpenAICompat_Chat_Success(t *testing.T) {
	var capturedPath string
	var capturedBody openAIRequest
	var capturedAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &capturedBody)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"model": "gpt-test",
			"choices": [{"finish_reason":"stop","message":{"content":"hello"}}],
			"usage": {"prompt_tokens": 10, "completion_tokens": 3, "total_tokens": 13}
		}`))
	}))
	defer srv.Close()

	cfg := &models.LLMProviderConfig{
		ProviderType: "openai",
		EndpointURL:  srv.URL,
		APIKey:       "sk-test-key",
	}
	c := newOpenAICompatClient(cfg)
	c.httpClient = srv.Client()

	resp, err := c.Chat(context.Background(), ChatRequest{
		Model:       "gpt-test",
		Messages:    []ChatMessage{{Role: "user", Content: "hi"}},
		Temperature: 0.5,
		MaxTokens:   100,
	})

	require.NoError(t, err)
	assert.Equal(t, "hello", resp.Content)
	assert.Equal(t, "gpt-test", resp.Model)
	assert.Equal(t, "stop", resp.FinishReason)
	require.NotNil(t, resp.Usage)
	assert.Equal(t, 13, resp.Usage.TotalTokens)
	assert.Equal(t, "/v1/chat/completions", capturedPath)
	assert.Equal(t, "Bearer sk-test-key", capturedAuth)
	require.NotNil(t, capturedBody.ResponseFormat)
	assert.Equal(t, "json_object", capturedBody.ResponseFormat.Type)
}

func TestOpenAICompat_Chat_LocalOmitsResponseFormat(t *testing.T) {
	var capturedBody openAIRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &capturedBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	c := newOpenAICompatClient(&models.LLMProviderConfig{ProviderType: "local", EndpointURL: srv.URL})
	c.httpClient = srv.Client()
	_, err := c.Chat(context.Background(), ChatRequest{Model: "m", Messages: []ChatMessage{{Role: "user", Content: "hi"}}})
	require.NoError(t, err)
	assert.Nil(t, capturedBody.ResponseFormat)
}

func TestOpenAICompat_Chat_DefaultEndpointUsed(t *testing.T) {
	// Empty endpoint, unknown provider type → URL built against "" (will fail to connect).
	c := newOpenAICompatClient(&models.LLMProviderConfig{ProviderType: "mystery", TimeoutSeconds: 1})
	_, err := c.Chat(context.Background(), ChatRequest{Model: "m", Messages: []ChatMessage{{Role: "user", Content: "hi"}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LLM request failed")
}

func TestOpenAICompat_Chat_HTTPErrorWithJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad model"}}`))
	}))
	defer srv.Close()

	c := newOpenAICompatClient(&models.LLMProviderConfig{ProviderType: "openai", EndpointURL: srv.URL})
	c.httpClient = srv.Client()
	_, err := c.Chat(context.Background(), ChatRequest{Model: "m", Messages: []ChatMessage{{Role: "user", Content: "hi"}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad model")
	assert.Contains(t, err.Error(), "400")
}

func TestOpenAICompat_Chat_HTTPErrorNonJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(strings.Repeat("x", 1000)))
	}))
	defer srv.Close()

	c := newOpenAICompatClient(&models.LLMProviderConfig{ProviderType: "openai", EndpointURL: srv.URL})
	c.httpClient = srv.Client()
	_, err := c.Chat(context.Background(), ChatRequest{Model: "m", Messages: []ChatMessage{{Role: "user", Content: "hi"}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
	assert.Contains(t, err.Error(), "truncated")
}

func TestOpenAICompat_Chat_EmptyChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer srv.Close()

	c := newOpenAICompatClient(&models.LLMProviderConfig{ProviderType: "openai", EndpointURL: srv.URL})
	c.httpClient = srv.Client()
	_, err := c.Chat(context.Background(), ChatRequest{Model: "m", Messages: []ChatMessage{{Role: "user", Content: "hi"}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty choices")
}

func TestOpenAICompat_Chat_InvalidJSONResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	c := newOpenAICompatClient(&models.LLMProviderConfig{ProviderType: "openai", EndpointURL: srv.URL})
	c.httpClient = srv.Client()
	_, err := c.Chat(context.Background(), ChatRequest{Model: "m", Messages: []ChatMessage{{Role: "user", Content: "hi"}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse response")
}

func TestOpenAICompat_TimeoutDefaults(t *testing.T) {
	c := newOpenAICompatClient(&models.LLMProviderConfig{ProviderType: "openai"})
	assert.Equal(t, 120*time.Second, c.httpClient.Timeout)

	c2 := newOpenAICompatClient(&models.LLMProviderConfig{ProviderType: "openai", TimeoutSeconds: 5})
	assert.Equal(t, 5*time.Second, c2.httpClient.Timeout)
}

func TestAnthropic_Chat_Success(t *testing.T) {
	var capturedPath, capturedKey, capturedVersion string
	var capturedBody anthropicRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedKey = r.Header.Get("x-api-key")
		capturedVersion = r.Header.Get("anthropic-version")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &capturedBody)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"model": "claude-test",
			"stop_reason": "end_turn",
			"content": [{"type":"text","text":"hi there"}],
			"usage": {"input_tokens": 5, "output_tokens": 2}
		}`))
	}))
	defer srv.Close()

	c := newAnthropicClient(&models.LLMProviderConfig{
		ProviderType: "anthropic",
		EndpointURL:  srv.URL,
		APIKey:       "k-test",
	})
	c.httpClient = srv.Client()

	resp, err := c.Chat(context.Background(), ChatRequest{
		Model: "claude-test",
		Messages: []ChatMessage{
			{Role: "system", Content: "be concise"},
			{Role: "user", Content: "hi"},
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "hi there", resp.Content)
	assert.Equal(t, "claude-test", resp.Model)
	assert.Equal(t, "end_turn", resp.FinishReason)
	require.NotNil(t, resp.Usage)
	assert.Equal(t, 7, resp.Usage.TotalTokens)

	assert.Equal(t, "/v1/messages", capturedPath)
	assert.Equal(t, "k-test", capturedKey)
	assert.Equal(t, "2023-06-01", capturedVersion)
	assert.Equal(t, "be concise", capturedBody.System)
	assert.Len(t, capturedBody.Messages, 1)
	assert.Equal(t, "user", capturedBody.Messages[0].Role)
	assert.Equal(t, 8192, capturedBody.MaxTokens) // default
}

func TestAnthropic_Chat_HTTPErrorWithJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid key"}}`))
	}))
	defer srv.Close()

	c := newAnthropicClient(&models.LLMProviderConfig{ProviderType: "anthropic", EndpointURL: srv.URL})
	c.httpClient = srv.Client()
	_, err := c.Chat(context.Background(), ChatRequest{Model: "m", Messages: []ChatMessage{{Role: "user", Content: "hi"}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid key")
	assert.Contains(t, err.Error(), "401")
}

func TestAnthropic_Chat_EmptyContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"content":[]}`))
	}))
	defer srv.Close()

	c := newAnthropicClient(&models.LLMProviderConfig{ProviderType: "anthropic", EndpointURL: srv.URL})
	c.httpClient = srv.Client()
	_, err := c.Chat(context.Background(), ChatRequest{Model: "m", Messages: []ChatMessage{{Role: "user", Content: "hi"}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty content")
}

func TestAnthropic_Chat_NoTextBlock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"content":[{"type":"image","text":""}]}`))
	}))
	defer srv.Close()

	c := newAnthropicClient(&models.LLMProviderConfig{ProviderType: "anthropic", EndpointURL: srv.URL})
	c.httpClient = srv.Client()
	_, err := c.Chat(context.Background(), ChatRequest{Model: "m", Messages: []ChatMessage{{Role: "user", Content: "hi"}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no text block")
}

func TestAnthropic_Chat_InvalidJSONResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	c := newAnthropicClient(&models.LLMProviderConfig{ProviderType: "anthropic", EndpointURL: srv.URL})
	c.httpClient = srv.Client()
	_, err := c.Chat(context.Background(), ChatRequest{Model: "m", Messages: []ChatMessage{{Role: "user", Content: "hi"}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse response")
}

func TestAnthropic_Chat_RespectsCustomMaxTokens(t *testing.T) {
	var capturedBody anthropicRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &capturedBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"ok"}]}`))
	}))
	defer srv.Close()

	c := newAnthropicClient(&models.LLMProviderConfig{ProviderType: "anthropic", EndpointURL: srv.URL})
	c.httpClient = srv.Client()
	_, err := c.Chat(context.Background(), ChatRequest{
		Model:     "m",
		Messages:  []ChatMessage{{Role: "user", Content: "hi"}},
		MaxTokens: 42,
	})
	require.NoError(t, err)
	assert.Equal(t, 42, capturedBody.MaxTokens)
}

func TestAnthropic_TimeoutDefaults(t *testing.T) {
	c := newAnthropicClient(&models.LLMProviderConfig{ProviderType: "anthropic"})
	assert.Equal(t, 120*time.Second, c.httpClient.Timeout)

	c2 := newAnthropicClient(&models.LLMProviderConfig{ProviderType: "anthropic", TimeoutSeconds: 7})
	assert.Equal(t, 7*time.Second, c2.httpClient.Timeout)
}
