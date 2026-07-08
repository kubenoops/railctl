package api

import (
	"encoding/json"
	"fmt"
)

// sharedVariablesQuery fetches environment-level ("shared", serviceless)
// variables. Railway's `variables` query returns the environment's shared
// variables when serviceId is omitted — unlike variablesForServiceDeployment,
// which always requires a service.
const sharedVariablesQuery = `
query SharedVariables($projectId: String!, $environmentId: String!) {
	variables(projectId: $projectId, environmentId: $environmentId)
}
`

// sharedVariableCollectionUpsertMutation sets environment-level shared
// variables: variableCollectionUpsert with serviceId omitted targets the
// environment itself rather than a service.
const sharedVariableCollectionUpsertMutation = `
mutation SharedVariableCollectionUpsert(
	$projectId: String!
	$environmentId: String!
	$variables: EnvironmentVariables!
) {
	variableCollectionUpsert(
		input: {
			projectId: $projectId
			environmentId: $environmentId
			variables: $variables
		}
	)
}
`

// GetSharedVariables retrieves the environment-level (shared, serviceless)
// variables of an environment.
func (c *Client) GetSharedVariables(projectID, environmentID string) (map[string]string, error) {
	data, err := c.execute(sharedVariablesQuery, map[string]any{
		"projectId":     projectID,
		"environmentId": environmentID,
	})
	if err != nil {
		return nil, err
	}

	var resp struct {
		Variables map[string]string `json:"variables"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	// Return empty map if nil (no shared variables)
	if resp.Variables == nil {
		return make(map[string]string), nil
	}

	return resp.Variables, nil
}

// SetSharedVariables sets or updates environment-level (shared, serviceless)
// variables on an environment.
func (c *Client) SetSharedVariables(projectID, environmentID string, variables map[string]string) error {
	data, err := c.execute(sharedVariableCollectionUpsertMutation, map[string]any{
		"projectId":     projectID,
		"environmentId": environmentID,
		"variables":     variables,
	})
	if err != nil {
		return err
	}

	var resp variableCollectionUpsertResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return err
	}

	if !resp.VariableCollectionUpsert {
		return fmt.Errorf("failed to set shared variables")
	}

	return nil
}
