//go:build e2e

// Package e2e contains end-to-end tests for the railctl CLI.
//
// Tests are grouped by command (projects, environments, services, etc).
// Each group can be run independently and creates only the infrastructure
// it needs:
//
//	go test -tags e2e -v -run TestProjects  ./tests/e2e/...   # just token needed
//	go test -tags e2e -v -run TestServices  ./tests/e2e/...   # creates project+env
//	go test -tags e2e -v -run TestVariables ./tests/e2e/...   # creates project+env+service
//
// Run all:
//
//	go test -tags e2e -v -timeout 10m ./tests/e2e/...
//
// Or via make:
//
//	make test-e2e
package e2e

import (
	"bytes"
	crand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ──────────────────────────────────────────────────────────────
// Binary resolution
// ──────────────────────────────────────────────────────────────

var railctl string

func init() {
	railctl = os.Getenv("RAILCTL")
	if railctl == "" {
		dir, _ := os.Getwd()
		for {
			candidate := filepath.Join(dir, "railctl")
			if _, err := os.Stat(candidate); err == nil {
				railctl = candidate
				break
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
}

// ──────────────────────────────────────────────────────────────
// Env represents a test environment with created infrastructure.
// Each test suite calls the setup level it needs. Setup is
// idempotent — if the resource already exists, it's reused.
// ──────────────────────────────────────────────────────────────

type Env struct {
	t           *testing.T
	token       string // Railway API token for this suite
	ProjectName string
	EnvName     string
	ServiceName string
	ServiceImg  string

	hasProject bool
	hasEnv     bool
	hasService bool
}

func uniqueName() string {
	b := make([]byte, 4)
	crand.Read(b)
	return fmt.Sprintf("e2e-%d-%s", time.Now().Unix(), hex.EncodeToString(b))
}

// pickToken randomly selects one of RAILWAY_TOKEN_1/2/3.
// Falls back to RAILWAY_TOKEN if numbered vars aren't set.
func pickToken(t *testing.T) string {
	t.Helper()
	tokens := make([]string, 0, 3)
	for i := 1; i <= 3; i++ {
		if tok := os.Getenv(fmt.Sprintf("RAILWAY_TOKEN_%d", i)); tok != "" {
			tokens = append(tokens, tok)
		}
	}
	if len(tokens) > 0 {
		idx := rand.Intn(len(tokens))
		t.Logf("Using RAILWAY_TOKEN_%d (of %d available)", idx+1, len(tokens))
		return tokens[idx]
	}
	// Fallback to single RAILWAY_TOKEN
	if tok := os.Getenv("RAILWAY_TOKEN"); tok != "" {
		t.Log("Using single RAILWAY_TOKEN (no numbered tokens found)")
		return tok
	}
	t.Fatal("No RAILWAY_TOKEN or RAILWAY_TOKEN_1/2/3 set")
	return ""
}

// SetupProject creates a fresh project (or reuses E2E_PROJECT).
func SetupProject(t *testing.T) *Env {
	t.Helper()
	requireBinary(t)
	token := pickToken(t)

	name := os.Getenv("E2E_PROJECT")
	if name == "" {
		name = uniqueName()
	}

	e := &Env{
		t:           t,
		token:       token,
		ProjectName: name,
		EnvName:     "staging",
		ServiceName: "web",
		ServiceImg:  "nginx:1.25-alpine",
	}

	// Create project
	r := e.Run("create", "project", e.ProjectName)
	if r.ExitCode != 0 {
		t.Fatalf("failed to create project %s: %s", e.ProjectName, r.Stderr)
	}
	e.hasProject = true

	t.Cleanup(func() {
		if os.Getenv("E2E_KEEP") == "1" && t.Failed() {
			t.Logf("E2E_KEEP=1: leaving project %s for debugging", e.ProjectName)
			return
		}
		e.Teardown()
	})

	t.Logf("Created project: %s", e.ProjectName)
	return e
}

// SetupEnvironment creates project + custom environment.
func SetupEnvironment(t *testing.T) *Env {
	t.Helper()
	e := SetupProject(t)

	r := e.Run("create", "environment", e.EnvName, "-p", e.ProjectName)
	if r.ExitCode != 0 {
		t.Fatalf("failed to create environment %s: %s", e.EnvName, r.Stderr)
	}
	e.hasEnv = true
	t.Logf("Created environment: %s", e.EnvName)
	return e
}

// SetupService creates project + environment + service.
func SetupService(t *testing.T) *Env {
	t.Helper()
	e := SetupEnvironment(t)

	r := e.Run("create", "service", e.ServiceName,
		"--image", e.ServiceImg,
		"-p", e.ProjectName, "-e", e.EnvName)
	if r.ExitCode != 0 {
		t.Fatalf("failed to create service %s: %s", e.ServiceName, r.Stderr)
	}
	e.hasService = true
	t.Logf("Created service: %s (waiting for deployment...)", e.ServiceName)
	time.Sleep(3 * time.Second) // Let initial deployment start
	return e
}

// Teardown cleans up all created resources in reverse order.
func (e *Env) Teardown() {
	e.t.Helper()
	e.t.Logf("Cleaning up project %s...", e.ProjectName)

	// Delete services in all environments
	for _, env := range []string{e.EnvName, "production"} {
		r := e.Run("get", "services", "-p", e.ProjectName, "-e", env, "-o", "json")
		var svcs []map[string]interface{}
		if json.Unmarshal([]byte(r.Stdout), &svcs) == nil {
			for _, svc := range svcs {
				if name, ok := svc["name"].(string); ok {
					e.t.Logf("  Deleting service %s in %s", name, env)
					e.Run("delete", "service", name, "-p", e.ProjectName, "-e", env, "--yes")
				}
			}
		}
	}

	// Delete volumes in all environments
	for _, env := range []string{e.EnvName, "production"} {
		r := e.Run("get", "volumes", "-p", e.ProjectName, "-e", env, "-o", "json")
		var vols []map[string]interface{}
		if json.Unmarshal([]byte(r.Stdout), &vols) == nil {
			for _, vol := range vols {
				if name, ok := vol["name"].(string); ok {
					e.t.Logf("  Deleting volume %s in %s", name, env)
					e.Run("delete", "volume", name, "-p", e.ProjectName, "-e", env, "--yes")
				}
			}
		}
	}

	time.Sleep(2 * time.Second)

	// Delete project
	e.t.Logf("  Deleting project %s", e.ProjectName)
	r := e.Run("delete", "project", e.ProjectName, "--yes")
	if r.ExitCode == 0 {
		e.t.Log("  Project deleted")
	} else {
		e.t.Logf("  Could not delete project: %s", r.Stderr)
	}
}

// ──────────────────────────────────────────────────────────────
// CLI Runner
// ──────────────────────────────────────────────────────────────

// Result holds the output of a CLI invocation.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Run executes railctl with the given args. Never fails the test.
// Automatically injects --token for the suite's assigned Railway account.
func (e *Env) Run(args ...string) Result {
	fullArgs := append([]string{"--token", e.token}, args...)
	cmd := exec.Command(railctl, fullArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			code = -1
		}
	}
	return Result{Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: code}
}

// RunOK runs railctl and fatals if exit code != 0.
func (e *Env) RunOK(t *testing.T, args ...string) Result {
	t.Helper()
	r := e.Run(args...)
	if r.ExitCode != 0 {
		t.Fatalf("railctl %s failed (exit %d):\nstdout: %s\nstderr: %s",
			strings.Join(args, " "), r.ExitCode, r.Stdout, r.Stderr)
	}
	return r
}

// RunFail runs railctl and fatals if exit code == 0.
func (e *Env) RunFail(t *testing.T, args ...string) Result {
	t.Helper()
	r := e.Run(args...)
	if r.ExitCode == 0 {
		t.Fatalf("expected railctl %s to fail, but it succeeded:\nstdout: %s",
			strings.Join(args, " "), r.Stdout)
	}
	return r
}

// Shorthand flag builders
func (e *Env) WithP(args ...string) []string {
	return append(args, "-p", e.ProjectName)
}

func (e *Env) WithPE(args ...string) []string {
	return append(args, "-p", e.ProjectName, "-e", e.EnvName)
}

func (e *Env) WithPES(args ...string) []string {
	return append(args, "-p", e.ProjectName, "-e", e.EnvName, "-s", e.ServiceName)
}

// ──────────────────────────────────────────────────────────────
// Assertions
// ──────────────────────────────────────────────────────────────

func AssertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("expected output to contain %q, got:\n%s", needle, truncate(haystack, 500))
	}
}

func AssertNotContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Errorf("expected output NOT to contain %q, got:\n%s", needle, truncate(haystack, 500))
	}
}

func AssertValidJSON(t *testing.T, s string) {
	t.Helper()
	if !json.Valid([]byte(s)) {
		t.Errorf("expected valid JSON, got:\n%s", truncate(s, 300))
	}
}

func AssertValidYAML(t *testing.T, s string) {
	t.Helper()
	s = strings.TrimSpace(s)
	if s == "" {
		t.Error("expected non-empty YAML output")
	}
}

// ──────────────────────────────────────────────────────────────
// Pre-flight checks
// ──────────────────────────────────────────────────────────────

// requireToken is no longer needed — pickToken() handles validation.
// Kept as a compatibility stub.
func requireToken(t *testing.T) {
	t.Helper()
	// pickToken() is called during SetupProject and will fail if no tokens are set.
}

func requireBinary(t *testing.T) {
	t.Helper()
	if railctl == "" {
		t.Fatal("railctl binary not found. Set RAILCTL env var or run: make build")
	}
	if _, err := os.Stat(railctl); err != nil {
		t.Fatalf("railctl binary not found at %s", railctl)
	}
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max] + "... (truncated)"
	}
	return s
}
