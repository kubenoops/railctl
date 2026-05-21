package api

import (
	"encoding/json"
	"fmt"
)

// TCPProxy represents a Railway TCP proxy that exposes a service port externally.
type TCPProxy struct {
	ID              string `json:"id"`
	Domain          string `json:"domain"`
	ProxyPort       int    `json:"proxyPort"`
	ApplicationPort int    `json:"applicationPort"`
}

// ListTCPProxies retrieves all TCP proxies for a service instance.
func (c *Client) ListTCPProxies(environmentID, serviceID string) ([]TCPProxy, error) {
	query := `
		query TCPProxies($environmentId: String!, $serviceId: String!) {
			tcpProxies(
				environmentId: $environmentId
				serviceId: $serviceId
			) {
				id
				domain
				proxyPort
				applicationPort
			}
		}
	`

	variables := map[string]any{
		"environmentId": environmentID,
		"serviceId":     serviceID,
	}

	data, err := c.execute(query, variables)
	if err != nil {
		return nil, err
	}

	var result struct {
		TCPProxies []TCPProxy `json:"tcpProxies"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result.TCPProxies, nil
}

// CreateTCPProxy creates a TCP proxy for a service on the given application port.
func (c *Client) CreateTCPProxy(applicationPort int, environmentID, serviceID string) (TCPProxy, error) {
	mutation := `
		mutation TCPProxyCreate($input: TCPProxyCreateInput!) {
			tcpProxyCreate(input: $input) {
				id
				domain
				proxyPort
				applicationPort
			}
		}
	`

	variables := map[string]any{
		"input": map[string]any{
			"applicationPort": applicationPort,
			"environmentId":   environmentID,
			"serviceId":       serviceID,
		},
	}

	data, err := c.execute(mutation, variables)
	if err != nil {
		return TCPProxy{}, err
	}

	var result struct {
		TCPProxyCreate TCPProxy `json:"tcpProxyCreate"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return TCPProxy{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result.TCPProxyCreate, nil
}

// DeleteTCPProxy deletes a TCP proxy by ID.
func (c *Client) DeleteTCPProxy(id string) error {
	mutation := `
		mutation TCPProxyDelete($id: String!) {
			tcpProxyDelete(id: $id)
		}
	`

	variables := map[string]any{
		"id": id,
	}

	_, err := c.execute(mutation, variables)
	return err
}
