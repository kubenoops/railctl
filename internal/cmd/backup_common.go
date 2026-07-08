package cmd

import (
	"fmt"
	"time"

	"github.com/kubenoops/railctl/internal/api"
)

// resolveVolumeInstance finds a volume instance by volume name or ID. Its ID
// is the volumeInstanceId used by backup operations.
func resolveVolumeInstance(client api.APIClient, projectID, environmentID, volumeNameOrID string) (*api.VolumeInstance, error) {
	volumes, err := client.ListVolumes(projectID, environmentID)
	if err != nil {
		return nil, err
	}
	for i := range volumes {
		if volumes[i].Volume.Name == volumeNameOrID || volumes[i].Volume.ID == volumeNameOrID {
			return &volumes[i], nil
		}
	}
	return nil, fmt.Errorf("volume '%s' not found in environment", volumeNameOrID)
}

// formatBackupTime renders an RFC3339 timestamp as "2006-01-02 15:04".
func formatBackupTime(ts string) string {
	if ts == "" {
		return "-"
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts
	}
	return t.Format("2006-01-02 15:04")
}
