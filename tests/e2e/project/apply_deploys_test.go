//go:build e2e

package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// TestApplyMultiServiceDeploys pins the rule that every service a multi-service
// apply creates ends up WITH a deployment.
//
// Regression guard: apply used to rely on serviceCreate rolling out implicitly.
// That is unreliable — a real 8-service apply left 7 services existing with no
// deployment at all — and `--await` then silently skipped them and exited 0, so
// a systemically broken apply reported success. A service with zero deployments
// has nothing running, which is a systemic failure, not an unhealthy deploy.
//
// The assertion is deliberately about EXISTENCE, not health: each service must
// have a deployment, whatever status it reaches.
func TestApplyMultiServiceDeploys(t *testing.T) {
	env := fixtureEnv(t)

	// Three services in one manifest — the multi-service case that broke.
	names := []string{harness.UniqueName(), harness.UniqueName(), harness.UniqueName()}
	for _, n := range names {
		n := n
		t.Cleanup(func() { env.Run("delete", "service", n, "--yes") })
	}

	manifest := "services:\n"
	for _, n := range names {
		manifest += fmt.Sprintf("  - name: %s\n    image: %s\n", n, env.ServiceImg)
	}
	path := filepath.Join(t.TempDir(), "stack.yaml")
	if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
		t.Fatalf("writing manifest: %v", err)
	}

	// Flag-free under the project token; --await must not return until every
	// created service has a deployment that reached a terminal status.
	env.RunOK(t, "apply", "-f", path, "--await", "--await-timeout", "420")

	// Every service must have at least one deployment.
	for _, n := range names {
		r := env.RunOK(t, "get", "deployments", "-s", n, "-o", "json", "--limit", "1")
		var deps []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}
		if err := json.Unmarshal([]byte(r.Stdout), &deps); err != nil {
			t.Fatalf("parsing deployments for %s: %v\nstdout: %s", n, err, r.Stdout)
		}
		if len(deps) == 0 {
			t.Errorf("service %s has NO deployment after apply --await — nothing is running for it", n)
			continue
		}
		t.Logf("service %s has deployment %s (status %s)", n, deps[0].ID, deps[0].Status)
	}
}
