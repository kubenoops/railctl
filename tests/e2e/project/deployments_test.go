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
//
// waitForDeployments polls `get deployments -o json` until at least min
// deployments are visible. Railway registers the first deployment
// asynchronously after service creation, so listing too early legitimately
// returns an empty list. Returns the count seen (may be < min on timeout).
func waitForDeployments(t *testing.T, env *harness.Env, svc string, min int) int {
	t.Helper()
	seen := 0
	for i := 0; i < 20; i++ {
		r := env.Run("get", "deployments", "-o", "json", "-s", svc)
		var deps []map[string]interface{}
		if r.ExitCode == 0 && json.Unmarshal([]byte(r.Stdout), &deps) == nil {
			seen = len(deps)
			if seen >= min {
				t.Logf("%d deployment(s) visible after %d poll(s)", seen, i+1)
				return seen
			}
		}
		time.Sleep(3 * time.Second)
	}
	return seen
}

func TestDeployments(t *testing.T) {
	env := fixtureEnv(t)
	svc := createService(t, env)
	// `create service` does not trigger a deployment itself (Railway may or
	// may not auto-deploy asynchronously) — create one explicitly so the
	// listing subtests have deterministic content.
	env.RunOK(t, "create", "deployment", "-s", svc)
	if got := waitForDeployments(t, env, svc, 1); got < 1 {
		t.Fatal("no deployment became visible within 60s of explicit create deployment")
	}

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

// TestDeploymentLifecycle exercises rollback (delete) and the reactivation
// boundary: `update deployment --set-active` is a workspace-level capability
// that Railway denies to project tokens. The service's image is updated to
// generate a second deployment.
//
//	go test -tags e2e -v -run TestDeploymentLifecycle ./tests/e2e/project/...
func TestDeploymentLifecycle(t *testing.T) {
	env := fixtureEnv(t)
	svc := createService(t, env)

	// First deployment: explicit (create service does not deploy by itself).
	env.RunOK(t, "create", "deployment", "-s", svc)
	if got := waitForDeployments(t, env, svc, 1); got < 1 {
		t.Skip("first deployment not visible within 60s")
	}

	// Generate a second deployment by updating the image
	t.Log("Updating service image to create a second deployment...")
	env.RunOK(t, "update", "service", svc, "--image", "nginx:1.26-alpine")

	// Poll until both deployments are visible (registration is async).
	if got := waitForDeployments(t, env, svc, 2); got < 2 {
		t.Skipf("need at least 2 deployments for lifecycle tests (have %d)", got)
	}
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

	t.Run("reactivate_previous_denied", func(t *testing.T) {
		// Deployment reactivation is a workspace-level capability: Railway
		// denies deploymentRedeploy to project tokens ("Not Authorized").
		// The positive reactivation test lives in the workspace group
		// (tests/e2e/workspace, TestDeploymentReactivate).
		r := env.RunFail(t, "update", "deployment", prevID, "--set-active")
		harness.AssertContains(t, r.Stdout+r.Stderr, "Not Authorized")
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
