package api

import (
	"encoding/json"
	"fmt"

	"github.com/kubenoops/railctl/internal/types"
)

// createProjectMutation creates a new project.
const createProjectMutation = `
mutation($name: String, $workspaceId: String) {
	projectCreate(input: { name: $name, workspaceId: $workspaceId }) {
		id
		name
		environments {
			edges {
				node {
					id
					name
				}
			}
		}
	}
}
`

// deleteProjectMutation deletes a project by ID.
const deleteProjectMutation = `
mutation($id: String!) {
	projectDelete(id: $id)
}
`

// createProjectResponse represents the response from projectCreate mutation.
type createProjectResponse struct {
	ProjectCreate struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		Environments struct {
			Edges []struct {
				Node struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"node"`
			} `json:"edges"`
		} `json:"environments"`
	} `json:"projectCreate"`
}

// deleteProjectResponse represents the response from projectDelete mutation.
type deleteProjectResponse struct {
	ProjectDelete bool `json:"projectDelete"`
}

// CreateProject creates a new project with the given name.
// Returns the created project with its default environment.
func (c *Client) CreateProject(name string) (types.Project, error) {
	workspaceID, err := c.GetWorkspaceID()
	if err != nil {
		return types.Project{}, err
	}

	if workspaceID == "" {
		return types.Project{}, fmt.Errorf("workspace required to create a project: use -w <name> or set RAILCTL_WORKSPACE=<name>, or use a personal API token if your token does not have workspace access")
	}

	data, err := c.execute(createProjectMutation, map[string]any{
		"name":        name,
		"workspaceId": workspaceID,
	})
	if err != nil {
		return types.Project{}, err
	}

	var resp createProjectResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return types.Project{}, fmt.Errorf("failed to parse response: %w", err)
	}

	project := types.Project{
		ID:           resp.ProjectCreate.ID,
		Name:         resp.ProjectCreate.Name,
		Environments: make([]types.Environment, 0),
		Services:     make([]types.Service, 0),
	}

	for _, edge := range resp.ProjectCreate.Environments.Edges {
		project.Environments = append(project.Environments, types.Environment{
			ID:   edge.Node.ID,
			Name: edge.Node.Name,
		})
	}

	return project, nil
}

// DeleteProject deletes a project by ID.
// Returns an error if the deletion fails.
func (c *Client) DeleteProject(id string) error {
	data, err := c.execute(deleteProjectMutation, map[string]any{"id": id})
	if err != nil {
		return err
	}

	var resp deleteProjectResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if !resp.ProjectDelete {
		return fmt.Errorf("failed to delete project")
	}

	return nil
}
