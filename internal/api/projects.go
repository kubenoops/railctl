package api

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/kubenoops/railctl/internal/types"
)

// listProjectsQuery is the GraphQL query for listing projects.
const listProjectsQuery = `
query($workspaceId: String) {
	projects(workspaceId: $workspaceId) {
		edges {
			node {
				id
				name
				updatedAt
				environments {
					edges {
						node {
							id
							name
						}
					}
				}
				services {
					edges {
						node {
							id
							name
						}
					}
				}
			}
		}
	}
}
`

// getProjectQuery is the GraphQL query for getting a single project.
const getProjectQuery = `
query($id: String!) {
	project(id: $id) {
		id
		name
		updatedAt
		environments {
			edges {
				node {
					id
					name
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
		services {
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

// projectsResponse represents the GraphQL response for projects query.
type projectsResponse struct {
	Projects struct {
		Edges []struct {
			Node projectNode `json:"node"`
		} `json:"edges"`
	} `json:"projects"`
}

// projectResponse represents the GraphQL response for single project query.
type projectResponse struct {
	Project projectNode `json:"project"`
}

// projectNode represents the raw project data from GraphQL.
type projectNode struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	UpdatedAt    time.Time `json:"updatedAt"`
	Environments struct {
		Edges []struct {
			Node struct {
				ID               string `json:"id"`
				Name             string `json:"name"`
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
	Services struct {
		Edges []struct {
			Node struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"node"`
		} `json:"edges"`
	} `json:"services"`
}

// toProject converts a projectNode to a types.Project.
func (n projectNode) toProject() types.Project {
	p := types.Project{
		ID:           n.ID,
		Name:         n.Name,
		UpdatedAt:    n.UpdatedAt,
		Environments: make([]types.Environment, 0, len(n.Environments.Edges)),
		Services:     make([]types.Service, 0, len(n.Services.Edges)),
	}

	for _, edge := range n.Environments.Edges {
		env := types.Environment{
			ID:   edge.Node.ID,
			Name: edge.Node.Name,
		}
		// Extract services for this environment
		for _, si := range edge.Node.ServiceInstances.Edges {
			env.Services = append(env.Services, types.Service{
				ID:   si.Node.ServiceID,
				Name: si.Node.ServiceName,
			})
		}
		env.ServiceCount = len(env.Services)
		p.Environments = append(p.Environments, env)
	}

	for _, edge := range n.Services.Edges {
		p.Services = append(p.Services, types.Service{
			ID:   edge.Node.ID,
			Name: edge.Node.Name,
		})
	}

	return p
}

// ListProjects retrieves all projects for the resolved workspace.
// Project-scoped tokens cannot list projects and return an error.
func (c *Client) ListProjects() ([]types.Project, error) {
	isProjectToken, err := c.IsProjectToken()
	if err != nil {
		return nil, err
	}
	if isProjectToken {
		return nil, fmt.Errorf("project tokens cannot list projects — the token is scoped to a single project")
	}

	workspaceID, err := c.GetWorkspaceID()
	if err != nil {
		return nil, err
	}

	vars := map[string]any{}
	if workspaceID != "" {
		vars["workspaceId"] = workspaceID
	}

	data, err := c.execute(listProjectsQuery, vars)
	if err != nil {
		return nil, err
	}

	var resp projectsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	projects := make([]types.Project, 0, len(resp.Projects.Edges))
	for _, edge := range resp.Projects.Edges {
		projects = append(projects, edge.Node.toProject())
	}
	return projects, nil
}

// GetProject retrieves a single project by ID.
func (c *Client) GetProject(id string) (types.Project, error) {
	data, err := c.execute(getProjectQuery, map[string]any{"id": id})
	if err != nil {
		return types.Project{}, err
	}

	var resp projectResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return types.Project{}, err
	}

	return resp.Project.toProject(), nil
}
