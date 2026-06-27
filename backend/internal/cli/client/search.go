package client

import (
	"encoding/json"
	"fmt"
)

// Search performs a full-text search across tests, requirements, and runs.
func (c *Client) Search(query string, limit, offset int) (json.RawMessage, error) {
	params := map[string]string{"q": query}
	if limit > 0 {
		params["limit"] = fmt.Sprintf("%d", limit)
	}
	if offset > 0 {
		params["offset"] = fmt.Sprintf("%d", offset)
	}
	raw, _, err := c.GetRaw("/api/search", params)
	return raw, err
}
