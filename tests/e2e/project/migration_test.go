//go:build e2e

package project

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// waitVolumeRegion polls until the named volume reads the wanted (short)
// region, returning false on deadline. A migration that Railway failed
// presents as the volume staying in the source region (the platform commits
// "Reset region due to volume migration failure" and parks a staged retry).
func waitVolumeRegion(t *testing.T, env *harness.Env, volName, want string, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if volumeRegion(t, env, volName) == want {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(10 * time.Second)
	}
}

// volumeRegion returns the (short) region of the named volume via
// `get volumes -o json`, or "" when the volume isn't listed.
func volumeRegion(t *testing.T, env *harness.Env, volName string) string {
	t.Helper()
	r := env.RunOK(t, "get", "volumes", "-o", "json")
	var volumes []struct {
		Name   string `json:"name"`
		Region string `json:"region"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &volumes); err != nil {
		t.Fatalf("parse volumes json: %v", err)
	}
	for _, v := range volumes {
		if v.Name == volName {
			return v.Region
		}
	}
	return ""
}

// serviceState returns the live single-region placement and latest deployment
// status of the named service via `get services -o json` (region is empty when
// default-placed or multi-region).
func serviceState(t *testing.T, env *harness.Env, name string) (region, status string) {
	t.Helper()
	r := env.RunOK(t, "get", "services", "-o", "json")
	var services []struct {
		Name   string `json:"name"`
		Region string `json:"region"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &services); err != nil {
		t.Fatalf("parse services json: %v", err)
	}
	for _, s := range services {
		if s.Name == name {
			return s.Region, s.Status
		}
	}
	t.Fatalf("service %q not found in get services output", name)
	return "", ""
}

// waitForDeploySuccess polls until the service's latest deployment reads
// SUCCESS, returning false on deadline. Migrating a volume before the service
// ever deployed makes Railway fail the migration ("Reset region due to volume
// migration failure"), so callers settle first — but only best-effort: the
// project-level deployment read can stick on a stale STOPPED entry (observed
// live 2026-08-30) while the service is actually fine, and a failed migration
// is retried by the caller anyway.
func waitForDeploySuccess(t *testing.T, env *harness.Env, name string, deadline time.Duration) bool {
	t.Helper()
	end := time.Now().Add(deadline)
	for {
		_, status := serviceState(t, env, name)
		if status == "SUCCESS" {
			return true
		}
		if time.Now().After(end) {
			t.Logf("service %q deployment did not read SUCCESS within %s (status %q; possibly a stale read) — proceeding, the migration retry covers a genuine not-deployed failure", name, deadline, status)
			return false
		}
		time.Sleep(10 * time.Second)
	}
}

// TestRegionVolumeMigration exercises REQ-VOL-100 live end to end, including
// Railway's actual volume migration and the project-token-safe deletions:
//
//	create (region A) + attach volume
//	→ update --region B           refused: migration + downtime, names --force
//	→ update --region B --force   accepted: Railway migrates the volume
//	→ placement reads region B
//	→ delete volume, delete service (clean room)
//
//	go test -tags e2e -v -run TestRegionVolumeMigration ./tests/e2e/project/...
func TestRegionVolumeMigration(t *testing.T) {
	env := fixtureEnv(t)

	regionNames := discoverRegions(t, env)
	if len(regionNames) < 2 {
		t.Skip("need at least two regions to test a migration")
	}
	from, to := regionNames[0], regionNames[1]

	name := harness.UniqueName()
	t.Cleanup(func() {
		// Both deletions work under a project token (volume via the
		// environmentPatchCommit path, service via env-scoped serviceDelete),
		// so a failed run must not leak resources.
		env.Run("delete", "volume", name+"-volume", "--yes")
		env.Run("delete", "service", name, "--yes")
	})

	env.RunOK(t, "create", "service", name, "--image", env.ServiceImg, "--region", from)
	time.Sleep(3 * time.Second)
	// Unnamed: volume rename is not authorized for a project token, so the
	// volume gets Railway's auto-name "<service>-volume".
	env.RunOK(t, "create", "volume", "--mount-path", "/data", "-s", name)
	if err := harness.WaitForVolume(env, name+"-volume"); err != nil {
		t.Fatalf("volume attachment did not propagate: %v", err)
	}

	// Without --force: refused with the migration/downtime message. (REQ-VOL-100)
	r := env.RunFail(t, "update", "service", name, "--region", to)
	harness.AssertContains(t, r.Stderr, "migrate")
	harness.AssertContains(t, r.Stderr, "--force")

	// Attaching the volume triggers a redeploy; wait for it to settle rather
	// than racing it with another deployment (a concurrent create deployment
	// was observed to fail INITIALIZING → FAILED).
	waitForDeploySuccess(t, env, name, 3*time.Minute)

	// With --force: accepted, Railway migrates the volume alongside the move.
	ok := env.RunOK(t, "update", "service", name, "--region", to, "--force")
	harness.AssertContains(t, ok.Stdout, to)

	// Assert the actual migration outcome: the VOLUME's region. (The service
	// placement can't be polled here — the project-level latestDeployment read
	// does not surface Railway's migration-initiated redeploy, observed live.)
	// Railway migrations fail transiently ("Reset region due to volume
	// migration failure", observed on a clean metal→metal move) — retry once,
	// like an operator would from the dashboard.
	if !waitVolumeRegion(t, env, name+"-volume", to, 5*time.Minute) {
		t.Logf("volume still in %q — Railway reset the migration; retrying once", volumeRegion(t, env, name+"-volume"))
		env.RunOK(t, "update", "service", name, "--region", to, "--force")
		if !waitVolumeRegion(t, env, name+"-volume", to, 5*time.Minute) {
			t.Fatalf("volume %q did not migrate to %q after a retry (got %q)",
				name+"-volume", to, volumeRegion(t, env, name+"-volume"))
		}
	}

	// Suite-3 rename attempt: the volume has just been MIGRATED to another
	// region — records whether the volumeUpdate denial is affected by a
	// completed migration.
	volName := tryRenameVolume(t, env, name+"-volume")

	// Clean-room teardown is part of the contract under a project token.
	env.RunOK(t, "delete", "volume", volName, "--yes")
	env.RunOK(t, "delete", "service", name, "--yes")
}
