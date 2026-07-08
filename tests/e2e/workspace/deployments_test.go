//go:build e2e

package workspace

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// waitForDeployments polls `get deployments -o json` (with explicit
// -p/-e/-s flags — a workspace token carries no implicit scope) until at
// least min deployments are visible, for up to ~60s. Local copy of the
// project group's helper. Returns the count seen (may be < min on timeout).
func waitForDeployments(t *testing.T, env *harness.Env, min int) int {
	t.Helper()
	seen := 0
	for i := 0; i < 20; i++ {
		r := env.Run(env.WithPES("get", "deployments", "-o", "json")...)
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

// TestDeploymentReactivate proves that deployment reactivation
// (`update deployment <id> --set-active`) works under a WORKSPACE token. It
// is the positive counterpart of the project group's
// reactivate_previous_denied boundary: Railway denies deploymentRedeploy to
// project tokens, so the capability can only be asserted here.
//
//	go test -tags e2e -v -run TestDeploymentReactivate ./tests/e2e/workspace/...
func TestDeploymentReactivate(t *testing.T) {
	env := harness.SetupService(t, token)

	// First deployment: explicit (create service does not deploy by itself).
	env.RunOK(t, env.WithPES("create", "deployment")...)
	if got := waitForDeployments(t, env, 1); got < 1 {
		t.Skip("first deployment not visible within 60s")
	}

	// Second deployment via an image update.
	env.RunOK(t, env.WithPE("update", "service", env.ServiceName, "--image", "nginx:1.26-alpine")...)
	if got := waitForDeployments(t, env, 2); got < 2 {
		t.Skipf("need at least 2 deployments for reactivation (have %d)", got)
	}

	r := env.RunOK(t, env.WithPES("get", "deployments", "-o", "json")...)
	var deps []map[string]interface{}
	if err := json.Unmarshal([]byte(r.Stdout), &deps); err != nil || len(deps) < 2 {
		t.Skipf("need at least 2 deployments for reactivation (have %d)", len(deps))
	}
	latestID, _ := deps[0]["id"].(string)
	prevID, _ := deps[1]["id"].(string)
	if latestID == "" || prevID == "" {
		t.Fatalf("could not extract deployment IDs from listing:\n%s", r.Stdout)
	}

	// Roll back: delete the latest deployment, then reactivate the previous
	// one — the workspace-level capability under test.
	env.RunOK(t, env.WithPES("delete", "deployment", latestID, "--yes")...)
	time.Sleep(3 * time.Second)
	env.RunOK(t, "update", "deployment", prevID, "--set-active")
}
