package cmd

import (
	"strings"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/resolver"
)

// resolveVolumeInstance finds a volume instance by volume name or ID, following
// the resolver contract: exact match first, then case-insensitive substring,
// then decide by count. Its ID is the volumeInstanceId used by backup operations.
func resolveVolumeInstance(client api.APIClient, projectID, environmentID, volumeNameOrID string) (*api.VolumeInstance, error) {
	volumes, err := client.ListVolumes(projectID, environmentID)
	if err != nil {
		return nil, err
	}

	// Exact match by ID or name.
	for i := range volumes {
		if volumes[i].Volume.ID == volumeNameOrID || volumes[i].Volume.Name == volumeNameOrID {
			return &volumes[i], nil
		}
	}

	// Case-insensitive substring match on name.
	var matches []api.VolumeInstance
	query := strings.ToLower(volumeNameOrID)
	for i := range volumes {
		if strings.Contains(strings.ToLower(volumes[i].Volume.Name), query) {
			matches = append(matches, volumes[i])
		}
	}

	switch len(matches) {
	case 0:
		return nil, resolver.ErrNotFound{Resource: "volume", Name: volumeNameOrID}
	case 1:
		return &matches[0], nil
	default:
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = m.Volume.Name
		}
		return nil, resolver.ErrAmbiguous{Resource: "volume", Name: volumeNameOrID, Matches: names}
	}
}
