//go:build e2e

package project

import (
	"encoding/json"
	"testing"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// TestVolumes exercises volume CRUD operations inside the shared fixture
// project under the minted project token (no -p/-e flags; -s kept).
//
//	go test -tags e2e -v -run TestVolumes ./tests/e2e/project/...
func TestVolumes(t *testing.T) {
	env := fixtureEnv(t)
	svc := createService(t, env)

	volName := "" // captured after creation
	t.Cleanup(func() {
		// Safety net if the test dies before its delete subtest.
		if volName != "" {
			env.Run("delete", "volume", volName, "--yes")
		}
	})

	t.Run("create", func(t *testing.T) {
		env.RunOK(t, "create", "volume", "--mount-path", "/app/data", "-s", svc)
		// Attachment propagation is not immediate — a fixed sleep flakes.
		if err := harness.WaitForVolume(env, svc+"-volume"); err != nil {
			t.Fatalf("volume did not propagate: %v", err)
		}
	})

	t.Run("get_table", func(t *testing.T) {
		r := env.RunOK(t, "get", "volumes")
		harness.AssertContains(t, r.Stdout, "/app/data")
	})

	t.Run("get_json", func(t *testing.T) {
		r := env.RunOK(t, "get", "volumes", "-o", "json")
		harness.AssertValidJSON(t, r.Stdout)
		// Extract volume name for subsequent tests
		var vols []map[string]interface{}
		if json.Unmarshal([]byte(r.Stdout), &vols) == nil && len(vols) > 0 {
			if n, ok := vols[0]["name"].(string); ok {
				volName = n
			}
		}
	})

	t.Run("get_yaml", func(t *testing.T) {
		r := env.RunOK(t, "get", "volumes", "-o", "yaml")
		harness.AssertValidYAML(t, r.Stdout)
	})

	t.Run("describe_table", func(t *testing.T) {
		if volName == "" {
			t.Skip("no volume name captured")
		}
		r := env.RunOK(t, "describe", "volume", volName)
		harness.AssertContains(t, r.Stdout, volName)
	})

	t.Run("describe_json", func(t *testing.T) {
		if volName == "" {
			t.Skip("no volume name captured")
		}
		r := env.RunOK(t, "describe", "volume", volName, "-o", "json")
		harness.AssertValidJSON(t, r.Stdout)
	})

	t.Run("describe_yaml", func(t *testing.T) {
		if volName == "" {
			t.Skip("no volume name captured")
		}
		r := env.RunOK(t, "describe", "volume", volName, "-o", "yaml")
		harness.AssertValidYAML(t, r.Stdout)
	})

	t.Run("update_name", func(t *testing.T) {
		if volName == "" {
			t.Skip("no volume name captured")
		}
		// Suite-1 rename attempt (no region anywhere): verified when the token
		// is authorized, logged when Railway denies volumeUpdate to it.
		volName = tryRenameVolume(t, env, volName)
	})

	// Error cases
	t.Run("create_bad_mount", func(t *testing.T) {
		env.RunFail(t, "create", "volume", "--mount-path", "no-slash", "-s", svc)
	})

	t.Run("create_no_service", func(t *testing.T) {
		env.RunFail(t, "create", "volume", "--mount-path", "/data")
	})

	// Cleanup: delete volume
	t.Run("delete", func(t *testing.T) {
		if volName == "" {
			t.Skip("no volume to delete")
		}
		env.RunOK(t, "delete", "volume", volName, "--yes")
		volName = ""
	})
}
