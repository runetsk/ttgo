package typesafe

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func choiceQ() map[string]Question {
	return map[string]Question{
		"verdict": {Type: "choice", Instructions: "which?", Criteria: map[string]any{"a": "A", "b": "B"}},
		"same":    {Type: "noul", Instructions: "same?"},
	}
}

func okBody() string {
	return `{"model":"jev-1.13.0","answers":{
		"verdict":{"type":"choice","choice":"a","probabilities":{"a":0.8,"b":0.2},"confidence":0.7},
		"same":{"type":"noul","noul":0.9}},
		"usage":{"input_tokens":321,"output_tokens":12}}`
}

func newTestClient(t *testing.T, h http.HandlerFunc) (*HTTPClient, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := NewHTTPClient("key-123", Options{
		BaseURL: srv.URL, Timeout: 2 * time.Second, HTTP: srv.Client(),
		sleep: func(ctx context.Context, d time.Duration) error { return nil }, // no real backoff in tests
	})
	return c, srv
}

func TestEvaluate_SendsBearerModelAndQuestions(t *testing.T) {
	var got map[string]any
	var auth, path string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		path = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(okBody()))
	})
	resp, err := c.Evaluate(context.Background(), Request{State: map[string]any{"x": 1}, Model: "jev-1.13.0", Questions: choiceQ()})
	require.NoError(t, err)
	require.Equal(t, "Bearer key-123", auth)
	require.Equal(t, "/v1/systemone", path)
	require.Equal(t, "jev-1.13.0", got["model"])
	require.Contains(t, got["questions"], "verdict")
	require.Equal(t, "jev-1.13.0", resp.Model)
	require.Equal(t, "a", resp.Answers["verdict"].Choice)
	require.InDelta(t, 0.7, resp.Answers["verdict"].Confidence, 1e-9)
	require.InDelta(t, 0.9, resp.Answers["same"].Noul, 1e-9)
	require.Equal(t, 321, resp.Usage.InputTokens)
}

func TestEvaluate_RetriesRateLimitHonoringRetryAfter(t *testing.T) {
	var calls int32
	var slept time.Duration
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set("Retry-After", "3")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(okBody()))
	}))
	defer srv.Close()
	c := NewHTTPClient("k", Options{BaseURL: srv.URL, HTTP: srv.Client(),
		sleep: func(ctx context.Context, d time.Duration) error { slept = d; return nil }})
	_, err := c.Evaluate(context.Background(), Request{State: "s", Model: "m", Questions: choiceQ()})
	require.NoError(t, err)
	require.Equal(t, int32(2), atomic.LoadInt32(&calls))
	require.Equal(t, 3*time.Second, slept)
}

func TestEvaluate_RetriesOverloadedAnd5xx_NotAuthOrValidation(t *testing.T) {
	for _, tc := range []struct {
		status    int
		wantCalls int32
		wantCat   Category
	}{
		{529, 3, CategoryOverloaded}, {503, 3, CategoryInternal},
		{401, 1, CategoryAuth}, {422, 1, CategoryValidation},
	} {
		var calls int32
		c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&calls, 1)
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(`{"error":"nope"}`))
		})
		_, err := c.Evaluate(context.Background(), Request{State: "s", Model: "m", Questions: choiceQ()})
		require.Error(t, err, tc.status)
		var te *Error
		require.ErrorAs(t, err, &te)
		require.Equal(t, tc.wantCat, te.Category, tc.status)
		require.Equal(t, tc.wantCalls, atomic.LoadInt32(&calls), tc.status)
	}
}

func TestEvaluate_OversizedValidationFlagged(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		_, _ = w.Write([]byte(`{"detail":"state exceeds the 32k token context limit"}`))
	})
	_, err := c.Evaluate(context.Background(), Request{State: "s", Model: "m", Questions: choiceQ()})
	var te *Error
	require.ErrorAs(t, err, &te)
	require.Equal(t, CategoryValidation, te.Category)
	require.True(t, te.Oversized)
}

func TestEvaluate_CancelDuringBackoffReturnsPromptly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	c := NewHTTPClient("k", Options{BaseURL: srv.URL, HTTP: srv.Client()}) // real sleep
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(50 * time.Millisecond); cancel() }()
	start := time.Now()
	_, err := c.Evaluate(ctx, Request{State: "s", Model: "m", Questions: choiceQ()})
	require.Error(t, err)
	require.Less(t, time.Since(start), 2*time.Second)
}

func TestEvaluate_RejectsInvalidAnswers(t *testing.T) {
	cases := map[string]string{
		"missing id":      `{"model":"m","answers":{"verdict":{"type":"choice","choice":"a","probabilities":{"a":0.8,"b":0.2},"confidence":0.7}},"usage":{"input_tokens":1,"output_tokens":1}}`,
		"wrong type":      `{"model":"m","answers":{"verdict":{"type":"noul","noul":0.5},"same":{"type":"noul","noul":0.9}},"usage":{"input_tokens":1,"output_tokens":1}}`,
		"foreign choice":  `{"model":"m","answers":{"verdict":{"type":"choice","choice":"zzz","probabilities":{"a":0.8,"b":0.2},"confidence":0.7},"same":{"type":"noul","noul":0.9}},"usage":{"input_tokens":1,"output_tokens":1}}`,
		"missing option":  `{"model":"m","answers":{"verdict":{"type":"choice","choice":"a","probabilities":{"a":1.0},"confidence":0.7},"same":{"type":"noul","noul":0.9}},"usage":{"input_tokens":1,"output_tokens":1}}`,
		"sum 0.9":         `{"model":"m","answers":{"verdict":{"type":"choice","choice":"a","probabilities":{"a":0.7,"b":0.2},"confidence":0.7},"same":{"type":"noul","noul":0.9}},"usage":{"input_tokens":1,"output_tokens":1}}`,
		"null confidence": `{"model":"m","answers":{"verdict":{"type":"choice","choice":"a","probabilities":{"a":0.8,"b":0.2},"confidence":null},"same":{"type":"noul","noul":0.9}},"usage":{"input_tokens":1,"output_tokens":1}}`,
		"null noul":       `{"model":"m","answers":{"verdict":{"type":"choice","choice":"a","probabilities":{"a":0.8,"b":0.2},"confidence":0.7},"same":{"type":"noul","noul":null}},"usage":{"input_tokens":1,"output_tokens":1}}`,
		"missing model":   `{"answers":{"verdict":{"type":"choice","choice":"a","probabilities":{"a":0.8,"b":0.2},"confidence":0.7},"same":{"type":"noul","noul":0.9}},"usage":{"input_tokens":1,"output_tokens":1}}`,
		"missing usage":   `{"model":"m","answers":{"verdict":{"type":"choice","choice":"a","probabilities":{"a":0.8,"b":0.2},"confidence":0.7},"same":{"type":"noul","noul":0.9}},"usage":{"output_tokens":1}}`,
		"not json":        `<html>`,
	}
	for name, body := range cases {
		c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) })
		_, err := c.Evaluate(context.Background(), Request{State: "s", Model: "m", Questions: choiceQ()})
		var te *Error
		require.ErrorAs(t, err, &te, name)
		require.Equal(t, CategoryParse, te.Category, name)
	}
}

func TestEvaluate_OversizedBodyIsParseError(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", MaxResponseBytes+10)))
	})
	_, err := c.Evaluate(context.Background(), Request{State: "s", Model: "m", Questions: choiceQ()})
	var te *Error
	require.ErrorAs(t, err, &te)
	require.Equal(t, CategoryParse, te.Category)
}

func TestListModels(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/models", r.URL.Path)
		_, _ = w.Write([]byte(`{"models":[{"name":"jev-latest","description":"d","release_date":"2026-09-01"}]}`))
	})
	ms, err := c.ListModels(context.Background())
	require.NoError(t, err)
	require.Len(t, ms, 1)
	require.Equal(t, "2026-09-01", ms[0].ReleaseDate)
}
