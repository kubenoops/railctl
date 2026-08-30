//go:build e2e

package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// The declarative twins of the three imperative suites. Each drives the full
// manifest cycle — write stack → diff → apply → verify live → diff idempotent
// → modify stack → diff shows the change → apply → verify — and tears down
// with `delete -f` (clean room: the environment ends empty).
//
//	suite 1  TestDeclarativeBaseline   services + volume, NO region anywhere
//	suite 2  TestDeclarativeRegion     region lifecycle + the apply volume guard
//	suite 3  TestDeclarativeMigration  volume migration via apply --force

// writeStack writes the manifest and returns its path (stable per test).
func writeStack(t *testing.T, dir, content string) string {
	t.Helper()
	p := filepath.Join(dir, "stack.yaml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("writing stack file: %v", err)
	}
	return p
}

// TestDeclarativeBaseline — suite 1, declaratively: a two-service stack (one
// volume-backed), no region anywhere.
//
//	go test -tags e2e -v -run TestDeclarativeBaseline ./tests/e2e/project/...
func TestDeclarativeBaseline(t *testing.T) {
	env := fixtureEnv(t)
	app := harness.UniqueName()
	db := harness.UniqueName()
	dir := t.TempDir()

	stack := func(appImage string) string {
		return fmt.Sprintf(`services:
  - name: %s
    image: %s
    variables:
      PORT: "80"
  - name: %s
    image: nginx:1.25-alpine
    volume:
      mountPath: /data
`, app, appImage, db)
	}
	cfg := writeStack(t, dir, stack("nginx:1.25-alpine"))

	t.Cleanup(func() {
		// Belt and braces if the delete -f step didn't run.
		env.Run("delete", "-f", cfg, "--yes")
	})

	// 1. diff reports both creates.
	r := env.RunOK(t, "diff", "-f", cfg)
	harness.AssertContains(t, r.Stdout, app)
	harness.AssertContains(t, r.Stdout, db)
	harness.AssertContains(t, r.Stdout, "create")

	// 2. apply creates the stack.
	r = env.RunOK(t, "apply", "-f", cfg)
	harness.AssertContains(t, r.Stdout, "Created")
	time.Sleep(3 * time.Second)

	// 3. Verify live state: services + the volume at its mount path.
	r = env.RunOK(t, "get", "services")
	harness.AssertContains(t, r.Stdout, app)
	harness.AssertContains(t, r.Stdout, db)
	if err := harness.WaitForVolume(env, db+"-volume"); err != nil {
		t.Fatalf("declared volume did not appear: %v", err)
	}
	r = env.RunOK(t, "get", "volumes")
	harness.AssertContains(t, r.Stdout, "/data")

	// 4. Idempotency: diff and apply report no changes.
	r = env.RunOK(t, "diff", "-f", cfg)
	harness.AssertContains(t, r.Stdout, "No changes")
	r = env.RunOK(t, "apply", "-f", cfg)
	harness.AssertContains(t, r.Stdout, "No changes")

	// 5. Modify the stack (image bump) → diff shows exactly one update, no
	//    creates (the summary line is "N to create, M to update, K to delete").
	cfg = writeStack(t, dir, stack("nginx:1.26-alpine"))
	r = env.RunOK(t, "diff", "-f", cfg)
	harness.AssertContains(t, r.Stdout, "nginx:1.26-alpine")
	harness.AssertContains(t, r.Stdout, "0 to create, 1 to update, 0 to delete")

	// 6. apply reconciles; diff converges back to no changes.
	env.RunOK(t, "apply", "-f", cfg)
	time.Sleep(3 * time.Second)
	r = env.RunOK(t, "diff", "-f", cfg)
	harness.AssertContains(t, r.Stdout, "No changes")

	// 7. Teardown declaratively; the environment ends clean.
	r = env.RunOK(t, "delete", "-f", cfg, "--yes")
	harness.AssertContains(t, r.Stdout, "2 services deleted")
	harness.AssertContains(t, r.Stdout, "1 volumes deleted")
	time.Sleep(3 * time.Second)
	r = env.RunOK(t, "get", "services")
	harness.AssertNotContains(t, r.Stdout, app)
	harness.AssertNotContains(t, r.Stdout, db)
}

// TestDeclarativeRegion — suite 2, declaratively: region lifecycle through the
// manifest, plus the apply-side volume guard (REQ-VOL-101) up to the refusal.
// The actual migration is suite 3.
//
//	go test -tags e2e -v -run TestDeclarativeRegion ./tests/e2e/project/...
func TestDeclarativeRegion(t *testing.T) {
	env := fixtureEnv(t)
	regions := discoverRegions(t, env)
	if len(regions) < 2 {
		t.Skip("need at least two regions")
	}
	from, to := regions[0], regions[1]

	svc := harness.UniqueName()
	vol := harness.UniqueName()
	dir := t.TempDir()

	stack := func(svcRegion, volRegion string) string {
		return fmt.Sprintf(`services:
  - name: %s
    image: nginx:1.25-alpine
    deploy:
      region: %s
      replicas: 1
  - name: %s
    image: nginx:1.25-alpine
    deploy:
      region: %s
      replicas: 1
    volume:
      mountPath: /data
`, svc, svcRegion, vol, volRegion)
	}
	cfg := writeStack(t, dir, stack(from, from))

	t.Cleanup(func() {
		env.Run("delete", "-f", cfg, "--yes")
	})

	// 1. diff shows the region on create; apply creates both in region A.
	r := env.RunOK(t, "diff", "-f", cfg)
	harness.AssertContains(t, r.Stdout, "deploy.region")
	harness.AssertContains(t, r.Stdout, from)
	env.RunOK(t, "apply", "-f", cfg)
	time.Sleep(3 * time.Second)
	if err := harness.WaitForVolume(env, vol+"-volume"); err != nil {
		t.Fatalf("declared volume did not appear: %v", err)
	}

	// 2. Idempotency for region-placed services.
	r = env.RunOK(t, "diff", "-f", cfg)
	harness.AssertContains(t, r.Stdout, "No changes")

	// 3. Move the volume-less service to region B → diff shows the region
	//    change, apply reconciles without --force (no volume attached).
	cfg = writeStack(t, dir, stack(to, from))
	r = env.RunOK(t, "diff", "-f", cfg)
	harness.AssertContains(t, r.Stdout, "deploy.region")
	harness.AssertContains(t, r.Stdout, to)
	env.RunOK(t, "apply", "-f", cfg)
	time.Sleep(3 * time.Second)
	r = env.RunOK(t, "diff", "-f", cfg)
	harness.AssertContains(t, r.Stdout, "No changes")

	// 4. REQ-VOL-101: a region change on the VOLUME-backed service is refused
	//    without --force, before mutating anything.
	cfg = writeStack(t, dir, stack(to, to))
	r = env.RunFail(t, "apply", "-f", cfg)
	harness.AssertContains(t, r.Stderr, "migrate")
	harness.AssertContains(t, r.Stderr, "--force")

	// 5. Suite-2 rename probe: volume attached to a region-placed service.
	tryRenameVolume(t, env, vol+"-volume")

	// 6. Revert the stack (volume service stays in region A) and tear down.
	cfg = writeStack(t, dir, stack(to, from))
	r = env.RunOK(t, "delete", "-f", cfg, "--yes")
	harness.AssertContains(t, r.Stdout, "2 services deleted")
	harness.AssertContains(t, r.Stdout, "1 volumes deleted")
}

// TestDeclarativeMigration — suite 3, declaratively: a volume-backed service
// migrates regions through the manifest with apply --force.
//
//	go test -tags e2e -v -run TestDeclarativeMigration ./tests/e2e/project/...
func TestDeclarativeMigration(t *testing.T) {
	env := fixtureEnv(t)
	regions := discoverRegions(t, env)
	if len(regions) < 2 {
		t.Skip("need at least two regions")
	}
	from, to := regions[0], regions[1]

	svc := harness.UniqueName()
	dir := t.TempDir()
	stack := func(region string) string {
		return fmt.Sprintf(`services:
  - name: %s
    image: nginx:1.25-alpine
    deploy:
      region: %s
      replicas: 1
    volume:
      mountPath: /data
`, svc, region)
	}
	cfg := writeStack(t, dir, stack(from))

	t.Cleanup(func() {
		env.Run("delete", "-f", cfg, "--yes")
	})

	// 1. Create the stack in region A and let the deployment settle — Railway
	//    fails a migration attempted before the service ever deployed.
	env.RunOK(t, "apply", "-f", cfg)
	if err := harness.WaitForVolume(env, svc+"-volume"); err != nil {
		t.Fatalf("declared volume did not appear: %v", err)
	}
	waitForDeploySuccess(t, env, svc, 3*time.Minute)

	// 2. Move to region B in the manifest: diff shows the change, apply is
	//    refused without --force. (REQ-VOL-101)
	cfg = writeStack(t, dir, stack(to))
	r := env.RunOK(t, "diff", "-f", cfg)
	harness.AssertContains(t, r.Stdout, "deploy.region")
	harness.AssertContains(t, r.Stdout, to)
	r = env.RunFail(t, "apply", "-f", cfg)
	harness.AssertContains(t, r.Stderr, "migrate")
	harness.AssertContains(t, r.Stderr, "--force")

	// 3. apply --force migrates; the volume lands in region B. Railway
	//    migrations fail transiently (the platform resets the region) — retry
	//    once, like an operator would.
	env.RunOK(t, "apply", "-f", cfg, "--force")
	if !waitVolumeRegion(t, env, svc+"-volume", to, 5*time.Minute) {
		t.Logf("volume still in %q — Railway reset the migration; retrying once", volumeRegion(t, env, svc+"-volume"))
		env.RunOK(t, "apply", "-f", cfg, "--force")
		if !waitVolumeRegion(t, env, svc+"-volume", to, 5*time.Minute) {
			t.Fatalf("volume did not migrate to %q after a retry (got %q)", to, volumeRegion(t, env, svc+"-volume"))
		}
	}

	// 4. diff must converge back to "No changes" — the migrated placement has
	//    to read back correctly. Railway's migration-initiated redeploy takes a
	//    while to surface in the project-level read, so poll with a deadline
	//    and log the convergence time (a permanent mismatch would mean railctl
	//    misreports drift after every migration).
	start := time.Now()
	deadline := time.Now().Add(8 * time.Minute)
	for {
		r = env.RunOK(t, "diff", "-f", cfg)
		if !strings.Contains(r.Stdout, "deploy.region") {
			harness.AssertContains(t, r.Stdout, "No changes")
			t.Logf("diff converged %s after migration", time.Since(start).Round(time.Second))
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("diff still reports a deploy.region change %s after the migration finished:\n%s",
				time.Since(start).Round(time.Second), r.Stdout)
		}
		time.Sleep(15 * time.Second)
	}

	// 5. Suite-3 rename probe: volume freshly migrated to another region.
	tryRenameVolume(t, env, svc+"-volume")

	// 6. Declarative teardown; clean room.
	r = env.RunOK(t, "delete", "-f", cfg, "--yes")
	harness.AssertContains(t, r.Stdout, "1 services deleted")
	harness.AssertContains(t, r.Stdout, "1 volumes deleted")
}
