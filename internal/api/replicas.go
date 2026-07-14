package api

import (
	"encoding/json"
	"fmt"
)

// replicasQuery lists the running replica (deployment-instance) ids of a
// service in an environment. Each instance id is a valid SSH target for the
// relay — the value `exec`/`port-forward` accept via --deployment-instance to
// reach one specific replica instead of letting the relay pick.
//
// Instances live under the service's latestDeployment; the element type is
// DeploymentDeploymentInstance, which exposes exactly { id, status }. The
// parent deployment's id and status are fetched too, for the command's
// context header (they cost nothing extra on this query).
const replicasQuery = `
query Replicas($environmentId: String!, $serviceId: String!) {
	serviceInstance(environmentId: $environmentId, serviceId: $serviceId) {
		latestDeployment {
			id
			status
			instances {
				id
				status
			}
		}
	}
}
`

// Replica is one running instance (replica) of a service's deployment. The ID
// is the deployment-instance id used as the SSH username to target this exact
// replica; Status is a DeploymentInstanceStatus (RUNNING, CRASHED, EXITED, …).
type Replica struct {
	ID     string `json:"id" yaml:"id"`
	Status string `json:"status" yaml:"status"`
}

// ReplicaList is a service's running replicas plus the parent deployment
// context they belong to. DeploymentID/DeploymentStatus are empty when the
// service has no active deployment (Replicas is then empty too).
type ReplicaList struct {
	DeploymentID     string    `json:"deploymentId,omitempty" yaml:"deploymentId,omitempty"`
	DeploymentStatus string    `json:"deploymentStatus,omitempty" yaml:"deploymentStatus,omitempty"`
	Replicas         []Replica `json:"replicas" yaml:"replicas"`
}

// replicasResponse is the GraphQL response for replicasQuery. latestDeployment
// is a pointer because a service with no deployment returns null for it.
type replicasResponse struct {
	ServiceInstance struct {
		LatestDeployment *struct {
			ID        string `json:"id"`
			Status    string `json:"status"`
			Instances []struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"instances"`
		} `json:"latestDeployment"`
	} `json:"serviceInstance"`
}

// ListReplicas returns the running replicas (deployment instances) of a service
// in an environment, plus their parent deployment context. A service with no
// active deployment (idle, scaled to zero, never deployed) yields an empty
// Replicas slice and no error — "no replicas" is a valid state the caller
// reports, not a failure.
func (c *Client) ListReplicas(environmentID, serviceID string) (ReplicaList, error) {
	data, err := c.execute(replicasQuery, map[string]any{
		"environmentId": environmentID,
		"serviceId":     serviceID,
	})
	if err != nil {
		return ReplicaList{}, fmt.Errorf("failed to execute replicas query: %w", err)
	}

	var resp replicasResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return ReplicaList{}, fmt.Errorf("failed to unmarshal replicas response: %w", err)
	}

	dep := resp.ServiceInstance.LatestDeployment
	if dep == nil {
		return ReplicaList{Replicas: []Replica{}}, nil
	}

	replicas := make([]Replica, 0, len(dep.Instances))
	for _, inst := range dep.Instances {
		replicas = append(replicas, Replica{
			ID:     inst.ID,
			Status: inst.Status,
		})
	}
	return ReplicaList{
		DeploymentID:     dep.ID,
		DeploymentStatus: dep.Status,
		Replicas:         replicas,
	}, nil
}
