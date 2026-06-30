package client

import "encoding/json"

func (c *Client) ListDefects(params map[string]string) (json.RawMessage, error) {
	raw, _, err := c.GetRaw("/api/defects", params)
	return raw, err
}

func (c *Client) CreateDefect(body map[string]interface{}) (json.RawMessage, error) {
	raw, _, err := c.PostRaw("/api/defects", body)
	return raw, err
}

func (c *Client) LinkDefect(runID, resultID, defectID string) error {
	return c.Post("/api/runs/"+runID+"/results/"+resultID+"/defect-links",
		map[string]string{"defect_id": defectID}, nil)
}

func (c *Client) UnlinkDefect(runID, resultID, defectID string) error {
	return c.Delete("/api/runs/"+runID+"/results/"+resultID+"/defect-links/"+defectID, nil)
}
