//go:build e2e

package e2e

import (
	"encoding/json"
	"testing"
	"time"
)

// TestVolumes exercises volume CRUD operations.
// Setup: creates project + environment + service
//
//	go test -tags e2e -v -run TestVolumes ./tests/e2e/...
func TestVolumes(t *testing.T) {
	env := SetupService(t)

	volName := "" // captured after creation

	t.Run("create", func(t *testing.T) {
		env.RunOK(t, env.WithPES("create", "volume", "--mount-path", "/app/data")...)
		time.Sleep(2 * time.Second)
	})

	t.Run("get_table", func(t *testing.T) {
		r := env.RunOK(t, env.WithPE("get", "volumes")...)
		AssertContains(t, r.Stdout, "/app/data")
	})

	t.Run("get_json", func(t *testing.T) {
		r := env.RunOK(t, env.WithPE("get", "volumes", "-o", "json")...)
		AssertValidJSON(t, r.Stdout)
		// Extract volume name for subsequent tests
		var vols []map[string]interface{}
		if json.Unmarshal([]byte(r.Stdout), &vols) == nil && len(vols) > 0 {
			if n, ok := vols[0]["name"].(string); ok {
				volName = n
			}
		}
	})

	t.Run("get_yaml", func(t *testing.T) {
		r := env.RunOK(t, env.WithPE("get", "volumes", "-o", "yaml")...)
		AssertValidYAML(t, r.Stdout)
	})

	t.Run("describe_table", func(t *testing.T) {
		if volName == "" {
			t.Skip("no volume name captured")
		}
		r := env.RunOK(t, env.WithPE("describe", "volume", volName)...)
		AssertContains(t, r.Stdout, volName)
	})

	t.Run("describe_json", func(t *testing.T) {
		if volName == "" {
			t.Skip("no volume name captured")
		}
		r := env.RunOK(t, env.WithPE("describe", "volume", volName, "-o", "json")...)
		AssertValidJSON(t, r.Stdout)
	})

	t.Run("describe_yaml", func(t *testing.T) {
		if volName == "" {
			t.Skip("no volume name captured")
		}
		r := env.RunOK(t, env.WithPE("describe", "volume", volName, "-o", "yaml")...)
		AssertValidYAML(t, r.Stdout)
	})

	t.Run("update_name", func(t *testing.T) {
		if volName == "" {
			t.Skip("no volume name captured")
		}
		env.RunOK(t, env.WithPE("update", "volume", volName, "--name", "e2e-renamed-vol")...)
		volName = "e2e-renamed-vol"
	})

	// Error cases
	t.Run("create_bad_mount", func(t *testing.T) {
		env.RunFail(t, env.WithPES("create", "volume", "--mount-path", "no-slash")...)
	})

	t.Run("create_no_service", func(t *testing.T) {
		env.RunFail(t, env.WithPE("create", "volume", "--mount-path", "/data")...)
	})

	// Cleanup: delete volume
	t.Run("delete", func(t *testing.T) {
		if volName == "" {
			t.Skip("no volume to delete")
		}
		env.RunOK(t, env.WithPE("delete", "volume", volName, "--yes")...)
	})
}
