//go:build e2e

// Package harness is the shared end-to-end test harness for the railctl CLI.
//
// It is a regular (non-test) package so that every e2e test group
// (tests/e2e/account, tests/e2e/workspace, tests/e2e/project) can share the
// same Env runner, fixtures, assertions, and token preflight. All files carry
// the e2e build tag: the package only exists when tests are built with
// `-tags e2e`.
package harness

import (
	"bytes"
	"context"
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

// Railctl is the path to the railctl binary under test. It is resolved once
// at package init: the RAILCTL env var wins, otherwise the working directory
// is walked upward until a `railctl` binary is found.
var Railctl string

func init() {
	Railctl = os.Getenv("RAILCTL")
	if Railctl == "" {
		dir, _ := os.Getwd()
		for {
			candidate := filepath.Join(dir, "railctl")
			if _, err := os.Stat(candidate); err == nil {
				Railctl = candidate
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

// RequireBinary fatals the test if the railctl binary could not be resolved.
func RequireBinary(t *testing.T) {
	t.Helper()
	if Railctl == "" {
		t.Fatal("railctl binary not found. Set RAILCTL env var or run: make build")
	}
	if _, err := os.Stat(Railctl); err != nil {
		t.Fatalf("railctl binary not found at %s", Railctl)
	}
}

// ──────────────────────────────────────────────────────────────
// Env represents a test environment with created infrastructure.
// Each test suite calls the setup level it needs. Setup is
// idempotent — if the resource already exists, it's reused.
// ──────────────────────────────────────────────────────────────

// Env is a handle on the infrastructure a test (or test group) runs against,
// bound to the Railway API token that operates on it.
type Env struct {
	T           *testing.T
	Token       string // Railway API token for this suite
	ProjectName string
	EnvName     string
	ServiceName string
	ServiceImg  string

	hasProject bool
	hasEnv     bool
	hasService bool
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
// Automatically injects --token for the suite's assigned Railway credential.
// Each command has a 3-minute timeout to prevent a single stuck command
// from killing the entire test suite.
func (e *Env) Run(args ...string) Result {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	fullArgs := append([]string{"--token", e.Token}, args...)
	cmd := exec.CommandContext(ctx, Railctl, fullArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	code := 0
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			code = -1
			stderr.WriteString("\n[TIMEOUT] command exceeded 3-minute deadline")
		} else if exitErr, ok := err.(*exec.ExitError); ok {
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

// WithP appends -p <project> to args.
func (e *Env) WithP(args ...string) []string {
	return append(args, "-p", e.ProjectName)
}

// WithPE appends -p <project> -e <environment> to args.
func (e *Env) WithPE(args ...string) []string {
	return append(args, "-p", e.ProjectName, "-e", e.EnvName)
}

// WithPES appends -p <project> -e <environment> -s <service> to args.
func (e *Env) WithPES(args ...string) []string {
	return append(args, "-p", e.ProjectName, "-e", e.EnvName, "-s", e.ServiceName)
}
