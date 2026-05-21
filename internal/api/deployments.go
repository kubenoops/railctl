package api

import (
	"encoding/json"
	"time"
)

// listDeploymentsQuery is the GraphQL query for listing deployments.
const listDeploymentsQuery = `
query($input: DeploymentListInput!, $first: Int) {
	deployments(input: $input, first: $first) {
		edges {
			node {
				id
				status
				createdAt
				updatedAt
				creator {
					name
				}
				canRedeploy
				canRollback
				meta
			}
		}
	}
}
`

// removeDeploymentMutation is the GraphQL mutation for removing a deployment.
const removeDeploymentMutation = `
mutation($id: String!) {
	deploymentRemove(id: $id)
}
`

// deploymentLogsQuery is the GraphQL query for fetching deployment logs.
const deploymentLogsQuery = `
query DeploymentLogs($deploymentId: String!, $limit: Int) {
	deploymentLogs(deploymentId: $deploymentId, limit: $limit) {
		timestamp
		message
	}
}
`

// Deployment represents a Railway deployment.
type Deployment struct {
	ID          string    `json:"id" yaml:"id"`
	Status      string    `json:"status" yaml:"status"`
	CreatedAt   time.Time `json:"createdAt" yaml:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt" yaml:"updatedAt"`
	CreatorName string    `json:"creatorName,omitempty" yaml:"creatorName,omitempty"`
	Image       string    `json:"image,omitempty" yaml:"image,omitempty"`
	CanRedeploy bool      `json:"-" yaml:"-"` // internal use only
	CanRollback bool      `json:"-" yaml:"-"` // internal use only
}

// LogEntry represents a single log line from a deployment.
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
}

// deploymentsResponse represents the GraphQL response for the deployments query.
type deploymentsResponse struct {
	Deployments struct {
		Edges []struct {
			Node struct {
				ID        string    `json:"id"`
				Status    string    `json:"status"`
				CreatedAt time.Time `json:"createdAt"`
				UpdatedAt time.Time `json:"updatedAt"`
				Creator   struct {
					Name string `json:"name"`
				} `json:"creator"`
				CanRedeploy bool `json:"canRedeploy"`
				CanRollback bool `json:"canRollback"`
				Meta        struct {
					Image string `json:"image"`
				} `json:"meta"`
			} `json:"node"`
		} `json:"edges"`
	} `json:"deployments"`
}

// deploymentLogsResponse represents the GraphQL response for deployment logs.
type deploymentLogsResponse struct {
	DeploymentLogs []struct {
		Timestamp time.Time `json:"timestamp"`
		Message   string    `json:"message"`
	} `json:"deploymentLogs"`
}

// ListDeployments retrieves deployments for a service in a given environment.
func (c *Client) ListDeployments(projectID, environmentID, serviceID string, limit int) ([]Deployment, error) {
	input := map[string]any{
		"projectId":     projectID,
		"environmentId": environmentID,
		"serviceId":     serviceID,
	}

	vars := map[string]any{
		"input": input,
	}
	if limit > 0 {
		vars["first"] = limit
	}

	data, err := c.execute(listDeploymentsQuery, vars)
	if err != nil {
		return nil, err
	}

	var resp deploymentsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	deployments := make([]Deployment, 0, len(resp.Deployments.Edges))
	for _, edge := range resp.Deployments.Edges {
		d := Deployment{
			ID:          edge.Node.ID,
			Status:      edge.Node.Status,
			CreatedAt:   edge.Node.CreatedAt,
			UpdatedAt:   edge.Node.UpdatedAt,
			CreatorName: edge.Node.Creator.Name,
			Image:       edge.Node.Meta.Image,
			CanRedeploy: edge.Node.CanRedeploy,
			CanRollback: edge.Node.CanRollback,
		}
		deployments = append(deployments, d)
	}

	return deployments, nil
}

// RemoveDeployment removes a deployment by ID.
func (c *Client) RemoveDeployment(deploymentID string) error {
	_, err := c.execute(removeDeploymentMutation, map[string]any{
		"id": deploymentID,
	})
	return err
}

// GetDeploymentLogs fetches deployment logs for a specific deployment.
func (c *Client) GetDeploymentLogs(deploymentID string, limit int) ([]LogEntry, error) {
	vars := map[string]any{
		"deploymentId": deploymentID,
	}
	if limit > 0 {
		vars["limit"] = limit
	}

	data, err := c.execute(deploymentLogsQuery, vars)
	if err != nil {
		return nil, err
	}

	var resp deploymentLogsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	logs := make([]LogEntry, 0, len(resp.DeploymentLogs))
	for _, log := range resp.DeploymentLogs {
		logs = append(logs, LogEntry{
			Timestamp: log.Timestamp,
			Message:   log.Message,
		})
	}

	return logs, nil
}
