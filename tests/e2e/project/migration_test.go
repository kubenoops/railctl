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

// volumeSettleDelay is how long a freshly deployed, volume-backed service is
// left alone before its region is changed. Railway finalizes the volume
// attachment after the deployment reports SUCCESS, and migrating inside that
// window makes the platform abandon the copy and revert the placement ("Reset
// region due to volume migration failure"). Live evidence (2026-09-07): the
// same railctl migration, same region pair, run by hand against a settled
// volume succeeded in both directions, while the suite — which migrates
// seconds after create — reset three runs in a row.
const volumeSettleDelay = 45 * time.Second

// latestDeploymentStatus returns the status of the service's most recent
// deployment via `get deployments -s <svc>` — the per-service read. The
// project-level `get services` status is NOT usable here: it can serve a stale
// STOPPED/REMOVED entry while the service is healthy (observed live
// 2026-08-30), which is exactly the shape a superseded first deployment takes.
func latestDeploymentStatus(t *testing.T, env *harness.Env, name string) string {
	t.Helper()
	r := env.RunOK(t, "get", "deployments", "-s", name, "-o", "json", "--limit", "1")
	var deps []struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &deps); err != nil || len(deps) == 0 {
		return ""
	}
	return deps[0].Status
}

// waitForDeploySuccess polls until the service's latest deployment reads
// SUCCESS, then lets the volume attachment settle. Both halves matter:
// migrating a volume before the service ever deployed makes Railway fail the
// migration, and so does migrating in the seconds right after it. Returns
// false when SUCCESS never arrived (the caller's migration retry covers a
// genuine not-deployed failure).
func waitForDeploySuccess(t *testing.T, env *harness.Env, name string, deadline time.Duration) bool {
	t.Helper()
	end := time.Now().Add(deadline)
	for {
		status := latestDeploymentStatus(t, env, name)
		if status == "SUCCESS" {
			t.Logf("service %q deployed; settling %s before touching its volume", name, volumeSettleDelay)
			time.Sleep(volumeSettleDelay)
			return true
		}
		if time.Now().After(end) {
			t.Logf("service %q latest deployment did not read SUCCESS within %s (status %q) — proceeding, the migration retry covers a genuine not-deployed failure", name, deadline, status)
			return false
		}
		time.Sleep(10 * time.Second)
	}
}

// migrateWithRetries force-moves a volume-backed service to region `to` and
// waits for the VOLUME to land there, retrying the whole trigger when Railway
// resets the migration. Resets are transient platform failures (Railway staff
// have described them as internal errors and reset migration state by hand),
// so an operator would simply re-issue the move — with a pause, since the
// platform needs a moment after a reset.
func migrateWithRetries(t *testing.T, env *harness.Env, volName, to string, trigger func(), attempts int) bool {
	t.Helper()
	for attempt := 1; attempt <= attempts; attempt++ {
		trigger()
		if waitVolumeRegion(t, env, volName, to, 5*time.Minute) {
			return true
		}
		got := volumeRegion(t, env, volName)
		if attempt < attempts {
			t.Logf("volume still in %q after attempt %d/%d — Railway reset the migration; retrying in %s",
				got, attempt, attempts, volumeSettleDelay)
			time.Sleep(volumeSettleDelay)
		}
	}
	return false
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
	// Assert the actual migration outcome — the VOLUME's region (the service
	// placement can't be polled here — the project-level latestDeployment read
	// does not surface Railway's migration-initiated redeploy, observed live).
	// Resets are transient platform failures; retry like an operator would.
	if !migrateWithRetries(t, env, name+"-volume", to, func() {
		ok := env.RunOK(t, "update", "service", name, "--region", to, "--force")
		harness.AssertContains(t, ok.Stdout, to)
	}, 3) {
		t.Fatalf("volume %q did not migrate to %q after %d attempts (got %q)",
			name+"-volume", to, 3, volumeRegion(t, env, name+"-volume"))
	}

	// Suite-3 rename attempt: the volume has just been MIGRATED to another
	// region — records whether the volumeUpdate denial is affected by a
	// completed migration.
	volName := tryRenameVolume(t, env, name+"-volume")

	// Clean-room teardown is part of the contract under a project token.
	env.RunOK(t, "delete", "volume", volName, "--yes")
	env.RunOK(t, "delete", "service", name, "--yes")
}
