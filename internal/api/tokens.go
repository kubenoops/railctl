package api

import (
	"encoding/json"
	"fmt"
)

// ProjectToken is a project + environment-scoped access token. DisplayToken is
// masked by Railway; the raw value is returned only once, at creation.
type ProjectToken struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	EnvironmentID string `json:"environmentId"`
	CreatedAt     string `json:"createdAt"`
	DisplayToken  string `json:"displayToken"`
}

// CreateProjectToken mints a token scoped to one project + environment and
// returns the raw token value. The value is shown only once by Railway and
// cannot be retrieved later.
func (c *Client) CreateProjectToken(projectID, environmentID, name string) (string, error) {
	mutation := `
		mutation ProjectTokenCreate($input: ProjectTokenCreateInput!) {
			projectTokenCreate(input: $input)
		}
	`
	variables := map[string]any{
		"input": map[string]any{
			"projectId":     projectID,
			"environmentId": environmentID,
			"name":          name,
		},
	}

	data, err := c.execute(mutation, variables)
	if err != nil {
		return "", err
	}

	var result struct {
		Token string `json:"projectTokenCreate"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return result.Token, nil
}

// ListProjectTokens returns the project tokens for a project, across all
// environments. Values are masked (DisplayToken); the raw value is never listed.
func (c *Client) ListProjectTokens(projectID string) ([]ProjectToken, error) {
	query := `
		query ProjectTokens($projectId: String!) {
			projectTokens(projectId: $projectId) {
				edges {
					node {
						id
						name
						environmentId
						createdAt
						displayToken
					}
				}
			}
		}
	`
	variables := map[string]any{"projectId": projectID}

	data, err := c.execute(query, variables)
	if err != nil {
		return nil, err
	}

	var result struct {
		ProjectTokens struct {
			Edges []struct {
				Node ProjectToken `json:"node"`
			} `json:"edges"`
		} `json:"projectTokens"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	tokens := make([]ProjectToken, 0, len(result.ProjectTokens.Edges))
	for _, e := range result.ProjectTokens.Edges {
		tokens = append(tokens, e.Node)
	}
	return tokens, nil
}

// DeleteProjectToken revokes a project token by ID.
func (c *Client) DeleteProjectToken(tokenID string) error {
	mutation := `
		mutation ProjectTokenDelete($id: String!) {
			projectTokenDelete(id: $id)
		}
	`
	variables := map[string]any{"id": tokenID}

	_, err := c.execute(mutation, variables)
	return err
}
