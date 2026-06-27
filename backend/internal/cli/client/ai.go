package client

import "encoding/json"

// ListLLMProviders returns all LLM provider configs.
func (c *Client) ListLLMProviders() (json.RawMessage, error) {
	raw, _, err := c.GetRaw("/api/settings/llm-providers", nil)
	return raw, err
}

// CreateLLMProvider creates a new LLM provider.
func (c *Client) CreateLLMProvider(label, providerType, endpoint, model string) (map[string]interface{}, error) {
	body := map[string]string{
		"label":         label,
		"provider_type": providerType,
		"endpoint_url":  endpoint,
	}
	if model != "" {
		body["model_name"] = model
	}
	var result map[string]interface{}
	err := c.Post("/api/settings/llm-providers", body, &result)
	return result, err
}

// TestLLMProvider tests a provider connection.
func (c *Client) TestLLMProvider(id string) (json.RawMessage, error) {
	raw, _, err := c.PostRaw("/api/settings/llm-providers/"+id+"/test", nil)
	return raw, err
}

// SetDefaultLLMProvider sets a provider as default.
func (c *Client) SetDefaultLLMProvider(id string) error {
	_, _, err := c.PostRaw("/api/settings/llm-providers/"+id+"/set-default", nil)
	return err
}

// DeleteLLMProvider deletes a provider.
func (c *Client) DeleteLLMProvider(id string) error {
	return c.Delete("/api/settings/llm-providers/"+id, nil)
}

// GenerateTests generates tests from a requirement.
func (c *Client) GenerateTests(reqID, coverage string) (json.RawMessage, error) {
	body := map[string]string{}
	if coverage != "" {
		body["coverage"] = coverage
	}
	raw, _, err := c.PostRaw("/api/requirements/"+reqID+"/generate-tests", body)
	return raw, err
}

// AcceptGeneratedTests accepts generated tests for a requirement.
func (c *Client) AcceptGeneratedTests(reqID string) (json.RawMessage, error) {
	raw, _, err := c.PostRaw("/api/requirements/"+reqID+"/accept-generated-tests", nil)
	return raw, err
}

// GetAITemplate returns the current prompt template.
func (c *Client) GetAITemplate() (json.RawMessage, error) {
	raw, _, err := c.GetRaw("/api/settings/ai-gen-template", nil)
	return raw, err
}

// SetAITemplate updates the prompt template.
func (c *Client) SetAITemplate(content string) (json.RawMessage, error) {
	raw, _, err := c.doRaw("PUT", "/api/settings/ai-gen-template", nil, map[string]string{"content": content})
	return raw, err
}

// ResetAITemplate restores the default template.
func (c *Client) ResetAITemplate() error {
	_, _, err := c.PostRaw("/api/settings/ai-gen-template/reset", nil)
	return err
}
