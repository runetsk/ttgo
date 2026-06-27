package client

import (
	"encoding/json"
	"fmt"
)

// ListCategories returns categories with optional search.
func (c *Client) ListCategories(search string, limit, offset int) (json.RawMessage, error) {
	params := map[string]string{}
	if search != "" {
		params["q"] = search
	}
	if limit > 0 {
		params["limit"] = fmt.Sprintf("%d", limit)
	}
	if offset > 0 {
		params["offset"] = fmt.Sprintf("%d", offset)
	}
	raw, _, err := c.GetRaw("/api/categories", params)
	return raw, err
}

// CreateCategory creates a new category.
func (c *Client) CreateCategory(name, description string) (map[string]interface{}, error) {
	body := map[string]string{"name": name}
	if description != "" {
		body["description"] = description
	}
	var result map[string]interface{}
	err := c.Post("/api/categories", body, &result)
	return result, err
}

// DeleteCategory deletes a category.
func (c *Client) DeleteCategory(id string) error {
	return c.Delete("/api/categories/"+id, nil)
}

// AssignCategory assigns a test case to a category.
func (c *Client) AssignCategory(testID, categoryID string) error {
	return c.Post("/api/tests/"+testID+"/categories", map[string]string{"category_id": categoryID}, nil)
}
