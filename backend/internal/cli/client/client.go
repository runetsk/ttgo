package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// APIError represents a non-2xx response from the TTGO server.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Message)
}

// Client is an HTTP client for the TTGO REST API.
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// New creates a new TTGO API client.
func New(baseURL, token string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Token:   token,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Get performs a GET request. params are added as query parameters.
func (c *Client) Get(path string, params map[string]string, result interface{}) error {
	return c.do(http.MethodGet, path, params, nil, result)
}

// Post performs a POST request with a JSON body.
func (c *Client) Post(path string, body interface{}, result interface{}) error {
	return c.do(http.MethodPost, path, nil, body, result)
}

// Put performs a PUT request with a JSON body.
func (c *Client) Put(path string, body interface{}, result interface{}) error {
	return c.do(http.MethodPut, path, nil, body, result)
}

// Patch performs a PATCH request with a JSON body.
func (c *Client) Patch(path string, body interface{}, result interface{}) error {
	return c.do(http.MethodPatch, path, nil, body, result)
}

// Delete performs a DELETE request.
func (c *Client) Delete(path string, result interface{}) error {
	return c.do(http.MethodDelete, path, nil, nil, result)
}

// PostRaw performs a POST and returns the raw JSON bytes.
func (c *Client) PostRaw(path string, body interface{}) (json.RawMessage, int, error) {
	return c.doRaw(http.MethodPost, path, nil, body)
}

// GetRaw performs a GET and returns the raw JSON bytes.
func (c *Client) GetRaw(path string, params map[string]string) (json.RawMessage, int, error) {
	return c.doRaw(http.MethodGet, path, params, nil)
}

func (c *Client) do(method, path string, params map[string]string, body interface{}, result interface{}) error {
	raw, statusCode, err := c.doRaw(method, path, params, body)
	if err != nil {
		return err
	}

	if statusCode == http.StatusNoContent {
		return nil
	}

	if result != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, result); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}
	return nil
}

func (c *Client) doRaw(method, path string, params map[string]string, body interface{}) (json.RawMessage, int, error) {
	u := c.BaseURL + path

	if len(params) > 0 {
		q := url.Values{}
		for k, v := range params {
			if v != "" {
				q.Set(k, v)
			}
		}
		u += "?" + q.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("encoding request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, u, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("creating request: %w", err)
	}

	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("cannot connect to TTGO server at %s. Is it running? (%w)", c.BaseURL, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode >= 400 {
		apiErr := &APIError{StatusCode: resp.StatusCode}
		var errResp map[string]string
		if json.Unmarshal(respBody, &errResp) == nil {
			if msg, ok := errResp["error"]; ok {
				apiErr.Message = msg
			}
		}
		if apiErr.Message == "" {
			apiErr.Message = http.StatusText(resp.StatusCode)
		}
		return nil, resp.StatusCode, apiErr
	}

	return json.RawMessage(respBody), resp.StatusCode, nil
}
