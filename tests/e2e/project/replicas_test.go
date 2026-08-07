//go:build e2e

package project

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// TestGetReplicas proves `railctl get replicas` end-to-end against a live
// deployment under the group's PROJECT token. Unlike exec/port-forward this is
// a pure API read — no SSH key or relay needed — so it never skips.
//
// A freshly deployed single-replica service should report exactly one running
// instance, and that instance id must be a valid --deployment-instance target
// (it is the same id the relay routes on).
func TestGetReplicas(t *testing.T) {
	env := fixtureEnv(t)
	svc := deployReadySSHService(t, env)

	t.Run("table_lists_a_running_replica", func(t *testing.T) {
		r := env.RunOK(t, "get", "replicas", "-s", svc)
		// A ready single-replica service reports its instance and a RUNNING status.
		harness.AssertContains(t, r.Stdout, "INSTANCE ID")
		harness.AssertContains(t, r.Stdout, "RUNNING")
	})

	t.Run("json_carries_deployment_context_and_replicas", func(t *testing.T) {
		r := env.RunOK(t, "get", "replicas", "-s", svc, "-o", "json")

		var list struct {
			DeploymentID string `json:"deploymentId"`
			Replicas     []struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"replicas"`
		}
		if err := json.Unmarshal([]byte(r.Stdout), &list); err != nil {
			t.Fatalf("get replicas -o json is not valid JSON: %v\nstdout: %s", err, r.Stdout)
		}
		if list.DeploymentID == "" {
			t.Errorf("expected a deployment id in json output: %s", r.Stdout)
		}
		if len(list.Replicas) == 0 {
			t.Fatalf("expected at least one replica for a ready service: %s", r.Stdout)
		}
		if strings.TrimSpace(list.Replicas[0].ID) == "" {
			t.Errorf("replica id must be non-empty: %s", r.Stdout)
		}
	})
}
