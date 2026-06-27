package client

import "encoding/json"

// ListWebhooks returns all webhook configs.
func (c *Client) ListWebhooks() (json.RawMessage, error) {
	raw, _, err := c.GetRaw("/api/webhooks", nil)
	return raw, err
}

// CreateWebhook creates a new webhook.
func (c *Client) CreateWebhook(url, eventType, description string) (map[string]interface{}, error) {
	body := map[string]string{
		"url":        url,
		"event_type": eventType,
	}
	if description != "" {
		body["description"] = description
	}
	var result map[string]interface{}
	err := c.Post("/api/webhooks", body, &result)
	return result, err
}

// DeleteWebhook deletes a webhook.
func (c *Client) DeleteWebhook(id string) error {
	return c.Delete("/api/webhooks/"+id, nil)
}
