package client

import "encoding/json"

// ListRuns returns test runs, optionally filtered.
func (c *Client) ListRuns(params map[string]string) (json.RawMessage, error) {
	raw, _, err := c.GetRaw("/api/runs", params)
	return raw, err
}

// GetRun returns a single test run by ID.
func (c *Client) GetRun(id string) (json.RawMessage, error) {
	raw, _, err := c.GetRaw("/api/runs/"+id, nil)
	return raw, err
}

// CreateRun creates a new test run.
func (c *Client) CreateRun(name, categoryID, folderID string) (map[string]interface{}, error) {
	body := map[string]interface{}{"name": name}
	if categoryID != "" {
		body["category_id"] = categoryID
	}
	if folderID != "" {
		body["run_folder_id"] = folderID
	}
	var result map[string]interface{}
	err := c.Post("/api/runs", body, &result)
	return result, err
}

// CompleteRun completes a test run.
func (c *Client) CompleteRun(id string) (json.RawMessage, error) {
	raw, _, err := c.PostRaw("/api/runs/"+id+"/complete", nil)
	return raw, err
}

// ReopenRun reopens a completed test run.
func (c *Client) ReopenRun(id string) (json.RawMessage, error) {
	raw, _, err := c.PostRaw("/api/runs/"+id+"/reopen", nil)
	return raw, err
}

// CopyRun copies a test run.
func (c *Client) CopyRun(id string) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := c.Post("/api/runs/"+id+"/copy", map[string]string{"name": ""}, &result)
	return result, err
}

// DeleteRun deletes a test run.
func (c *Client) DeleteRun(id string) error {
	return c.Delete("/api/runs/"+id, nil)
}

// AddRunResult adds a result to a run.
func (c *Client) AddRunResult(runID, testCaseID, status string, extra map[string]interface{}) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"test_case_id": testCaseID,
		"status":       status,
	}
	for k, v := range extra {
		body[k] = v
	}
	var result map[string]interface{}
	err := c.Post("/api/runs/"+runID+"/results", body, &result)
	return result, err
}

// UpdateRunResult updates a result within a run.
func (c *Client) UpdateRunResult(runID, resultID string, fields map[string]interface{}) (json.RawMessage, error) {
	raw, _, err := c.doRaw("PUT", "/api/runs/"+runID+"/results/"+resultID, nil, fields)
	return raw, err
}

// RetryRunResult increments the attempt number for a result.
func (c *Client) RetryRunResult(runID, resultID string) (json.RawMessage, error) {
	raw, _, err := c.PostRaw("/api/runs/"+runID+"/results/"+resultID+"/retry", nil)
	return raw, err
}

// BulkUpdateRunResults updates multiple results at once.
func (c *Client) BulkUpdateRunResults(runID string, resultIDs []string, status, defectType string) (json.RawMessage, error) {
	body := map[string]interface{}{
		"result_ids": resultIDs,
		"status":     status,
	}
	if defectType != "" {
		body["defect_type"] = defectType
	}
	raw, _, err := c.PostRaw("/api/runs/"+runID+"/results/bulk-update", body)
	return raw, err
}

// DeleteRunResult deletes a result from a run.
func (c *Client) DeleteRunResult(runID, resultID string) error {
	return c.Delete("/api/runs/"+runID+"/results/"+resultID, nil)
}
