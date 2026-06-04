//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestApplyDiff_CreateFromFile tests the full apply lifecycle with a single config file.
// Setup: creates project + environment
//
//	go test -tags e2e -v -run TestApplyDiff_CreateFromFile ./tests/e2e/...
func TestApplyDiff_CreateFromFile(t *testing.T) {
	e := SetupEnvironment(t)

	// Write config file with one service.
	cfgDir := t.TempDir()
	cfgFile := filepath.Join(cfgDir, "svc.yaml")
	cfgData := `services:
  - name: apply-test-svc
    image: nginx:1.25-alpine
    variables:
      PORT: "80"
`
	if err := os.WriteFile(cfgFile, []byte(cfgData), 0644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	// 1. diff should show changes (exit non-zero because differences detected).
	r := e.Run(e.WithPE("diff", "-f", cfgFile)...)
	if r.ExitCode == 0 {
		t.Fatalf("expected diff to exit non-zero when changes exist, got 0\nstdout: %s", r.Stdout)
	}
	AssertContains(t, r.Stdout, "create")
	AssertContains(t, r.Stdout, "apply-test-svc")

	// 2. apply --dry-run should show diff but not create anything.
	e.RunOK(t, e.WithPE("apply", "-f", cfgFile, "--dry-run")...)

	// 3. Verify service does NOT exist after dry run.
	r = e.RunOK(t, e.WithPE("get", "services")...)
	AssertNotContains(t, r.Stdout, "apply-test-svc")

	// 4. Apply for real — should create the service.
	r = e.RunOK(t, e.WithPE("apply", "-f", cfgFile)...)
	AssertContains(t, r.Stdout, "Created")

	time.Sleep(3 * time.Second)

	// 5. Verify service exists.
	r = e.RunOK(t, e.WithPE("get", "services")...)
	AssertContains(t, r.Stdout, "apply-test-svc")

	// 6. Verify variable was set.
	r = e.RunOK(t, e.WithPE("get", "variables", "-s", "apply-test-svc")...)
	AssertContains(t, r.Stdout, "PORT")
}

// TestApplyDiff_Idempotent tests that applying the same config twice produces no changes.
// Setup: creates project + environment
//
//	go test -tags e2e -v -run TestApplyDiff_Idempotent ./tests/e2e/...
func TestApplyDiff_Idempotent(t *testing.T) {
	e := SetupEnvironment(t)

	cfgDir := t.TempDir()
	cfgFile := filepath.Join(cfgDir, "svc.yaml")
	cfgData := `services:
  - name: idempotent-svc
    image: nginx:1.25-alpine
    variables:
      PORT: "80"
`
	if err := os.WriteFile(cfgFile, []byte(cfgData), 0644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	// First apply — creates the service.
	e.RunOK(t, e.WithPE("apply", "-f", cfgFile)...)
	time.Sleep(3 * time.Second)

	// diff should show no changes (exit 0).
	r := e.RunOK(t, e.WithPE("diff", "-f", cfgFile)...)
	AssertContains(t, r.Stdout, "No changes")

	// Second apply — should say no changes needed.
	r = e.RunOK(t, e.WithPE("apply", "-f", cfgFile)...)
	AssertContains(t, r.Stdout, "No changes")
}

// TestApplyDiff_UpdateService tests updating a service via apply.
// Setup: creates project + environment
//
//	go test -tags e2e -v -run TestApplyDiff_UpdateService ./tests/e2e/...
func TestApplyDiff_UpdateService(t *testing.T) {
	e := SetupEnvironment(t)

	cfgDir := t.TempDir()
	cfgFile := filepath.Join(cfgDir, "svc.yaml")

	// Initial config.
	cfgData := `services:
  - name: update-svc
    image: nginx:1.25-alpine
    variables:
      PORT: "80"
`
	if err := os.WriteFile(cfgFile, []byte(cfgData), 0644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	// Apply initial config.
	e.RunOK(t, e.WithPE("apply", "-f", cfgFile)...)
	time.Sleep(3 * time.Second)

	// Update config: change image and add a variable.
	cfgDataUpdated := `services:
  - name: update-svc
    image: nginx:1.27-alpine
    variables:
      PORT: "80"
      UPDATED: "true"
`
	if err := os.WriteFile(cfgFile, []byte(cfgDataUpdated), 0644); err != nil {
		t.Fatalf("writing updated config file: %v", err)
	}

	// diff should show image change and new variable.
	r := e.Run(e.WithPE("diff", "-f", cfgFile)...)
	if r.ExitCode == 0 {
		t.Fatalf("expected diff to exit non-zero when changes exist, got 0\nstdout: %s", r.Stdout)
	}
	AssertContains(t, r.Stdout, "nginx:1.25-alpine")
	AssertContains(t, r.Stdout, "nginx:1.27-alpine")
	AssertContains(t, r.Stdout, "UPDATED")

	// Apply the update.
	r = e.RunOK(t, e.WithPE("apply", "-f", cfgFile)...)
	AssertContains(t, r.Stdout, "Updated")
}

// TestApplyDiff_DryRunNoSideEffects tests that --dry-run truly doesn't modify anything.
// Setup: creates project + environment
//
//	go test -tags e2e -v -run TestApplyDiff_DryRunNoSideEffects ./tests/e2e/...
func TestApplyDiff_DryRunNoSideEffects(t *testing.T) {
	e := SetupEnvironment(t)

	cfgDir := t.TempDir()
	cfgFile := filepath.Join(cfgDir, "svc.yaml")
	cfgData := `services:
  - name: dryrun-svc
    image: nginx:1.25-alpine
`
	if err := os.WriteFile(cfgFile, []byte(cfgData), 0644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	// Apply with --dry-run.
	e.RunOK(t, e.WithPE("apply", "-f", cfgFile, "--dry-run")...)

	// Verify service does NOT exist.
	r := e.RunOK(t, e.WithPE("get", "services")...)
	AssertNotContains(t, r.Stdout, "dryrun-svc")
}

// TestApplyDiff_LegacyFormat tests backward compatibility with old single-service config format.
// Setup: creates project + environment
//
//	go test -tags e2e -v -run TestApplyDiff_LegacyFormat ./tests/e2e/...
func TestApplyDiff_LegacyFormat(t *testing.T) {
	e := SetupEnvironment(t)

	cfgDir := t.TempDir()
	cfgFile := filepath.Join(cfgDir, "legacy.yaml")
	cfgData := `service:
  name: legacy-svc
  image: nginx:1.25-alpine
deploy:
  restartPolicyType: ON_FAILURE
  restartPolicyMaxRetries: 3
variables:
  PORT: "80"
`
	if err := os.WriteFile(cfgFile, []byte(cfgData), 0644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	// Apply should work with legacy format (auto-convert).
	r := e.RunOK(t, e.WithPE("apply", "-f", cfgFile)...)
	AssertContains(t, r.Stdout, "Created")

	time.Sleep(3 * time.Second)

	// Verify service was created.
	r = e.RunOK(t, e.WithPE("get", "services")...)
	AssertContains(t, r.Stdout, "legacy-svc")
}

// TestApplyDiff_Directory tests applying from a directory of configs.
// Setup: creates project + environment
//
//	go test -tags e2e -v -run TestApplyDiff_Directory ./tests/e2e/...
func TestApplyDiff_Directory(t *testing.T) {
	e := SetupEnvironment(t)

	cfgDir := filepath.Join(t.TempDir(), "configs")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatalf("creating config directory: %v", err)
	}

	// First config file.
	cfgA := `services:
  - name: svc-a
    image: nginx:1.25-alpine
`
	if err := os.WriteFile(filepath.Join(cfgDir, "01-svc-a.yaml"), []byte(cfgA), 0644); err != nil {
		t.Fatalf("writing config file A: %v", err)
	}

	// Second config file.
	cfgB := `services:
  - name: svc-b
    image: redis:7-alpine
`
	if err := os.WriteFile(filepath.Join(cfgDir, "02-svc-b.yaml"), []byte(cfgB), 0644); err != nil {
		t.Fatalf("writing config file B: %v", err)
	}

	// Apply from directory — should create both services.
	r := e.RunOK(t, e.WithPE("apply", "-f", cfgDir)...)
	AssertContains(t, r.Stdout, "Created")

	time.Sleep(3 * time.Second)

	// Verify both services exist.
	r = e.RunOK(t, e.WithPE("get", "services")...)
	AssertContains(t, r.Stdout, "svc-a")
	AssertContains(t, r.Stdout, "svc-b")
}
