package client

import "encoding/json"

// ListTests returns test cases, optionally filtered by folder or category.
// If summary is true, the server returns the slimmer view=list response
// (omits full Steps/CustomValues, includes steps_count).
func (c *Client) ListTests(folderID, categoryID string, summary bool) (json.RawMessage, error) {
	params := map[string]string{}
	if folderID != "" {
		params["folder_id"] = folderID
	}
	if categoryID != "" {
		params["category_id"] = categoryID
	}
	if summary {
		params["view"] = "list"
	}
	raw, _, err := c.GetRaw("/api/tests", params)
	return raw, err
}

// GetTest returns a single test case by ID.
func (c *Client) GetTest(id string) (json.RawMessage, error) {
	raw, _, err := c.GetRaw("/api/tests/"+id, nil)
	return raw, err
}

// CreateTest creates a new test case.
func (c *Client) CreateTest(name, folderID, description string) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"name":      name,
		"folder_id": folderID,
	}
	if description != "" {
		body["description"] = description
	}
	var result map[string]interface{}
	err := c.Post("/api/tests", body, &result)
	return result, err
}

// UpdateTest updates a test case.
func (c *Client) UpdateTest(id string, fields map[string]interface{}) (json.RawMessage, error) {
	raw, _, err := c.doRaw("PUT", "/api/tests/"+id, nil, fields)
	return raw, err
}

// DeleteTest deletes a test case.
func (c *Client) DeleteTest(id string) error {
	return c.Delete("/api/tests/"+id, nil)
}

// ListTestVersions returns version history for a test case.
func (c *Client) ListTestVersions(id string) (json.RawMessage, error) {
	raw, _, err := c.GetRaw("/api/tests/"+id+"/versions", nil)
	return raw, err
}

// RestoreTestVersion restores a test case to a specific version.
func (c *Client) RestoreTestVersion(testID, versionID string) error {
	return c.Post("/api/tests/"+testID+"/versions/"+versionID+"/restore", nil, nil)
}

// ListTestExecutions returns execution history for a test case.
func (c *Client) ListTestExecutions(id string) (json.RawMessage, error) {
	raw, _, err := c.GetRaw("/api/tests/"+id+"/executions", nil)
	return raw, err
}
