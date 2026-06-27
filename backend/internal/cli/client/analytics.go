package client

import "encoding/json"

// AnalyticsGet performs a GET on an analytics endpoint with standard date/folder params.
func (c *Client) AnalyticsGet(path string, params map[string]string) (json.RawMessage, error) {
	raw, _, err := c.GetRaw("/api/analytics/"+path, params)
	return raw, err
}
