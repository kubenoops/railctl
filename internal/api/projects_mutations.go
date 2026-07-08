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

	// A workspace-scoped token resolves to an empty ID (its workspace is implicit),
	// so send a null workspaceId and let the API infer it — exactly as ListProjects
	// does. GetWorkspaceID already errors on an ambiguous multi-workspace account
	// token, so an empty ID here unambiguously means "infer from the token".
	vars := map[string]any{"name": name}
	if workspaceID != "" {
		vars["workspaceId"] = workspaceID
	}

	data, err := c.execute(createProjectMutation, vars)
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
