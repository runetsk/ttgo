package client

import (
	"encoding/json"
	"strings"
)

// ListRequirements returns all requirements.
func (c *Client) ListRequirements() (json.RawMessage, error) {
	raw, _, err := c.GetRaw("/api/requirements", nil)
	return raw, err
}

// GetRequirement returns a single requirement.
func (c *Client) GetRequirement(id string) (json.RawMessage, error) {
	raw, _, err := c.GetRaw("/api/requirements/"+id, nil)
	return raw, err
}

// CreateRequirement creates a new requirement.
func (c *Client) CreateRequirement(identifier, title, description string) (map[string]interface{}, error) {
	body := map[string]string{
		"identifier":  identifier,
		"title":       title,
		"description": description,
	}
	var result map[string]interface{}
	err := c.Post("/api/requirements", body, &result)
	return result, err
}

// UpdateRequirement updates a requirement.
func (c *Client) UpdateRequirement(id string, fields map[string]string) (json.RawMessage, error) {
	raw, _, err := c.doRaw("PUT", "/api/requirements/"+id, nil, fields)
	return raw, err
}

// DeleteRequirement deletes a requirement.
func (c *Client) DeleteRequirement(id string) error {
	return c.Delete("/api/requirements/"+id, nil)
}

// LinkRequirement links a test case to a requirement.
func (c *Client) LinkRequirement(reqID, testCaseID string) error {
	return c.Post("/api/requirements/"+reqID+"/links", map[string]string{"test_case_id": testCaseID}, nil)
}

// UnlinkRequirement removes a test case link.
func (c *Client) UnlinkRequirement(reqID, testCaseID string) error {
	return c.Delete("/api/requirements/"+reqID+"/links/"+testCaseID, nil)
}

// ImportRequirement imports from Jira or Confluence.
func (c *Client) ImportRequirement(sourceType, sourceKey string) (json.RawMessage, error) {
	raw, _, err := c.PostRaw("/api/requirements/import", map[string]string{
		"source_type": sourceType,
		"source_key":  sourceKey,
	})
	return raw, err
}

// BulkImportRequirements imports multiple requirements.
func (c *Client) BulkImportRequirements(sourceType string, sourceKeys []string) (json.RawMessage, error) {
	raw, _, err := c.PostRaw("/api/requirements/bulk-import", map[string]interface{}{
		"source_type": sourceType,
		"source_keys": sourceKeys,
	})
	return raw, err
}

// ResyncRequirement resyncs from external source.
func (c *Client) ResyncRequirement(id string) (json.RawMessage, error) {
	raw, _, err := c.PostRaw("/api/requirements/"+id+"/resync", nil)
	return raw, err
}

// PostRequirementToJira posts test cases to Jira.
func (c *Client) PostRequirementToJira(id string) (json.RawMessage, error) {
	raw, _, err := c.PostRaw("/api/requirements/"+id+"/post-to-jira", nil)
	return raw, err
}

// ParseCSV splits comma-separated string into slice.
func ParseCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
