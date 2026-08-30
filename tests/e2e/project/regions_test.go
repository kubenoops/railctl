//go:build e2e

package project

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// discoverRegions returns the available region names via `get regions -o json`.
func discoverRegions(t *testing.T, env *harness.Env) []string {
	t.Helper()
	r := env.RunOK(t, "get", "regions", "-o", "json")
	harness.AssertValidJSON(t, r.Stdout)
	var regions []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &regions); err != nil {
		t.Fatalf("parse regions json: %v", err)
	}
	names := make([]string, 0, len(regions))
	for _, rg := range regions {
		names = append(names, rg.Name)
	}
	return names
}

// TestRegions exercises region discovery and placement inside the shared fixture
// project under the minted project token (no -p/-e flags).
//
//	go test -tags e2e -v -run TestRegions ./tests/e2e/project/...
func TestRegions(t *testing.T) {
	env := fixtureEnv(t)

	var regionNames []string

	t.Run("get_regions_table", func(t *testing.T) {
		r := env.RunOK(t, "get", "regions")
		harness.AssertContains(t, r.Stdout, "NAME")
	})

	t.Run("get_regions_wide", func(t *testing.T) {
		r := env.RunOK(t, "get", "regions", "-o", "wide")
		harness.AssertContains(t, r.Stdout, "COUNTRY")
	})

	t.Run("get_regions_yaml", func(t *testing.T) {
		r := env.RunOK(t, "get", "regions", "-o", "yaml")
		harness.AssertValidYAML(t, r.Stdout)
	})

	t.Run("get_regions_json", func(t *testing.T) {
		regionNames = discoverRegions(t, env)
		if len(regionNames) == 0 {
			t.Fatal("expected at least one region")
		}
	})

	t.Run("create_with_region", func(t *testing.T) {
		if len(regionNames) == 0 {
			t.Skip("no regions discovered")
		}
		region := regionNames[0]
		name := harness.UniqueName()
		t.Cleanup(func() { env.Run("delete", "service", name, "--yes") })

		r := env.RunOK(t, "create", "service", name, "--image", env.ServiceImg, "--region", region)
		harness.AssertContains(t, r.Stdout, region)
	})

	t.Run("create_with_unknown_region_fails", func(t *testing.T) {
		env.RunFail(t, "create", "service", harness.UniqueName(),
			"--image", env.ServiceImg, "--region", "nowhere-central9")
	})

	t.Run("update_region_moves_and_noop", func(t *testing.T) {
		if len(regionNames) < 2 {
			t.Skip("need at least two regions to test a move")
		}
		from, to := regionNames[0], regionNames[1]
		name := harness.UniqueName()
		t.Cleanup(func() { env.Run("delete", "service", name, "--yes") })

		env.RunOK(t, "create", "service", name, "--image", env.ServiceImg, "--region", from)
		time.Sleep(3 * time.Second)

		// Move to a different region.
		r := env.RunOK(t, "update", "service", name, "--region", to)
		harness.AssertContains(t, r.Stdout, to)

		// Re-issuing the same region is a no-op.
		r2 := env.RunOK(t, "update", "service", name, "--region", to)
		harness.AssertContains(t, r2.Stdout, "already in region")
	})

	t.Run("region_change_blocked_by_volume", func(t *testing.T) {
		if len(regionNames) < 2 {
			t.Skip("need at least two regions to test a move")
		}
		from, to := regionNames[0], regionNames[1]
		name := harness.UniqueName()
		t.Cleanup(func() {
			// The volume is created unnamed, so it gets Railway's auto-name
			// "<service>-volume".
			env.Run("delete", "volume", name+"-volume", "--yes")
			env.Run("delete", "service", name, "--yes")
		})

		env.RunOK(t, "create", "service", name, "--image", env.ServiceImg, "--region", from)
		time.Sleep(3 * time.Second)
		// Create the volume WITHOUT a custom name: railctl renames a named volume
		// after creation, and volume rename is not authorized for a project token.
		env.RunOK(t, "create", "volume", "--mount-path", "/data", "-s", name)
		// The guard reads ListVolumes — poll the same view until the attachment
		// is visible (a fixed sleep flakes when propagation is slow).
		if err := harness.WaitForVolume(env, name+"-volume"); err != nil {
			t.Fatalf("volume attachment did not propagate: %v", err)
		}

		r := env.RunFail(t, "update", "service", name, "--region", to)
		harness.AssertContains(t, r.Stderr, "migrate")
		harness.AssertContains(t, r.Stderr, "--force")

		// Suite-2 rename attempt: the volume is attached to a REGION-PLACED
		// service — records whether the volumeUpdate denial is region-dependent.
		tryRenameVolume(t, env, name+"-volume")
	})
}
