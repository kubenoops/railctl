package api

import (
	"encoding/json"
	"fmt"
)

// GraphQL query to get variables for a service deployment
const variablesForServiceDeploymentQuery = `
query VariablesForServiceDeployment(
	$projectId: String!
	$environmentId: String!
	$serviceId: String!
) {
	variablesForServiceDeployment(
		projectId: $projectId
		environmentId: $environmentId
		serviceId: $serviceId
	)
}
`

// rawVariablesQuery returns variables unrendered (${{...}} kept intact), so the
// diff compares config templates against the stored template, not resolved values.
const rawVariablesQuery = `
query RawVariables(
	$projectId: String!
	$environmentId: String!
	$serviceId: String!
) {
	variables(
		projectId: $projectId
		environmentId: $environmentId
		serviceId: $serviceId
		unrendered: true
	)
}
`

// GraphQL mutation to upsert variables (set/update multiple variables)
const variableCollectionUpsertMutation = `
mutation VariableCollectionUpsert(
	$projectId: String!
	$serviceId: String!
	$environmentId: String!
	$variables: EnvironmentVariables!
	$skipDeploys: Boolean
) {
	variableCollectionUpsert(
		input: {
			projectId: $projectId
			environmentId: $environmentId
			serviceId: $serviceId
			variables: $variables
			skipDeploys: $skipDeploys
		}
	)
}
`

// GraphQL mutation to delete a single variable
const variableDeleteMutation = `
mutation VariableDelete(
	$projectId: String!
	$environmentId: String!
	$name: String!
	$serviceId: String
) {
	variableDelete(
		input: {
			projectId: $projectId
			environmentId: $environmentId
			name: $name
			serviceId: $serviceId
		}
	)
}
`

// GraphQL query to get variables with sealed status from environment
const variablesWithSealedQuery = `
query VariablesWithSealed($environmentId: String!) {
	environment(id: $environmentId) {
		variables {
			edges {
				node {
					name
					isSealed
					serviceId
				}
			}
		}
	}
}
`

// Response structure for variablesForServiceDeployment query
type variablesForServiceDeploymentResponse struct {
	VariablesForServiceDeployment map[string]string `json:"variablesForServiceDeployment"`
}

// Response structure for variableCollectionUpsert mutation
type variableCollectionUpsertResponse struct {
	VariableCollectionUpsert bool `json:"variableCollectionUpsert"`
}

// Response structure for variableDelete mutation
type variableDeleteResponse struct {
	VariableDelete bool `json:"variableDelete"`
}

// Response structure for variablesWithSealed query
type variablesWithSealedResponse struct {
	Environment struct {
		Variables struct {
			Edges []struct {
				Node struct {
					Name      string `json:"name"`
					IsSealed  bool   `json:"isSealed"`
					ServiceID string `json:"serviceId"`
				} `json:"node"`
			} `json:"edges"`
		} `json:"variables"`
	} `json:"environment"`
}

// SealedVarInfo contains information about whether a variable is sealed
type SealedVarInfo struct {
	Name     string
	IsSealed bool
}

// GetSealedVariables returns a map of variable names to their sealed status for a service
func (c *Client) GetSealedVariables(environmentID, serviceID string) (map[string]bool, error) {
	data, err := c.execute(variablesWithSealedQuery, map[string]any{
		"environmentId": environmentID,
	})
	if err != nil {
		return nil, err
	}

	var resp variablesWithSealedResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	result := make(map[string]bool)
	for _, edge := range resp.Environment.Variables.Edges {
		// Only include variables for this service (or service-less variables)
		if edge.Node.ServiceID == serviceID || edge.Node.ServiceID == "" {
			result[edge.Node.Name] = edge.Node.IsSealed
		}
	}

	return result, nil
}

// GetVariables retrieves all environment variables for a service
func (c *Client) GetVariables(projectID, environmentID, serviceID string) (map[string]string, error) {
	data, err := c.execute(variablesForServiceDeploymentQuery, map[string]any{
		"projectId":     projectID,
		"environmentId": environmentID,
		"serviceId":     serviceID,
	})
	if err != nil {
		return nil, err
	}

	var resp variablesForServiceDeploymentResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	// Return empty map if nil (no variables)
	if resp.VariablesForServiceDeployment == nil {
		return make(map[string]string), nil
	}

	return resp.VariablesForServiceDeployment, nil
}

// GetRawVariables retrieves a service's variables unrendered (${{...}} kept intact).
func (c *Client) GetRawVariables(projectID, environmentID, serviceID string) (map[string]string, error) {
	data, err := c.execute(rawVariablesQuery, map[string]any{
		"projectId":     projectID,
		"environmentId": environmentID,
		"serviceId":     serviceID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to execute raw variables query: %w", err)
	}

	var resp struct {
		Variables map[string]string `json:"variables"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal raw variables: %w", err)
	}
	if resp.Variables == nil {
		return make(map[string]string), nil
	}
	return resp.Variables, nil
}

// SetVariables sets or updates multiple environment variables for a service
// If skipDeploys is true, the service will not be redeployed after setting variables
func (c *Client) SetVariables(projectID, environmentID, serviceID string, variables map[string]string, skipDeploys bool) error {
	input := map[string]any{
		"projectId":     projectID,
		"environmentId": environmentID,
		"serviceId":     serviceID,
		"variables":     variables,
	}

	// Only include skipDeploys if true
	if skipDeploys {
		input["skipDeploys"] = true
	}

	data, err := c.execute(variableCollectionUpsertMutation, input)
	if err != nil {
		return err
	}

	var resp variableCollectionUpsertResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return err
	}

	if !resp.VariableCollectionUpsert {
		return fmt.Errorf("failed to set variables")
	}

	return nil
}

// DeleteVariable deletes a single environment variable from a service
func (c *Client) DeleteVariable(projectID, environmentID, serviceID, name string) error {
	data, err := c.execute(variableDeleteMutation, map[string]any{
		"projectId":     projectID,
		"environmentId": environmentID,
		"name":          name,
		"serviceId":     serviceID,
	})
	if err != nil {
		return err
	}

	var resp variableDeleteResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return err
	}

	if !resp.VariableDelete {
		return fmt.Errorf("failed to delete variable '%s'", name)
	}

	return nil
}
