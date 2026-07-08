package api

import (
	"encoding/json"
	"fmt"
)

// Volume backup schedule kinds (Railway's VolumeInstanceBackupScheduleKind).
const (
	BackupScheduleDaily   = "DAILY"
	BackupScheduleWeekly  = "WEEKLY"
	BackupScheduleMonthly = "MONTHLY"
)

// BackupSchedule is an automated backup schedule on a volume instance.
// Retention is fixed by Railway per kind.
type BackupSchedule struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Cron             string `json:"cron"`
	Kind             string `json:"kind"`
	RetentionSeconds int    `json:"retentionSeconds"`
}

// VolumeBackup represents a single backup of a volume instance.
type VolumeBackup struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	CreatedAt    string `json:"createdAt"`
	ExpiresAt    string `json:"expiresAt"`
	UsedMB       int    `json:"usedMB"`
	ReferencedMB int    `json:"referencedMB"`
	ScheduleID   string `json:"scheduleId"`
}

// ListVolumeBackupSchedules returns a volume instance's backup schedules.
func (c *Client) ListVolumeBackupSchedules(volumeInstanceID string) ([]BackupSchedule, error) {
	query := `
		query BackupSchedules($volumeInstanceId: String!) {
			volumeInstanceBackupScheduleList(volumeInstanceId: $volumeInstanceId) {
				id
				name
				cron
				kind
				retentionSeconds
			}
		}
	`

	variables := map[string]any{
		"volumeInstanceId": volumeInstanceID,
	}

	data, err := c.execute(query, variables)
	if err != nil {
		return nil, err
	}

	var result struct {
		Schedules []BackupSchedule `json:"volumeInstanceBackupScheduleList"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result.Schedules, nil
}

// SetVolumeBackupSchedules replaces a volume instance's schedules with the
// given kinds (DAILY/WEEKLY/MONTHLY). An empty slice clears all schedules.
func (c *Client) SetVolumeBackupSchedules(volumeInstanceID string, kinds []string) error {
	mutation := `
		mutation BackupScheduleUpdate($volumeInstanceId: String!, $kinds: [VolumeInstanceBackupScheduleKind!]!) {
			volumeInstanceBackupScheduleUpdate(volumeInstanceId: $volumeInstanceId, kinds: $kinds)
		}
	`

	// Send [] rather than null when clearing.
	if kinds == nil {
		kinds = []string{}
	}

	variables := map[string]any{
		"volumeInstanceId": volumeInstanceID,
		"kinds":            kinds,
	}

	_, err := c.execute(mutation, variables)
	return err
}

// ListVolumeBackups returns the backups for a volume instance.
func (c *Client) ListVolumeBackups(volumeInstanceID string) ([]VolumeBackup, error) {
	query := `
		query VolumeBackups($volumeInstanceId: String!) {
			volumeInstanceBackupList(volumeInstanceId: $volumeInstanceId) {
				id
				name
				createdAt
				expiresAt
				usedMB
				referencedMB
				scheduleId
			}
		}
	`

	variables := map[string]any{
		"volumeInstanceId": volumeInstanceID,
	}

	data, err := c.execute(query, variables)
	if err != nil {
		return nil, err
	}

	var result struct {
		Backups []VolumeBackup `json:"volumeInstanceBackupList"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result.Backups, nil
}

// CreateVolumeBackup requests a manual backup (async). An empty name lets
// Railway assign one. Returns the backup workflow ID.
func (c *Client) CreateVolumeBackup(volumeInstanceID, name string) (string, error) {
	mutation := `
		mutation VolumeBackupCreate($volumeInstanceId: String!, $name: String) {
			volumeInstanceBackupCreate(volumeInstanceId: $volumeInstanceId, name: $name) {
				workflowId
			}
		}
	`

	variables := map[string]any{"volumeInstanceId": volumeInstanceID, "name": nil}
	if name != "" {
		variables["name"] = name
	}

	data, err := c.execute(mutation, variables)
	if err != nil {
		return "", err
	}

	var result struct {
		Result struct {
			WorkflowID string `json:"workflowId"`
		} `json:"volumeInstanceBackupCreate"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result.Result.WorkflowID, nil
}

// RestoreVolumeBackup restores a volume instance from a backup. Railway stages
// a new volume (finalized by a deploy) and drops backups newer than the restore point.
func (c *Client) RestoreVolumeBackup(backupID, volumeInstanceID string) error {
	mutation := `
		mutation VolumeBackupRestore($volumeInstanceBackupId: String!, $volumeInstanceId: String!) {
			volumeInstanceBackupRestore(volumeInstanceBackupId: $volumeInstanceBackupId, volumeInstanceId: $volumeInstanceId) {
				workflowId
			}
		}
	`

	variables := map[string]any{
		"volumeInstanceBackupId": backupID,
		"volumeInstanceId":       volumeInstanceID,
	}

	_, err := c.execute(mutation, variables)
	return err
}

// DeleteVolumeBackup deletes a backup of a volume instance.
func (c *Client) DeleteVolumeBackup(backupID, volumeInstanceID string) error {
	mutation := `
		mutation VolumeBackupDelete($volumeInstanceBackupId: String!, $volumeInstanceId: String!) {
			volumeInstanceBackupDelete(volumeInstanceBackupId: $volumeInstanceBackupId, volumeInstanceId: $volumeInstanceId) {
				workflowId
			}
		}
	`

	variables := map[string]any{
		"volumeInstanceBackupId": backupID,
		"volumeInstanceId":       volumeInstanceID,
	}

	_, err := c.execute(mutation, variables)
	return err
}

// LockVolumeBackup prevents a backup from expiring.
func (c *Client) LockVolumeBackup(backupID, volumeInstanceID string) error {
	mutation := `
		mutation VolumeBackupLock($volumeInstanceBackupId: String!, $volumeInstanceId: String!) {
			volumeInstanceBackupLock(volumeInstanceBackupId: $volumeInstanceBackupId, volumeInstanceId: $volumeInstanceId)
		}
	`

	variables := map[string]any{
		"volumeInstanceBackupId": backupID,
		"volumeInstanceId":       volumeInstanceID,
	}

	_, err := c.execute(mutation, variables)
	return err
}
