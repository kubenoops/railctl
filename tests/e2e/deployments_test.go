//go:build e2e

package e2e

import (
	"encoding/json"
	"testing"
	"time"
)

// TestDeployments exercises deployment list, logs, and error cases.
// Setup: creates project + environment + service (which triggers a deployment)
//
//	go test -tags e2e -v -run TestDeployments ./tests/e2e/...
func TestDeployments(t *testing.T) {
	env := SetupService(t)

	t.Run("get_table", func(t *testing.T) {
		env.RunOK(t, env.WithPES("get", "deployments")...)
	})

	t.Run("get_json", func(t *testing.T) {
		r := env.RunOK(t, env.WithPES("get", "deployments", "-o", "json")...)
		AssertValidJSON(t, r.Stdout)
	})

	t.Run("get_yaml", func(t *testing.T) {
		r := env.RunOK(t, env.WithPES("get", "deployments", "-o", "yaml")...)
		AssertValidYAML(t, r.Stdout)
	})

	t.Run("get_wide", func(t *testing.T) {
		env.RunOK(t, env.WithPES("get", "deployments", "-o", "wide")...)
	})

	t.Run("get_limit", func(t *testing.T) {
		env.RunOK(t, env.WithPES("get", "deployments", "--limit", "1")...)
	})

	t.Run("logs_latest", func(t *testing.T) {
		// Logs may or may not have content — just verify the command runs
		env.Run(env.WithPES("logs", env.ServiceName)...)
	})

	t.Run("logs_by_deployment_id", func(t *testing.T) {
		r := env.RunOK(t, env.WithPES("get", "deployments", "-o", "json")...)
		var deps []map[string]interface{}
		if json.Unmarshal([]byte(r.Stdout), &deps) == nil && len(deps) > 0 {
			if id, ok := deps[0]["id"].(string); ok && id != "" {
				env.Run(env.WithPES("logs", env.ServiceName, "--deployment", id)...)
			}
		}
	})

	t.Run("logs_tail", func(t *testing.T) {
		env.Run(env.WithPES("logs", env.ServiceName, "--tail", "1")...)
	})

	t.Run("logs_bad_deployment", func(t *testing.T) {
		env.RunFail(t, env.WithPES("logs", env.ServiceName, "--deployment", "bad-id-xyz")...)
	})

	// Error cases
	t.Run("get_no_service", func(t *testing.T) {
		env.RunFail(t, env.WithPE("get", "deployments")...)
	})

	t.Run("delete_nonexistent", func(t *testing.T) {
		env.RunFail(t, env.WithPES("delete", "deployment", "nonexistent-dep-xyz", "--yes")...)
	})

	t.Run("update_no_set_active", func(t *testing.T) {
		r := env.RunOK(t, env.WithPES("get", "deployments", "-o", "json")...)
		var deps []map[string]interface{}
		if json.Unmarshal([]byte(r.Stdout), &deps) == nil && len(deps) > 0 {
			if id, ok := deps[0]["id"].(string); ok && id != "" {
				env.RunFail(t, env.WithPES("update", "deployment", id)...)
			}
		}
	})

	t.Run("update_nonexistent", func(t *testing.T) {
		env.RunFail(t, env.WithPES("update", "deployment", "nonexistent-dep-xyz", "--set-active")...)
	})
}

// TestDeploymentLifecycle exercises rollback (delete) and reactivate (update --set-active).
// Setup: creates project + environment + service, then updates the image to generate
// a second deployment.
//
//	go test -tags e2e -v -run TestDeploymentLifecycle ./tests/e2e/...
func TestDeploymentLifecycle(t *testing.T) {
	env := SetupService(t)

	// Generate a second deployment by updating the image
	t.Log("Updating service image to create a second deployment...")
	env.RunOK(t, env.WithPE("update", "service", env.ServiceName, "--image", "nginx:1.26-alpine")...)
	time.Sleep(5 * time.Second)

	// Get deployment list — need at least 2
	r := env.RunOK(t, env.WithPES("get", "deployments", "-o", "json")...)
	var deps []map[string]interface{}
	if err := json.Unmarshal([]byte(r.Stdout), &deps); err != nil || len(deps) < 2 {
		t.Skipf("need at least 2 deployments for lifecycle tests (have %d)", len(deps))
	}

	latestID, _ := deps[0]["id"].(string)
	prevID, _ := deps[1]["id"].(string)
	t.Logf("Latest deployment: %s", latestID)
	t.Logf("Previous deployment: %s", prevID)

	t.Run("rollback_delete_latest", func(t *testing.T) {
		env.RunOK(t, env.WithPES("delete", "deployment", latestID, "--yes")...)
		time.Sleep(3 * time.Second)
	})

	t.Run("verify_removed", func(t *testing.T) {
		r := env.RunOK(t, env.WithPES("get", "deployments", "-o", "json")...)
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
		env.RunOK(t, env.WithPES("update", "deployment", prevID, "--set-active")...)
		time.Sleep(3 * time.Second)
	})

	t.Run("new_deployment_after_reactivate", func(t *testing.T) {
		r := env.RunOK(t, env.WithPES("get", "deployments", "-o", "json")...)
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
// Setup: creates project + environment + service, then triggers a deployment
// with --await-completion to verify the CLI waits for completion.
//
//	go test -tags e2e -v -run TestDeploymentAwait ./tests/e2e/...
func TestDeploymentAwait(t *testing.T) {
	env := SetupService(t)

	t.Run("create_deployment_await", func(t *testing.T) {
		env.RunOK(t, env.WithPES("create", "deployment", "--await-completion")...)
	})

	t.Run("update_service_await", func(t *testing.T) {
		env.RunOK(t, env.WithPE("update", "service", env.ServiceName, "--image", "nginx:1.26-alpine", "--await-completion")...)
	})
}
