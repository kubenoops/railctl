//go:build e2e

package project

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// The apply/diff tests run inside the shared fixture project under the
// minted project token: no -p/-e flags anywhere — the token carries the
// scope. Service names in the config files are unique per test (the fixture
// is shared across the whole group), and each test deletes the services it
// applied.

// TestApplyDiff_CreateFromFile tests the full apply lifecycle with a single config file.
//
//	go test -tags e2e -v -run TestApplyDiff_CreateFromFile ./tests/e2e/project/...
func TestApplyDiff_CreateFromFile(t *testing.T) {
	e := fixtureEnv(t)
	svcName := harness.UniqueName()
	t.Cleanup(func() {
		e.Run("delete", "service", svcName, "--yes")
	})

	// Write config file with one service.
	cfgDir := t.TempDir()
	cfgFile := filepath.Join(cfgDir, "svc.yaml")
	cfgData := fmt.Sprintf(`services:
  - name: %s
    image: nginx:1.25-alpine
    variables:
      PORT: "80"
`, svcName)
	if err := os.WriteFile(cfgFile, []byte(cfgData), 0644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	// 1. diff should show changes (exit non-zero because differences detected).
	r := e.Run("diff", "-f", cfgFile)
	if r.ExitCode == 0 {
		t.Fatalf("expected diff to exit non-zero when changes exist, got 0\nstdout: %s", r.Stdout)
	}
	harness.AssertContains(t, r.Stdout, "create")
	harness.AssertContains(t, r.Stdout, svcName)

	// 2. apply --dry-run should show diff but not create anything.
	e.RunOK(t, "apply", "-f", cfgFile, "--dry-run")

	// 3. Verify service does NOT exist after dry run.
	r = e.RunOK(t, "get", "services")
	harness.AssertNotContains(t, r.Stdout, svcName)

	// 4. Apply for real — should create the service.
	r = e.RunOK(t, "apply", "-f", cfgFile)
	harness.AssertContains(t, r.Stdout, "Created")

	time.Sleep(3 * time.Second)

	// 5. Verify service exists.
	r = e.RunOK(t, "get", "services")
	harness.AssertContains(t, r.Stdout, svcName)

	// 6. Verify variable was set.
	r = e.RunOK(t, "get", "variables", "-s", svcName)
	harness.AssertContains(t, r.Stdout, "PORT")
}

// TestApplyDiff_Idempotent tests that applying the same config twice produces no changes.
//
//	go test -tags e2e -v -run TestApplyDiff_Idempotent ./tests/e2e/project/...
func TestApplyDiff_Idempotent(t *testing.T) {
	e := fixtureEnv(t)
	svcName := harness.UniqueName()
	t.Cleanup(func() {
		e.Run("delete", "service", svcName, "--yes")
	})

	cfgDir := t.TempDir()
	cfgFile := filepath.Join(cfgDir, "svc.yaml")
	cfgData := fmt.Sprintf(`services:
  - name: %s
    image: nginx:1.25-alpine
    variables:
      PORT: "80"
`, svcName)
	if err := os.WriteFile(cfgFile, []byte(cfgData), 0644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	// First apply — creates the service.
	e.RunOK(t, "apply", "-f", cfgFile)
	time.Sleep(3 * time.Second)

	// diff should show no changes (exit 0).
	r := e.RunOK(t, "diff", "-f", cfgFile)
	harness.AssertContains(t, r.Stdout, "No changes")

	// Second apply — should say no changes needed.
	r = e.RunOK(t, "apply", "-f", cfgFile)
	harness.AssertContains(t, r.Stdout, "No changes")
}

// TestApplyDiff_UpdateService tests updating a service via apply.
//
//	go test -tags e2e -v -run TestApplyDiff_UpdateService ./tests/e2e/project/...
func TestApplyDiff_UpdateService(t *testing.T) {
	e := fixtureEnv(t)
	svcName := harness.UniqueName()
	t.Cleanup(func() {
		e.Run("delete", "service", svcName, "--yes")
	})

	cfgDir := t.TempDir()
	cfgFile := filepath.Join(cfgDir, "svc.yaml")

	// Initial config.
	cfgData := fmt.Sprintf(`services:
  - name: %s
    image: nginx:1.25-alpine
    variables:
      PORT: "80"
`, svcName)
	if err := os.WriteFile(cfgFile, []byte(cfgData), 0644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	// Apply initial config.
	e.RunOK(t, "apply", "-f", cfgFile)
	time.Sleep(3 * time.Second)

	// Update config: change image and add a variable.
	cfgDataUpdated := fmt.Sprintf(`services:
  - name: %s
    image: nginx:1.27-alpine
    variables:
      PORT: "80"
      UPDATED: "true"
`, svcName)
	if err := os.WriteFile(cfgFile, []byte(cfgDataUpdated), 0644); err != nil {
		t.Fatalf("writing updated config file: %v", err)
	}

	// diff should show image change and new variable.
	r := e.Run("diff", "-f", cfgFile)
	if r.ExitCode == 0 {
		t.Fatalf("expected diff to exit non-zero when changes exist, got 0\nstdout: %s", r.Stdout)
	}
	harness.AssertContains(t, r.Stdout, "nginx:1.25-alpine")
	harness.AssertContains(t, r.Stdout, "nginx:1.27-alpine")
	harness.AssertContains(t, r.Stdout, "UPDATED")

	// Apply the update.
	r = e.RunOK(t, "apply", "-f", cfgFile)
	harness.AssertContains(t, r.Stdout, "Updated")
}

// TestApplyDiff_DryRunNoSideEffects tests that --dry-run truly doesn't modify anything.
//
//	go test -tags e2e -v -run TestApplyDiff_DryRunNoSideEffects ./tests/e2e/project/...
func TestApplyDiff_DryRunNoSideEffects(t *testing.T) {
	e := fixtureEnv(t)
	svcName := harness.UniqueName()

	cfgDir := t.TempDir()
	cfgFile := filepath.Join(cfgDir, "svc.yaml")
	cfgData := fmt.Sprintf(`services:
  - name: %s
    image: nginx:1.25-alpine
`, svcName)
	if err := os.WriteFile(cfgFile, []byte(cfgData), 0644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	// Apply with --dry-run.
	e.RunOK(t, "apply", "-f", cfgFile, "--dry-run")

	// Verify service does NOT exist.
	r := e.RunOK(t, "get", "services")
	harness.AssertNotContains(t, r.Stdout, svcName)
}

// TestApplyDiff_LegacyFormat tests backward compatibility with old single-service config format.
//
//	go test -tags e2e -v -run TestApplyDiff_LegacyFormat ./tests/e2e/project/...
func TestApplyDiff_LegacyFormat(t *testing.T) {
	e := fixtureEnv(t)
	svcName := harness.UniqueName()
	t.Cleanup(func() {
		e.Run("delete", "service", svcName, "--yes")
	})

	cfgDir := t.TempDir()
	cfgFile := filepath.Join(cfgDir, "legacy.yaml")
	cfgData := fmt.Sprintf(`service:
  name: %s
  image: nginx:1.25-alpine
deploy:
  restartPolicyType: ON_FAILURE
  restartPolicyMaxRetries: 3
variables:
  PORT: "80"
`, svcName)
	if err := os.WriteFile(cfgFile, []byte(cfgData), 0644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	// Apply should work with legacy format (auto-convert).
	r := e.RunOK(t, "apply", "-f", cfgFile)
	harness.AssertContains(t, r.Stdout, "Created")

	time.Sleep(3 * time.Second)

	// Verify service was created.
	r = e.RunOK(t, "get", "services")
	harness.AssertContains(t, r.Stdout, svcName)
}

// TestApplyDelete_Roundtrip tests the declarative teardown counterpart of
// apply: apply a two-service config, then `delete -f` the same config and
// verify the services are gone and diff reports creates again.
//
//	go test -tags e2e -v -run TestApplyDelete_Roundtrip ./tests/e2e/project/...
func TestApplyDelete_Roundtrip(t *testing.T) {
	e := fixtureEnv(t)
	svcA := harness.UniqueName()
	svcB := harness.UniqueName()
	t.Cleanup(func() {
		// Belt-and-braces: the declarative delete below should already have
		// removed both.
		e.Run("delete", "service", svcA, "--yes")
		e.Run("delete", "service", svcB, "--yes")
	})

	cfgDir := t.TempDir()
	cfgFile := filepath.Join(cfgDir, "stack.yaml")
	cfgData := fmt.Sprintf(`services:
  - name: %s
    image: nginx:1.25-alpine
  - name: %s
    image: redis:7-alpine
`, svcA, svcB)
	if err := os.WriteFile(cfgFile, []byte(cfgData), 0644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	// Apply — creates both services.
	r := e.RunOK(t, "apply", "-f", cfgFile)
	harness.AssertContains(t, r.Stdout, "Created")

	time.Sleep(3 * time.Second)

	// Verify both exist.
	r = e.RunOK(t, "get", "services")
	harness.AssertContains(t, r.Stdout, svcA)
	harness.AssertContains(t, r.Stdout, svcB)

	// Declarative delete of the same config.
	r = e.RunOK(t, "delete", "-f", cfgFile, "--yes")
	harness.AssertContains(t, r.Stdout, "2 services deleted")

	time.Sleep(3 * time.Second)

	// Verify both are gone.
	r = e.RunOK(t, "get", "services")
	harness.AssertNotContains(t, r.Stdout, svcA)
	harness.AssertNotContains(t, r.Stdout, svcB)

	// diff against the same config shows creates again (exit non-zero).
	r = e.Run("diff", "-f", cfgFile)
	if r.ExitCode == 0 {
		t.Fatalf("expected diff to exit non-zero after delete -f, got 0\nstdout: %s", r.Stdout)
	}
	harness.AssertContains(t, r.Stdout, "create")
	harness.AssertContains(t, r.Stdout, svcA)
	harness.AssertContains(t, r.Stdout, svcB)
}

// TestApplyDiff_Directory tests applying from a directory of configs.
//
//	go test -tags e2e -v -run TestApplyDiff_Directory ./tests/e2e/project/...
func TestApplyDiff_Directory(t *testing.T) {
	e := fixtureEnv(t)
	svcA := harness.UniqueName()
	svcB := harness.UniqueName()
	t.Cleanup(func() {
		e.Run("delete", "service", svcA, "--yes")
		e.Run("delete", "service", svcB, "--yes")
	})

	cfgDir := filepath.Join(t.TempDir(), "configs")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatalf("creating config directory: %v", err)
	}

	// First config file.
	cfgA := fmt.Sprintf(`services:
  - name: %s
    image: nginx:1.25-alpine
`, svcA)
	if err := os.WriteFile(filepath.Join(cfgDir, "01-svc-a.yaml"), []byte(cfgA), 0644); err != nil {
		t.Fatalf("writing config file A: %v", err)
	}

	// Second config file.
	cfgB := fmt.Sprintf(`services:
  - name: %s
    image: redis:7-alpine
`, svcB)
	if err := os.WriteFile(filepath.Join(cfgDir, "02-svc-b.yaml"), []byte(cfgB), 0644); err != nil {
		t.Fatalf("writing config file B: %v", err)
	}

	// Apply from directory — should create both services.
	r := e.RunOK(t, "apply", "-f", cfgDir)
	harness.AssertContains(t, r.Stdout, "Created")

	time.Sleep(3 * time.Second)

	// Verify both services exist.
	r = e.RunOK(t, "get", "services")
	harness.AssertContains(t, r.Stdout, svcA)
	harness.AssertContains(t, r.Stdout, svcB)
}
