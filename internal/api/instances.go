package api

import (
	"encoding/json"
	"fmt"
)

// serviceInstanceQuery resolves a service's connectable instance id in an
// environment. The returned id is the SSH username the relay uses to route an
// exec/forward session into the right container.
const serviceInstanceQuery = `
query ServiceInstance($environmentId: String!, $serviceId: String!) {
	serviceInstance(environmentId: $environmentId, serviceId: $serviceId) {
		id
	}
}
`

// serviceInstanceResponse is the GraphQL response for serviceInstanceQuery.
type serviceInstanceResponse struct {
	ServiceInstance struct {
		ID string `json:"id"`
	} `json:"serviceInstance"`
}

// GetServiceInstanceID resolves (environmentID, serviceID) → the connectable
// service instance id used as the SSH username for `railctl exec`. Returns a
// friendly error if no active instance exists.
func (c *Client) GetServiceInstanceID(environmentID, serviceID string) (string, error) {
	data, err := c.execute(serviceInstanceQuery, map[string]any{
		"environmentId": environmentID,
		"serviceId":     serviceID,
	})
	if err != nil {
		return "", err
	}

	var resp serviceInstanceResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}

	if resp.ServiceInstance.ID == "" {
		return "", fmt.Errorf("no active service instance found for the service in this environment (is it deployed and running?)")
	}

	return resp.ServiceInstance.ID, nil
}
