package typesafe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"ttgo/internal/safehttp"
)

const (
	DefaultBaseURL = "https://api.typesafe.ai"
	// EnvBaseURL names the operator-set environment variable that replaces
	// DefaultBaseURL when Options.BaseURL is empty — for gateways that serve the
	// same System One endpoint under another host (OpenRouter: https://openrouter.ai/api).
	EnvBaseURL              = "TYPESAFE_BASE_URL"
	MaxResponseBytes        = 10 << 20
	ProbabilitySumTolerance = 0.02
	defaultTimeout          = 30 * time.Second
	defaultMaxAttempts      = 3
	baseBackoff             = 500 * time.Millisecond
	maxBackoff              = 8 * time.Second
	maxRetryAfter           = 30 * time.Second
)

// Question is one typed question. Instructions may be a string, object or array; Criteria is
// a map (choice/noul) or ordered slice (score). Question ids are NOT sent to the model.
type Question struct {
	Type         string `json:"type"`
	Instructions any    `json:"instructions"`
	Criteria     any    `json:"criteria,omitempty"`
}

type Request struct {
	State     any                 `json:"state"`
	Model     string              `json:"model"`
	Questions map[string]Question `json:"questions"`
}

type Answer struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice,omitempty"`
	Score         float64            `json:"score,omitempty"`
	Noul          float64            `json:"noul,omitempty"`
	Confidence    float64            `json:"confidence,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Legend        map[string]string  `json:"legend,omitempty"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type Response struct {
	Model   string            `json:"model"`
	Answers map[string]Answer `json:"answers"`
	Usage   Usage             `json:"usage"`
}

type Model struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ReleaseDate string `json:"release_date"`
}

// Client is the seam callers depend on; tests substitute a fake.
type Client interface {
	Evaluate(ctx context.Context, req Request) (*Response, error)
	ListModels(ctx context.Context) ([]Model, error)
}

// Options configure NewHTTPClient. Zero values take the defaults; sleep is a test seam.
type Options struct {
	BaseURL     string
	Timeout     time.Duration
	MaxAttempts int
	HTTP        *http.Client
	sleep       func(ctx context.Context, d time.Duration) error
}

type HTTPClient struct {
	apiKey      string
	baseURL     string
	maxAttempts int
	http        *http.Client
	sleep       func(ctx context.Context, d time.Duration) error
	jitter      func() float64
}

func NewHTTPClient(apiKey string, opts Options) *HTTPClient {
	c := &HTTPClient{apiKey: apiKey, baseURL: strings.TrimRight(opts.BaseURL, "/"),
		maxAttempts: opts.MaxAttempts, http: opts.HTTP, sleep: opts.sleep, jitter: rand.Float64}
	if c.baseURL == "" {
		c.baseURL = strings.TrimRight(os.Getenv(EnvBaseURL), "/")
	}
	if c.baseURL == "" {
		c.baseURL = DefaultBaseURL
	}
	if c.maxAttempts <= 0 {
		c.maxAttempts = defaultMaxAttempts
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	if c.http == nil {
		c.http = safehttp.GuardedClient(timeout) // SSRF guard, fixed public host
	}
	if c.sleep == nil {
		c.sleep = func(ctx context.Context, d time.Duration) error {
			t := time.NewTimer(d)
			defer t.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-t.C:
				return nil
			}
		}
	}
	return c
}

// wire structs use pointers so "missing or null" is distinguishable from zero.
type wireAnswer struct {
	Type          *string             `json:"type"`
	Choice        *string             `json:"choice"`
	Score         *float64            `json:"score"`
	Noul          *float64            `json:"noul"`
	Confidence    *float64            `json:"confidence"`
	Probabilities map[string]*float64 `json:"probabilities"`
	Legend        map[string]string   `json:"legend"`
}
type wireUsage struct {
	InputTokens  *int `json:"input_tokens"`
	OutputTokens *int `json:"output_tokens"`
}
type wireResponse struct {
	Model   *string               `json:"model"`
	Answers map[string]wireAnswer `json:"answers"`
	Usage   *wireUsage            `json:"usage"`
}

func (c *HTTPClient) Evaluate(ctx context.Context, req Request) (*Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, &Error{Category: CategoryInternal, Message: "encode request: " + err.Error()}
	}
	var lastErr error
	for attempt := 0; attempt < c.maxAttempts; attempt++ {
		raw, herr := c.do(ctx, http.MethodPost, "/v1/systemone", body)
		if herr == nil {
			return validate(raw, req)
		}
		lastErr = herr
		if ctx.Err() != nil {
			return nil, herr
		}
		var te *Error
		if !errors.As(herr, &te) || !te.Retryable() || attempt == c.maxAttempts-1 {
			return nil, herr
		}
		delay := baseBackoff << attempt
		if delay > maxBackoff {
			delay = maxBackoff
		}
		delay = time.Duration(float64(delay) * c.jitter()) // full jitter
		if te.RetryAfter > 0 {
			delay = te.RetryAfter
		}
		if serr := c.sleep(ctx, delay); serr != nil {
			return nil, &Error{Category: CategoryNetwork, Message: "cancelled during backoff: " + serr.Error()}
		}
	}
	return nil, lastErr
}

func (c *HTTPClient) ListModels(ctx context.Context) ([]Model, error) {
	raw, err := c.do(ctx, http.MethodGet, "/v1/models", nil)
	if err != nil {
		return nil, err
	}
	var out struct {
		Models []Model `json:"models"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, &Error{Category: CategoryParse, Message: "decode models: " + err.Error()}
	}
	return out.Models, nil
}

// do performs one attempt and classifies the outcome. Success returns the raw body (≤ MaxResponseBytes).
func (c *HTTPClient) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	httpReq, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, rdr)
	if err != nil {
		return nil, &Error{Category: CategoryInternal, Message: err.Error()}
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, classifyTransport(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBytes+1))
	if err != nil {
		return nil, classifyTransport(err)
	}
	if len(raw) > MaxResponseBytes {
		return nil, &Error{Status: resp.StatusCode, Category: CategoryParse, Message: "response exceeds size limit"}
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return raw, nil
	}
	return nil, classifyStatus(resp.StatusCode, resp.Header.Get("Retry-After"), raw)
}

func classifyTransport(err error) *Error {
	var ne net.Error
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout()) {
		return &Error{Category: CategoryTimeout, Message: "request timed out"}
	}
	if errors.Is(err, context.Canceled) {
		return &Error{Category: CategoryNetwork, Message: "request cancelled"}
	}
	return &Error{Category: CategoryNetwork, Message: "network error: " + err.Error()}
}

func classifyStatus(status int, retryAfter string, body []byte) *Error {
	msg := truncateBody(body)
	e := &Error{Status: status, Message: msg}
	switch {
	case status == 401:
		e.Category = CategoryAuth
	case status == 422:
		e.Category = CategoryValidation
		lower := strings.ToLower(msg)
		hasContextOrToken := strings.Contains(lower, "context") || strings.Contains(lower, "token")
		hasLimitOrExceed := strings.Contains(lower, "limit") || strings.Contains(lower, "exceed") || strings.Contains(lower, "too many")
		e.Oversized = hasContextOrToken && hasLimitOrExceed
	case status == 429:
		e.Category = CategoryRateLimit
		e.RetryAfter = parseRetryAfter(retryAfter)
	case status == 529:
		e.Category = CategoryOverloaded
		e.RetryAfter = parseRetryAfter(retryAfter)
	default:
		e.Category = CategoryInternal
	}
	return e
}

// truncateBody keeps a short, single-line excerpt of the vendor error body for admin
// diagnostics (the settings "Test connection" result and warn logs). Vendor 4xx bodies can echo
// request content (i.e. redacted failure text), so the excerpt is short and never reaches an
// analysis row: the rationale prefix uses only the category.
func truncateBody(b []byte) string {
	s := strings.Join(strings.Fields(string(b)), " ")
	if len(s) > 160 {
		s = s[:160]
	}
	return s
}

// parseRetryAfter accepts integer seconds or an HTTP date; capped at maxRetryAfter.
func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	var d time.Duration
	if secs, err := strconv.Atoi(v); err == nil {
		d = time.Duration(secs) * time.Second
	} else if t, err := http.ParseTime(v); err == nil {
		d = time.Until(t)
	}
	if d < 0 {
		d = 0
	}
	if d > maxRetryAfter {
		d = maxRetryAfter
	}
	return d
}

func finite01(f *float64) bool {
	return f != nil && !math.IsNaN(*f) && !math.IsInf(*f, 0) && *f >= 0 && *f <= 1
}

// validate decodes and checks the response against the request; any violation is CategoryParse.
func validate(raw []byte, req Request) (*Response, error) {
	var w wireResponse
	if err := json.Unmarshal(raw, &w); err != nil {
		return nil, &Error{Category: CategoryParse, Message: "decode response: " + err.Error()}
	}
	bad := func(f string, a ...any) (*Response, error) {
		return nil, &Error{Category: CategoryParse, Message: fmt.Sprintf(f, a...)}
	}
	if w.Model == nil || *w.Model == "" {
		return bad("response missing model")
	}
	if w.Usage == nil || w.Usage.InputTokens == nil {
		return bad("response missing usage.input_tokens")
	}
	out := &Response{Model: *w.Model, Answers: make(map[string]Answer, len(req.Questions)),
		Usage: Usage{InputTokens: *w.Usage.InputTokens}}
	if w.Usage.OutputTokens != nil {
		out.Usage.OutputTokens = *w.Usage.OutputTokens
	}
	for id, q := range req.Questions {
		a, ok := w.Answers[id]
		if !ok {
			return bad("answer %q missing", id)
		}
		if a.Type == nil || *a.Type != q.Type {
			gotType := "<missing>"
			if a.Type != nil {
				gotType = *a.Type
			}
			return bad("answer %q has type %q, want %q", id, gotType, q.Type)
		}
		ans := Answer{Type: q.Type, Legend: a.Legend}
		switch q.Type {
		case "noul":
			if !finite01(a.Noul) {
				return bad("answer %q noul missing or out of range", id)
			}
			ans.Noul = *a.Noul
		case "choice":
			opts, ok := q.Criteria.(map[string]any)
			if !ok {
				return bad("question %q criteria is not a map", id)
			}
			if a.Choice == nil {
				return bad("answer %q choice missing", id)
			}
			if _, ok := opts[*a.Choice]; !ok {
				return bad("answer %q choice %q not in options", id, *a.Choice)
			}
			if !finite01(a.Confidence) {
				return bad("answer %q confidence missing or out of range", id)
			}
			if len(a.Probabilities) != len(opts) {
				return bad("answer %q probabilities cover %d of %d options", id, len(a.Probabilities), len(opts))
			}
			sum := 0.0
			probs := make(map[string]float64, len(opts))
			for k := range opts {
				p, ok := a.Probabilities[k]
				if !ok || !finite01(p) {
					return bad("answer %q probability for %q missing or out of range", id, k)
				}
				probs[k] = *p
				sum += *p
			}
			if math.Abs(sum-1) > ProbabilitySumTolerance {
				return bad("answer %q probabilities sum to %.3f", id, sum)
			}
			ans.Choice, ans.Confidence, ans.Probabilities = *a.Choice, *a.Confidence, probs
		case "score":
			if a.Score == nil || math.IsNaN(*a.Score) || math.IsInf(*a.Score, 0) || !finite01(a.Confidence) {
				return bad("answer %q score/confidence missing", id)
			}
			ans.Score, ans.Confidence = *a.Score, *a.Confidence
			ans.Probabilities = make(map[string]float64, len(a.Probabilities))
			for k, p := range a.Probabilities {
				if !finite01(p) {
					return bad("answer %q probability for %q out of range", id, k)
				}
				ans.Probabilities[k] = *p
			}
		default:
			return bad("question %q has unsupported type %q", id, q.Type)
		}
		out.Answers[id] = ans
	}
	return out, nil
}
