package api

import (
	"encoding/json"
	"fmt"
)

// ServiceDomain represents a Railway-generated service domain (*.up.railway.app).
type ServiceDomain struct {
	ID         string `json:"id"`
	Domain     string `json:"domain"`
	TargetPort *int   `json:"targetPort"`
}

// CustomDomain represents a user-configured custom domain (e.g. Cloudflare).
type CustomDomain struct {
	ID         string `json:"id"`
	Domain     string `json:"domain"`
	TargetPort *int   `json:"targetPort"`
}

// DomainList contains both Railway-generated and custom domains for a service.
type DomainList struct {
	ServiceDomains []ServiceDomain `json:"serviceDomains"`
	CustomDomains  []CustomDomain  `json:"customDomains"`
}

// ListDomains retrieves all domains (service + custom) for a service instance.
func (c *Client) ListDomains(projectID, environmentID, serviceID string) (DomainList, error) {
	query := `
		query Domains($environmentId: String!, $projectId: String!, $serviceId: String!) {
			domains(
				environmentId: $environmentId
				projectId: $projectId
				serviceId: $serviceId
			) {
				serviceDomains {
					id
					domain
					targetPort
				}
				customDomains {
					id
					domain
					targetPort
				}
			}
		}
	`

	variables := map[string]any{
		"environmentId": environmentID,
		"projectId":     projectID,
		"serviceId":     serviceID,
	}

	data, err := c.execute(query, variables)
	if err != nil {
		return DomainList{}, err
	}

	var result struct {
		Domains DomainList `json:"domains"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return DomainList{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result.Domains, nil
}

// CreateServiceDomain creates a Railway-generated domain (*.up.railway.app),
// routing to targetPort when > 0.
func (c *Client) CreateServiceDomain(serviceID, environmentID string, targetPort int) (ServiceDomain, error) {
	mutation := `
		mutation ServiceDomainCreate($input: ServiceDomainCreateInput!) {
			serviceDomainCreate(input: $input) {
				id
				domain
			}
		}
	`

	input := map[string]any{
		"environmentId": environmentID,
		"serviceId":     serviceID,
	}
	if targetPort > 0 {
		input["targetPort"] = targetPort
	}
	variables := map[string]any{"input": input}

	data, err := c.execute(mutation, variables)
	if err != nil {
		return ServiceDomain{}, err
	}

	var result struct {
		ServiceDomainCreate ServiceDomain `json:"serviceDomainCreate"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return ServiceDomain{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result.ServiceDomainCreate, nil
}

// UpdateServiceDomainPort sets a service domain's target port. All four fields are
// required by ServiceDomainUpdateInput; omitting any fails with "Problem processing request".
func (c *Client) UpdateServiceDomainPort(serviceDomainID, domain, environmentID, serviceID string, port int) error {
	mutation := `
		mutation ServiceDomainUpdate($input: ServiceDomainUpdateInput!) {
			serviceDomainUpdate(input: $input)
		}
	`

	variables := map[string]any{
		"input": map[string]any{
			"serviceDomainId": serviceDomainID,
			"domain":          domain,
			"environmentId":   environmentID,
			"serviceId":       serviceID,
			"targetPort":      port,
		},
	}

	_, err := c.execute(mutation, variables)
	return err
}

// UpdateCustomDomainPort updates the target port of a custom domain.
func (c *Client) UpdateCustomDomainPort(customDomainID, environmentID string, port int) error {
	mutation := `
		mutation CustomDomainUpdate($environmentId: String!, $id: String!, $targetPort: Int!) {
			customDomainUpdate(environmentId: $environmentId, id: $id, targetPort: $targetPort)
		}
	`

	variables := map[string]any{
		"environmentId": environmentID,
		"id":            customDomainID,
		"targetPort":    port,
	}

	_, err := c.execute(mutation, variables)
	return err
}

// DeleteServiceDomain deletes a Railway-generated service domain by ID.
func (c *Client) DeleteServiceDomain(id string) error {
	mutation := `
		mutation ServiceDomainDelete($id: String!) {
			serviceDomainDelete(id: $id)
		}
	`

	variables := map[string]any{
		"id": id,
	}

	_, err := c.execute(mutation, variables)
	return err
}

// DeleteCustomDomain deletes a custom domain by ID.
func (c *Client) DeleteCustomDomain(id string) error {
	mutation := `
		mutation CustomDomainDelete($id: String!) {
			customDomainDelete(id: $id)
		}
	`

	variables := map[string]any{
		"id": id,
	}

	_, err := c.execute(mutation, variables)
	return err
}
