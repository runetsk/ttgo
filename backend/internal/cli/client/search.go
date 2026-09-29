package client

import (
	"encoding/json"
	"fmt"
)

// Search performs a full-text search across tests, requirements, and runs.
func (c *Client) Search(query string, limit, offset int, rerank bool) (json.RawMessage, error) {
	params := map[string]string{"q": query}
	if limit > 0 {
		params["limit"] = fmt.Sprintf("%d", limit)
	}
	if offset > 0 {
		params["offset"] = fmt.Sprintf("%d", offset)
	}
	if rerank {
		params["rerank"] = "true" // TypeSafe.ai re-ranks the first page when its search re-ranking is on
	}
	raw, _, err := c.GetRaw("/api/search", params)
	return raw, err
}
