//go:build e2e

package project

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// TestDeployments exercises deployment list, logs, and error cases inside
// the shared fixture project under the minted project token (no -p/-e
// flags; -s kept). Creating the service triggers the first deployment.
//
//	go test -tags e2e -v -run TestDeployments ./tests/e2e/project/...
func TestDeployments(t *testing.T) {
	env := fixtureEnv(t)
	svc := createService(t, env)

	t.Run("get_table", func(t *testing.T) {
		env.RunOK(t, "get", "deployments", "-s", svc)
	})

	t.Run("get_json", func(t *testing.T) {
		r := env.RunOK(t, "get", "deployments", "-o", "json", "-s", svc)
		harness.AssertValidJSON(t, r.Stdout)
	})

	t.Run("get_yaml", func(t *testing.T) {
		r := env.RunOK(t, "get", "deployments", "-o", "yaml", "-s", svc)
		harness.AssertValidYAML(t, r.Stdout)
	})

	t.Run("get_wide", func(t *testing.T) {
		env.RunOK(t, "get", "deployments", "-o", "wide", "-s", svc)
	})

	t.Run("get_limit", func(t *testing.T) {
		env.RunOK(t, "get", "deployments", "--limit", "1", "-s", svc)
	})

	t.Run("logs_latest", func(t *testing.T) {
		// Logs may or may not have content — just verify the command runs
		env.Run("logs", svc, "-s", svc)
	})

	t.Run("logs_by_deployment_id", func(t *testing.T) {
		r := env.RunOK(t, "get", "deployments", "-o", "json", "-s", svc)
		var deps []map[string]interface{}
		if json.Unmarshal([]byte(r.Stdout), &deps) == nil && len(deps) > 0 {
			if id, ok := deps[0]["id"].(string); ok && id != "" {
				env.Run("logs", svc, "--deployment", id, "-s", svc)
			}
		}
	})

	t.Run("logs_tail", func(t *testing.T) {
		env.Run("logs", svc, "--tail", "1", "-s", svc)
	})

	t.Run("logs_bad_deployment", func(t *testing.T) {
		env.RunFail(t, "logs", svc, "--deployment", "bad-id-xyz", "-s", svc)
	})

	// Error cases
	t.Run("get_no_service", func(t *testing.T) {
		env.RunFail(t, "get", "deployments")
	})

	t.Run("delete_nonexistent", func(t *testing.T) {
		env.RunFail(t, "delete", "deployment", "nonexistent-dep-xyz", "--yes", "-s", svc)
	})

	t.Run("update_no_set_active", func(t *testing.T) {
		// `update deployment` without --set-active fails on flag validation
		// before any API call, so it works under any token type.
		r := env.RunOK(t, "get", "deployments", "-o", "json", "-s", svc)
		var deps []map[string]interface{}
		if json.Unmarshal([]byte(r.Stdout), &deps) == nil && len(deps) > 0 {
			if id, ok := deps[0]["id"].(string); ok && id != "" {
				env.RunFail(t, "update", "deployment", id)
			}
		}
	})

	t.Run("update_nonexistent", func(t *testing.T) {
		env.RunFail(t, "update", "deployment", "nonexistent-dep-xyz", "--set-active")
	})
}

// TestDeploymentLifecycle exercises rollback (delete) and reactivate
// (update --set-active). The service's image is updated to generate a
// second deployment.
//
//	go test -tags e2e -v -run TestDeploymentLifecycle ./tests/e2e/project/...
func TestDeploymentLifecycle(t *testing.T) {
	env := fixtureEnv(t)
	svc := createService(t, env)

	// Generate a second deployment by updating the image
	t.Log("Updating service image to create a second deployment...")
	env.RunOK(t, "update", "service", svc, "--image", "nginx:1.26-alpine")
	time.Sleep(5 * time.Second)

	// Get deployment list — need at least 2
	r := env.RunOK(t, "get", "deployments", "-o", "json", "-s", svc)
	var deps []map[string]interface{}
	if err := json.Unmarshal([]byte(r.Stdout), &deps); err != nil || len(deps) < 2 {
		t.Skipf("need at least 2 deployments for lifecycle tests (have %d)", len(deps))
	}

	latestID, _ := deps[0]["id"].(string)
	prevID, _ := deps[1]["id"].(string)
	t.Logf("Latest deployment: %s", latestID)
	t.Logf("Previous deployment: %s", prevID)

	t.Run("rollback_delete_latest", func(t *testing.T) {
		env.RunOK(t, "delete", "deployment", latestID, "--yes", "-s", svc)
		time.Sleep(3 * time.Second)
	})

	t.Run("verify_removed", func(t *testing.T) {
		r := env.RunOK(t, "get", "deployments", "-o", "json", "-s", svc)
		// Railway keeps deleted deployments with status "REMOVED"
		// rather than removing them from the list entirely.
		var updatedDeps []map[string]interface{}
		if json.Unmarshal([]byte(r.Stdout), &updatedDeps) == nil {
			for _, d := range updatedDeps {
				if id, _ := d["id"].(string); id == latestID {
					status, _ := d["status"].(string)
					if status != "REMOVED" {
						t.Errorf("expected deployment %s to have status REMOVED, got %s", latestID, status)
					}
					return
				}
			}
		}
	})

	t.Run("reactivate_previous", func(t *testing.T) {
		env.RunOK(t, "update", "deployment", prevID, "--set-active")
		time.Sleep(3 * time.Second)
	})

	t.Run("new_deployment_after_reactivate", func(t *testing.T) {
		r := env.RunOK(t, "get", "deployments", "-o", "json", "-s", svc)
		var newDeps []map[string]interface{}
		if json.Unmarshal([]byte(r.Stdout), &newDeps) == nil && len(newDeps) > 0 {
			newID, _ := newDeps[0]["id"].(string)
			if newID == latestID {
				t.Error("expected a new deployment ID after reactivation")
			}
		}
	})
}

// TestDeploymentAwait exercises the --await-completion flag.
//
//	go test -tags e2e -v -run TestDeploymentAwait ./tests/e2e/project/...
func TestDeploymentAwait(t *testing.T) {
	env := fixtureEnv(t)
	svc := createService(t, env)

	t.Run("create_deployment_await", func(t *testing.T) {
		env.RunOK(t, "create", "deployment", "--await-completion", "-s", svc)
	})

	t.Run("update_service_await", func(t *testing.T) {
		env.RunOK(t, "update", "service", svc, "--image", "nginx:1.26-alpine", "--await-completion")
	})
}
