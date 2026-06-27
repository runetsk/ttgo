package client

import "encoding/json"

// ListUsers returns all users.
func (c *Client) ListUsers(includeDeleted bool) (json.RawMessage, error) {
	params := map[string]string{}
	if includeDeleted {
		params["include_deleted"] = "true"
	}
	raw, _, err := c.GetRaw("/api/users", params)
	return raw, err
}

// CreateUser creates a new user.
func (c *Client) CreateUser(email, displayName, password, role string) (map[string]interface{}, error) {
	body := map[string]string{
		"email":        email,
		"display_name": displayName,
		"password":     password,
	}
	if role != "" {
		body["role"] = role
	}
	var result map[string]interface{}
	err := c.Post("/api/users", body, &result)
	return result, err
}

// UpdateUser updates a user.
func (c *Client) UpdateUser(id string, fields map[string]interface{}) (json.RawMessage, error) {
	raw, _, err := c.doRaw("PATCH", "/api/users/"+id, nil, fields)
	return raw, err
}

// DeleteUser soft-deletes a user.
func (c *Client) DeleteUser(id string) error {
	return c.Delete("/api/users/"+id, nil)
}

// RestoreUser restores a soft-deleted user.
func (c *Client) RestoreUser(id string) error {
	_, _, err := c.PostRaw("/api/users/"+id+"/restore", nil)
	return err
}
