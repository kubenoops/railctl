//go:build e2e

package workspace

import (
	"encoding/json"
	"testing"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// TestDeleteProtectionResources exercises the v1.1.0 refinement of the
// DELETE_PROTECTION guard: a delete-protected environment shields DATA
// (volumes) and STRUCTURE (services) — those deletes are refused — but still
// allows CONFIGURATION and OPERATIONAL changes (image updates, variable
// set/delete), which do not call RequireDeletable.
//
//	go test -tags e2e -v -run TestDeleteProtectionResources ./tests/e2e/workspace/...
func TestDeleteProtectionResources(t *testing.T) {
	env := harness.SetupService(t, token)

	env.RunOK(t, env.WithPE("create", "volume", "--mount-path", "/app/data", "-s", env.ServiceName)...)

	// Discover the created volume's name (mirrors the Teardown parsing in
	// fixture.go — `get volumes -o json` returns a list of objects with a
	// "name" field).
	r := env.RunOK(t, env.WithPE("get", "volumes", "-o", "json")...)
	var vols []map[string]interface{}
	var volName string
	if json.Unmarshal([]byte(r.Stdout), &vols) == nil && len(vols) > 0 {
		if n, ok := vols[0]["name"].(string); ok {
			volName = n
		}
	}
	if volName == "" {
		t.Fatalf("could not determine created volume name from: %s", r.Stdout)
	}

	env.RunOK(t, env.WithPE("set", "variable", "FOO=bar", "-s", env.ServiceName)...)

	env.RunOK(t, env.WithP("protect", "environment", env.EnvName)...)

	// Immediately register the unprotect so the suite's Teardown (which
	// deletes services/volumes then the project) is never blocked by the
	// protection this test arms.
	t.Cleanup(func() {
		env.Run(env.WithP("unprotect", "environment", env.EnvName)...)
	})

	t.Run("delete_service_blocked", func(t *testing.T) {
		r := env.RunFail(t, env.WithPE("delete", "service", env.ServiceName, "--yes")...)
		harness.AssertContains(t, r.Stderr, "delete-protected")
		harness.AssertContains(t, r.Stderr, "cannot delete service")
	})

	t.Run("delete_volume_blocked", func(t *testing.T) {
		r := env.RunFail(t, env.WithPE("delete", "volume", volName, "--yes")...)
		harness.AssertContains(t, r.Stderr, "delete-protected")
		harness.AssertContains(t, r.Stderr, "cannot delete volume")
	})

	t.Run("update_service_allowed", func(t *testing.T) {
		env.RunOK(t, env.WithPE("update", "service", env.ServiceName, "--image", "nginx:1.26-alpine")...)
	})

	t.Run("set_variable_allowed", func(t *testing.T) {
		env.RunOK(t, env.WithPE("set", "variable", "BAZ=qux", "-s", env.ServiceName)...)
	})

	t.Run("delete_variable_allowed", func(t *testing.T) {
		env.RunOK(t, env.WithPE("delete", "variable", "BAZ", "-s", env.ServiceName, "--yes")...)
	})

	t.Run("unprotect_then_delete_service", func(t *testing.T) {
		env.RunOK(t, env.WithP("unprotect", "environment", env.EnvName)...)
		env.RunOK(t, env.WithPE("delete", "service", env.ServiceName, "--yes")...)
	})
}
