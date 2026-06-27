package client

import "encoding/json"

// ListTestDefects returns defect links for a test case.
func (c *Client) ListTestDefects(testID string) (json.RawMessage, error) {
	raw, _, err := c.GetRaw("/api/tests/"+testID+"/defect-links", nil)
	return raw, err
}

// ListRunDefects returns defect links for a run.
func (c *Client) ListRunDefects(runID string) (json.RawMessage, error) {
	raw, _, err := c.GetRaw("/api/runs/"+runID+"/defect-links", nil)
	return raw, err
}

// ListResultDefects returns defect links for a specific result.
func (c *Client) ListResultDefects(runID, resultID string) (json.RawMessage, error) {
	raw, _, err := c.GetRaw("/api/runs/"+runID+"/results/"+resultID+"/defect-links", nil)
	return raw, err
}

// LinkDefect links a Jira issue to a run result.
func (c *Client) LinkDefect(runID, resultID, jiraKey string) error {
	return c.Post("/api/runs/"+runID+"/results/"+resultID+"/defect-links",
		map[string]string{"jira_issue_key": jiraKey}, nil)
}

// UnlinkDefect removes a Jira issue link from a run result.
func (c *Client) UnlinkDefect(runID, resultID, jiraKey string) error {
	return c.Delete("/api/runs/"+runID+"/results/"+resultID+"/defect-links/"+jiraKey, nil)
}

// CreateDefectIssue creates a new Jira issue from a test case.
func (c *Client) CreateDefectIssue(testID string) (json.RawMessage, error) {
	raw, _, err := c.PostRaw("/api/tests/"+testID+"/defect-links/create-issue", nil)
	return raw, err
}
