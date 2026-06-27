package client

import "encoding/json"

// GetFolderTree returns the full folder hierarchy.
func (c *Client) GetFolderTree() ([]json.RawMessage, error) {
	var result []json.RawMessage
	err := c.Get("/api/folders/tree", nil, &result)
	return result, err
}

// GetFolderTreeRaw returns the raw JSON for the folder tree.
func (c *Client) GetFolderTreeRaw() (json.RawMessage, error) {
	raw, _, err := c.GetRaw("/api/folders/tree", nil)
	return raw, err
}

// GetFolder returns a folder by ID.
func (c *Client) GetFolder(id string) (json.RawMessage, error) {
	raw, _, err := c.GetRaw("/api/folders/"+id, nil)
	return raw, err
}

// CreateFolder creates a new folder.
func (c *Client) CreateFolder(name string, parentID *string) (map[string]interface{}, error) {
	body := map[string]interface{}{"name": name}
	if parentID != nil {
		body["parent_id"] = *parentID
	}
	var result map[string]interface{}
	err := c.Post("/api/folders", body, &result)
	return result, err
}

// RenameFolder renames a folder.
func (c *Client) RenameFolder(id, name string) error {
	return c.Patch("/api/folders/"+id, map[string]string{"name": name}, nil)
}

// MoveFolder moves a folder to a new parent.
func (c *Client) MoveFolder(id string, parentID *string) error {
	body := map[string]interface{}{}
	if parentID != nil {
		body["parent_id"] = *parentID
	}
	return c.Patch("/api/folders/"+id+"/parent", body, nil)
}

// DeleteFolder deletes a folder.
func (c *Client) DeleteFolder(id string) error {
	return c.Delete("/api/folders/"+id, nil)
}
