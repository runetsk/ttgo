package client

import "encoding/json"

// ListBackups returns all backups.
func (c *Client) ListBackups() (json.RawMessage, error) {
	raw, _, err := c.GetRaw("/api/backups", nil)
	return raw, err
}

// CreateBackup triggers a manual backup.
func (c *Client) CreateBackup() (json.RawMessage, error) {
	raw, _, err := c.PostRaw("/api/backups", nil)
	return raw, err
}

// RestoreBackup restores from a backup.
func (c *Client) RestoreBackup(id string) (json.RawMessage, error) {
	raw, _, err := c.PostRaw("/api/backups/"+id+"/restore", nil)
	return raw, err
}

// DeleteBackup deletes a backup.
func (c *Client) DeleteBackup(id string) error {
	return c.Delete("/api/backups/"+id, nil)
}

// GetBackupSchedule returns the backup schedule config.
func (c *Client) GetBackupSchedule() (json.RawMessage, error) {
	raw, _, err := c.GetRaw("/api/settings/backup-schedule", nil)
	return raw, err
}

// SetBackupSchedule updates the backup schedule.
func (c *Client) SetBackupSchedule(cron string) (json.RawMessage, error) {
	raw, _, err := c.doRaw("PUT", "/api/settings/backup-schedule", nil, map[string]string{"cron": cron})
	return raw, err
}
