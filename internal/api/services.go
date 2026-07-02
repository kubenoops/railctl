package api

import (
	"encoding/json"
	"time"

	"github.com/kubenoops/railctl/internal/types"
)

// listServicesQuery is the GraphQL query for listing services in a project's environment.
const listServicesQuery = `
query($projectId: String!) {
	project(id: $projectId) {
		services {
			edges {
				node {
					id
					name
					icon
					updatedAt
					serviceInstances {
						edges {
							node {
								id
								environmentId
								startCommand
								restartPolicyType
								restartPolicyMaxRetries
								numReplicas
								healthcheckPath
								healthcheckTimeout
								source {
									image
									repo
								}
								latestDeployment {
									id
									status
									deploymentStopped
								}
								activeDeployments {
									id
									status
								}
							}
						}
					}
				}
			}
		}
	}
}
`

// getServiceQuery is the GraphQL query for getting a single service.
const getServiceQuery = `
query($id: String!) {
	service(id: $id) {
		id
		name
		icon
		updatedAt
		serviceInstances {
			edges {
				node {
					id
					environmentId
					startCommand
					restartPolicyType
					restartPolicyMaxRetries
					numReplicas
					healthcheckPath
					healthcheckTimeout
					source {
						image
						repo
					}
					latestDeployment {
						id
						status
						createdAt
						meta
						deploymentStopped
					}
					activeDeployments {
						id
						status
					}
				}
			}
		}
	}
}
`

// createServiceMutation is the GraphQL mutation for creating a service.
const createServiceMutation = `
mutation($input: ServiceCreateInput!) {
	serviceCreate(input: $input) {
		id
		name
	}
}
`

// deleteServiceMutation is the GraphQL mutation for deleting a service.
const deleteServiceMutation = `
mutation($id: String!) {
	serviceDelete(id: $id)
}
`

// deleteServiceInstanceMutation is the GraphQL mutation for deleting a service instance in a specific environment.
const deleteServiceInstanceMutation = `
mutation($id: String!, $environmentId: String!) {
	serviceDelete(id: $id, environmentId: $environmentId)
}
`

// updateServiceInstanceMutation is the GraphQL mutation for updating a service instance.
const updateServiceInstanceMutation = `
mutation($serviceId: String!, $environmentId: String!, $input: ServiceInstanceUpdateInput!) {
	serviceInstanceUpdate(serviceId: $serviceId, environmentId: $environmentId, input: $input)
}
`

// deploymentRedeployMutation is the GraphQL mutation for triggering a redeploy.
const deploymentRedeployMutation = `
mutation($id: String!) {
	deploymentRedeploy(id: $id) {
		id
	}
}
`

// serviceInstanceDeployMutation is the GraphQL mutation for deploying a service instance.
const serviceInstanceDeployMutation = `
mutation($serviceId: String!, $environmentId: String!) {
	serviceInstanceDeployV2(serviceId: $serviceId, environmentId: $environmentId)
}
`

// buildLogsQuery is the GraphQL query for fetching build logs.
const buildLogsQuery = `
query($deploymentId: String!, $limit: Int) {
	buildLogs(deploymentId: $deploymentId, limit: $limit) {
		message
	}
}
`

// servicesResponse represents the GraphQL response for services query.
type servicesResponse struct {
	Project struct {
		Services struct {
			Edges []struct {
				Node serviceNode `json:"node"`
			} `json:"edges"`
		} `json:"services"`
	} `json:"project"`
}

// serviceResponse represents the GraphQL response for single service query.
type serviceResponse struct {
	Service serviceNode `json:"service"`
}

// serviceCreateResponse represents the GraphQL response for service creation.
type serviceCreateResponse struct {
	ServiceCreate struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"serviceCreate"`
}

// buildLogsResponse represents the GraphQL response for build logs.
type buildLogsResponse struct {
	BuildLogs []struct {
		Message string `json:"message"`
	} `json:"buildLogs"`
}

// serviceInstanceSource is the image/repo source of a service instance.
type serviceInstanceSource struct {
	Image string `json:"image"`
	Repo  string `json:"repo"`
}

// serviceInstanceDeployment is a deployment referenced by a service instance.
type serviceInstanceDeployment struct {
	ID                string    `json:"id"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"createdAt"`
	Meta              any       `json:"meta"`
	DeploymentStopped bool      `json:"deploymentStopped"`
}

// serviceInstanceNode is one service instance (per environment) in a service response.
type serviceInstanceNode struct {
	ID                      string                      `json:"id"`
	EnvironmentID           string                      `json:"environmentId"`
	StartCommand            string                      `json:"startCommand"`
	RestartPolicyType       string                      `json:"restartPolicyType"`
	RestartPolicyMaxRetries int                         `json:"restartPolicyMaxRetries"`
	NumReplicas             int                         `json:"numReplicas"`
	HealthcheckPath         string                      `json:"healthcheckPath"`
	HealthcheckTimeout      int                         `json:"healthcheckTimeout"`
	Source                  serviceInstanceSource       `json:"source"`
	LatestDeployment        *serviceInstanceDeployment  `json:"latestDeployment"`
	ActiveDeployments       []serviceInstanceDeployment `json:"activeDeployments"`
}

// serviceInstanceEdge wraps a service instance node in the GraphQL edges list.
type serviceInstanceEdge struct {
	Node serviceInstanceNode `json:"node"`
}

// serviceInstances is the GraphQL edges collection of service instances.
type serviceInstances struct {
	Edges []serviceInstanceEdge `json:"edges"`
}

// serviceNode represents the raw service data from GraphQL.
type serviceNode struct {
	ID               string           `json:"id"`
	Name             string           `json:"name"`
	Icon             string           `json:"icon"`
	UpdatedAt        time.Time        `json:"updatedAt"`
	ServiceInstances serviceInstances `json:"serviceInstances"`
}

// toServiceDetail converts a serviceNode to a types.ServiceDetail.
func (n serviceNode) toServiceDetail(envID string) types.ServiceDetail {
	sd := types.ServiceDetail{
		ID:        n.ID,
		Name:      n.Name,
		Icon:      n.Icon,
		UpdatedAt: n.UpdatedAt,
	}

	// Find the service instance for the given environment
	for _, edge := range n.ServiceInstances.Edges {
		if envID == "" || edge.Node.EnvironmentID == envID {
			sd.InstanceID = edge.Node.ID
			sd.StartCommand = edge.Node.StartCommand
			sd.RestartPolicy = edge.Node.RestartPolicyType
			sd.MaxRetries = edge.Node.RestartPolicyMaxRetries
			sd.Replicas = edge.Node.NumReplicas
			sd.HealthcheckPath = edge.Node.HealthcheckPath
			sd.HealthcheckTimeout = edge.Node.HealthcheckTimeout
			if edge.Node.Source.Image != "" {
				sd.Source = edge.Node.Source.Image
				sd.SourceType = "image"
			} else if edge.Node.Source.Repo != "" {
				sd.Source = edge.Node.Source.Repo
				sd.SourceType = "repo"
			}
			// Extract deployment status
			if edge.Node.LatestDeployment != nil {
				sd.Status = edge.Node.LatestDeployment.Status
				sd.DeploymentID = edge.Node.LatestDeployment.ID
				sd.DeployedAt = edge.Node.LatestDeployment.CreatedAt

				// Check if service is actually running:
				// - If deploymentStopped is true, service is STOPPED
				// - If there are no activeDeployments, service is OFFLINE
				if edge.Node.LatestDeployment.DeploymentStopped {
					sd.Status = "STOPPED"
				} else if len(edge.Node.ActiveDeployments) == 0 && sd.Status == "SUCCESS" {
					// Deployment succeeded but nothing is actually running
					sd.Status = "OFFLINE"
				}

				// Extract error from meta if present (try various keys)
				if meta, ok := edge.Node.LatestDeployment.Meta.(map[string]any); ok {
					// Try common error field names
					for _, key := range []string{"error", "message", "errorMessage", "reason"} {
						if errMsg, ok := meta[key].(string); ok && errMsg != "" {
							sd.DeploymentError = errMsg
							break
						}
					}
					// If no error found but status is FAILED, serialize meta for debugging
					if sd.DeploymentError == "" && sd.Status == "FAILED" {
						if data, err := json.Marshal(meta); err == nil && len(meta) > 0 {
							sd.DeploymentError = string(data)
						}
					}
				}
			}
			break
		}
	}

	return sd
}

// ListServices retrieves all services in a project's environment.
func (c *Client) ListServices(projectID, environmentID string) ([]types.ServiceDetail, error) {
	data, err := c.execute(listServicesQuery, map[string]any{
		"projectId": projectID,
	})
	if err != nil {
		return nil, err
	}

	var resp servicesResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	services := make([]types.ServiceDetail, 0, len(resp.Project.Services.Edges))
	for _, edge := range resp.Project.Services.Edges {
		// Check if service has an instance in this environment
		hasInstance := false
		for _, inst := range edge.Node.ServiceInstances.Edges {
			if inst.Node.EnvironmentID == environmentID {
				hasInstance = true
				break
			}
		}
		if hasInstance {
			services = append(services, edge.Node.toServiceDetail(environmentID))
		}
	}

	return services, nil
}

// GetService retrieves a single service by ID.
func (c *Client) GetService(id string) (types.ServiceDetail, error) {
	data, err := c.execute(getServiceQuery, map[string]any{"id": id})
	if err != nil {
		return types.ServiceDetail{}, err
	}

	var resp serviceResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return types.ServiceDetail{}, err
	}

	return resp.Service.toServiceDetail(""), nil
}

// CreateService creates a new service in a project.
func (c *Client) CreateService(projectID, environmentID, name, image string, creds *RegistryCredentials) (types.Service, error) {
	input := map[string]any{
		"projectId":     projectID,
		"environmentId": environmentID,
		"name":          name,
	}
	if image != "" {
		input["source"] = map[string]any{"image": image}
	}
	if creds != nil && creds.Username != "" && creds.Password != "" {
		input["registryCredentials"] = map[string]any{
			"username": creds.Username,
			"password": creds.Password,
		}
	}

	data, err := c.execute(createServiceMutation, map[string]any{"input": input})
	if err != nil {
		return types.Service{}, err
	}

	var resp serviceCreateResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return types.Service{}, err
	}

	return types.Service{
		ID:   resp.ServiceCreate.ID,
		Name: resp.ServiceCreate.Name,
	}, nil
}

// DeleteService deletes a service.
func (c *Client) DeleteService(id string) error {
	_, err := c.execute(deleteServiceMutation, map[string]any{"id": id})
	return err
}

// DeleteServiceInstance deletes a service instance in a specific environment.
// This removes the service from only the specified environment, not the entire project.
func (c *Client) DeleteServiceInstance(serviceID, environmentID string) error {
	_, err := c.execute(deleteServiceInstanceMutation, map[string]any{
		"id":            serviceID,
		"environmentId": environmentID,
	})
	return err
}

// UpdateServiceInstance updates a service instance (e.g., change image).
func (c *Client) UpdateServiceInstance(serviceID, environmentID, image string, creds *RegistryCredentials) error {
	input := map[string]any{}

	// Only include source if image is provided
	if image != "" {
		input["source"] = map[string]any{"image": image}
	}

	// Include registry credentials if provided
	if creds != nil && creds.Username != "" && creds.Password != "" {
		input["registryCredentials"] = map[string]any{
			"username": creds.Username,
			"password": creds.Password,
		}
	}

	_, err := c.execute(updateServiceInstanceMutation, map[string]any{
		"serviceId":     serviceID,
		"environmentId": environmentID,
		"input":         input,
	})
	return err
}

// UpdateServiceInstanceConfig updates service instance deploy configuration.
// This includes startCommand, restartPolicy, maxRetries, numReplicas, healthcheckPath, and healthcheckTimeout.
// All parameters are optional (use nil to skip).
func (c *Client) UpdateServiceInstanceConfig(
	serviceID string,
	environmentID string,
	startCommand *string,
	restartPolicy *string,
	maxRetries *int,
	replicas *int,
	healthcheckPath *string,
	healthcheckTimeout *int,
) error {
	// Build input map with only non-nil values
	input := make(map[string]any)

	if startCommand != nil {
		input["startCommand"] = *startCommand
	}
	if restartPolicy != nil {
		input["restartPolicyType"] = *restartPolicy
	}
	if maxRetries != nil {
		input["restartPolicyMaxRetries"] = *maxRetries
	}
	if replicas != nil {
		input["numReplicas"] = *replicas
	}
	if healthcheckPath != nil {
		input["healthcheckPath"] = *healthcheckPath
	}
	if healthcheckTimeout != nil {
		input["healthcheckTimeout"] = *healthcheckTimeout
	}

	// If no parameters provided, nothing to update
	if len(input) == 0 {
		return nil
	}

	_, err := c.execute(updateServiceInstanceMutation, map[string]any{
		"serviceId":     serviceID,
		"environmentId": environmentID,
		"input":         input,
	})
	return err
}

// RedeployDeployment triggers a redeploy of a specific deployment.
func (c *Client) RedeployDeployment(deploymentID string) error {
	_, err := c.execute(deploymentRedeployMutation, map[string]any{
		"id": deploymentID,
	})
	return err
}

// DeployServiceInstance triggers a new deployment for a service instance.
// Returns the deployment ID of the newly created deployment.
func (c *Client) DeployServiceInstance(serviceID, environmentID string) (string, error) {
	data, err := c.execute(serviceInstanceDeployMutation, map[string]any{
		"serviceId":     serviceID,
		"environmentId": environmentID,
	})
	if err != nil {
		return "", err
	}

	// The mutation returns the deployment ID as a string
	var resp struct {
		ServiceInstanceDeployV2 string `json:"serviceInstanceDeployV2"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}

	return resp.ServiceInstanceDeployV2, nil
}

// GetBuildLogs fetches build logs for a deployment.
func (c *Client) GetBuildLogs(deploymentID string, limit int) ([]string, error) {
	data, err := c.execute(buildLogsQuery, map[string]any{
		"deploymentId": deploymentID,
		"limit":        limit,
	})
	if err != nil {
		return nil, err
	}

	var resp buildLogsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	messages := make([]string, 0, len(resp.BuildLogs))
	for _, log := range resp.BuildLogs {
		if log.Message != "" {
			messages = append(messages, log.Message)
		}
	}
	return messages, nil
}

// ExtractErrorFromLogs finds error messages in build logs.
func ExtractErrorFromLogs(logs []string) string {
	// Look for common error patterns
	for _, msg := range logs {
		// Skip decorative lines
		if msg == "" || msg == "=========================" {
			continue
		}
		// Skip header messages, look for actual error content
		if msg != "Container failed to start" && msg != "Build failed" {
			return msg
		}
	}
	return ""
}
