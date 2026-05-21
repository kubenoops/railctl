package api

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/kubenoops/railctl/internal/types"
)

// listEnvironmentsQuery fetches environments for a project.
const listEnvironmentsQuery = `
query($id: String!) {
	project(id: $id) {
		environments {
			edges {
				node {
					id
					name
					updatedAt
					serviceInstances {
						edges {
							node {
								serviceId
								serviceName
							}
						}
					}
				}
			}
		}
	}
}
`

// createEnvironmentMutation creates a new environment in a project.
const createEnvironmentMutation = `
mutation($projectId: String!, $name: String!) {
	environmentCreate(input: { projectId: $projectId, name: $name }) {
		id
		name
	}
}
`

// deleteEnvironmentMutation deletes an environment by ID.
const deleteEnvironmentMutation = `
mutation($id: String!) {
	environmentDelete(id: $id)
}
`

// environmentsResponse represents the response for listing environments.
type environmentsResponse struct {
	Project struct {
		Environments struct {
			Edges []struct {
				Node struct {
					ID               string `json:"id"`
					Name             string `json:"name"`
					UpdatedAt        string `json:"updatedAt"`
					ServiceInstances struct {
						Edges []struct {
							Node struct {
								ServiceID   string `json:"serviceId"`
								ServiceName string `json:"serviceName"`
							} `json:"node"`
						} `json:"edges"`
					} `json:"serviceInstances"`
				} `json:"node"`
			} `json:"edges"`
		} `json:"environments"`
	} `json:"project"`
}

// createEnvironmentResponse represents the response from environmentCreate.
type createEnvironmentResponse struct {
	EnvironmentCreate struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"environmentCreate"`
}

// deleteEnvironmentResponse represents the response from environmentDelete.
type deleteEnvironmentResponse struct {
	EnvironmentDelete bool `json:"environmentDelete"`
}

// ListEnvironments retrieves all environments for a project.
func (c *Client) ListEnvironments(projectID string) ([]types.Environment, error) {
	data, err := c.execute(listEnvironmentsQuery, map[string]any{"id": projectID})
	if err != nil {
		return nil, err
	}

	var resp environmentsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	environments := make([]types.Environment, 0, len(resp.Project.Environments.Edges))
	for _, edge := range resp.Project.Environments.Edges {
		// Extract service names from service instances
		services := make([]types.Service, 0, len(edge.Node.ServiceInstances.Edges))
		for _, si := range edge.Node.ServiceInstances.Edges {
			services = append(services, types.Service{
				ID:   si.Node.ServiceID,
				Name: si.Node.ServiceName,
			})
		}

		env := types.Environment{
			ID:           edge.Node.ID,
			Name:         edge.Node.Name,
			ServiceCount: len(services),
			Services:     services,
		}

		// Parse updatedAt timestamp
		if edge.Node.UpdatedAt != "" {
			if t, err := time.Parse(time.RFC3339, edge.Node.UpdatedAt); err == nil {
				env.UpdatedAt = t
			}
		}

		environments = append(environments, env)
	}

	return environments, nil
}

// CreateEnvironment creates a new environment in a project.
func (c *Client) CreateEnvironment(projectID, name string) (types.Environment, error) {
	data, err := c.execute(createEnvironmentMutation, map[string]any{
		"projectId": projectID,
		"name":      name,
	})
	if err != nil {
		return types.Environment{}, err
	}

	var resp createEnvironmentResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return types.Environment{}, fmt.Errorf("failed to parse response: %w", err)
	}

	return types.Environment{
		ID:   resp.EnvironmentCreate.ID,
		Name: resp.EnvironmentCreate.Name,
	}, nil
}

// DeleteEnvironment deletes an environment by ID.
func (c *Client) DeleteEnvironment(id string) error {
	data, err := c.execute(deleteEnvironmentMutation, map[string]any{"id": id})
	if err != nil {
		return err
	}

	var resp deleteEnvironmentResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if !resp.EnvironmentDelete {
		return fmt.Errorf("failed to delete environment")
	}

	return nil
}
