package api

import (
	"encoding/json"
	"fmt"
)

// Volume represents a Railway volume
type Volume struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// VolumeInstance represents a volume instance in an environment. ID is the
// volumeInstanceId used by backup operations.
type VolumeInstance struct {
	ID            string  `json:"id"`
	Volume        Volume  `json:"volume"`
	MountPath     string  `json:"mountPath"`
	ServiceID     *string `json:"serviceId"`
	CurrentSizeMB float64 `json:"currentSizeMB"`
	SizeMB        int     `json:"sizeMB"`
	// Region is the region the volume instance lives in (volumes are
	// region-placed; Railway migrates them when their service moves).
	Region string `json:"region"`
}

// ListVolumes retrieves all volumes for a project environment
func (c *Client) ListVolumes(projectID, environmentID string) ([]VolumeInstance, error) {
	query := `
		query GetVolumes($projectId: String!) {
			project(id: $projectId) {
				environments {
					edges {
						node {
							id
							volumeInstances {
								edges {
									node {
										id
										volume {
											id
											name
										}
										mountPath
										serviceId
										currentSizeMB
										sizeMB
										region
									}
								}
							}
						}
					}
				}
			}
		}
	`

	variables := map[string]any{
		"projectId": projectID,
	}

	data, err := c.execute(query, variables)
	if err != nil {
		return nil, err
	}

	var result struct {
		Project struct {
			Environments struct {
				Edges []struct {
					Node struct {
						ID              string `json:"id"`
						VolumeInstances struct {
							Edges []struct {
								Node VolumeInstance `json:"node"`
							} `json:"edges"`
						} `json:"volumeInstances"`
					} `json:"node"`
				} `json:"edges"`
			} `json:"environments"`
		} `json:"project"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Find the matching environment and return its volumes
	for _, env := range result.Project.Environments.Edges {
		if env.Node.ID == environmentID {
			volumes := make([]VolumeInstance, 0, len(env.Node.VolumeInstances.Edges))
			for _, edge := range env.Node.VolumeInstances.Edges {
				volumes = append(volumes, edge.Node)
			}
			return volumes, nil
		}
	}

	return []VolumeInstance{}, nil
}

// CreateVolume creates a new volume attached to a service
func (c *Client) CreateVolume(projectID, environmentID, serviceID, mountPath string) (Volume, error) {
	mutation := `
		mutation VolumeCreate($projectId: String!, $environmentId: String!, $serviceId: String!, $mountPath: String!) {
			volumeCreate(
				input: {projectId: $projectId, environmentId: $environmentId, serviceId: $serviceId, mountPath: $mountPath}
			) {
				id
				name
			}
		}
	`

	variables := map[string]any{
		"projectId":     projectID,
		"environmentId": environmentID,
		"serviceId":     serviceID,
		"mountPath":     mountPath,
	}

	data, err := c.execute(mutation, variables)
	if err != nil {
		return Volume{}, err
	}

	var result struct {
		VolumeCreate Volume `json:"volumeCreate"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return Volume{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result.VolumeCreate, nil
}

// DeleteVolume deletes a volume by committing {volumes:{<id>:{isDeleted:true}}}
// to the environment config — the same environmentPatchCommit mechanism the
// Railway dashboard uses. The standalone volumeDelete(volumeId) mutation is
// denied to project tokens (it carries no environmentId; the official Railway
// CLI fails identically), while the patch path is authorized for every token
// scope. Verified live 2026-08-30.
func (c *Client) DeleteVolume(environmentID, volumeID string) error {
	patch := map[string]any{
		"volumes": map[string]any{
			volumeID: map[string]any{
				"isDeleted": true,
			},
		},
	}
	_, err := c.execute(environmentPatchCommitMutation, map[string]any{
		"environmentId": environmentID,
		"patch":         patch,
		"commitMessage": "railctl: delete volume",
	})
	return err
}

// UpdateVolumeName updates the name of a volume
func (c *Client) UpdateVolumeName(volumeID, name string) error {
	mutation := `
		mutation VolumeNameUpdate($volumeId: String!, $name: String!) {
			volumeUpdate(volumeId: $volumeId, input: {name: $name}) {
				id
				name
			}
		}
	`

	variables := map[string]any{
		"volumeId": volumeID,
		"name":     name,
	}

	_, err := c.execute(mutation, variables)
	return err
}

// UpdateVolumeMountPath updates the mount path of a volume
func (c *Client) UpdateVolumeMountPath(volumeID, serviceID, environmentID, mountPath string) error {
	mutation := `
		mutation VolumeMountPathUpdate($volumeId: String!, $serviceId: String, $environmentId: String!, $mountPath: String!) {
			volumeInstanceUpdate(
				volumeId: $volumeId
				environmentId: $environmentId
				input: {serviceId: $serviceId, mountPath: $mountPath}
			)
		}
	`

	variables := map[string]any{
		"volumeId":      volumeID,
		"serviceId":     serviceID,
		"environmentId": environmentID,
		"mountPath":     mountPath,
	}

	data, err := c.execute(mutation, variables)
	if err != nil {
		return err
	}

	var result struct {
		VolumeInstanceUpdate bool `json:"volumeInstanceUpdate"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return nil
}

// AttachVolume attaches a volume to a service
func (c *Client) AttachVolume(volumeID, serviceID, environmentID string) error {
	mutation := `
		mutation VolumeAttach($environmentId: String!, $volumeId: String!, $serviceId: String!) {
			volumeInstanceUpdate(
				input: { serviceId: $serviceId }
				environmentId: $environmentId
				volumeId: $volumeId
			)
		}
	`

	variables :=

		map[string]any{
			"volumeId":      volumeID,
			"serviceId":     serviceID,
			"environmentId": environmentID,
		}

	data, err := c.execute(mutation, variables)
	if err != nil {
		return err
	}

	var result struct {
		VolumeInstanceUpdate bool `json:"volumeInstanceUpdate"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !result.VolumeInstanceUpdate {
		return fmt.Errorf("failed to attach volume")
	}

	return nil
}

// DetachVolume detaches a volume from its service
func (c *Client) DetachVolume(volumeID, environmentID string) error {
	mutation := `
		mutation VolumeDetach($environmentId: String!, $volumeId: String!) {
			volumeInstanceUpdate(
				input: { serviceId: null }
				environmentId: $environmentId
				volumeId: $volumeId
			)
		}
	`

	variables := map[string]any{
		"volumeId":      volumeID,
		"environmentId": environmentID,
	}

	data, err := c.execute(mutation, variables)
	if err != nil {
		return err
	}

	var result struct {
		VolumeInstanceUpdate bool `json:"volumeInstanceUpdate"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !result.VolumeInstanceUpdate {
		return fmt.Errorf("failed to detach volume")
	}

	return nil
}
