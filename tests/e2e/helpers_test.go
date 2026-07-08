//go:build e2e

// Package e2e contains the legacy flat end-to-end tests for the railctl CLI.
//
// This file is a TRANSITIONAL SHIM: all real harness logic lives in
// tests/e2e/harness. It only re-exposes the old identifiers with their old
// signatures so the un-migrated flat tests keep compiling. As the flat tests
// migrate into tests/e2e/{account,workspace,project}, this shim shrinks and
// is finally deleted.
package e2e

import (
	"os"
	"testing"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// Env aliases harness.Env; methods (Run, RunOK, RunFail, With*, Teardown)
// carry over.
type Env = harness.Env

// Result aliases harness.Result.
type Result = harness.Result

// railctl mirrors the resolved binary path for tests that exec it directly.
var railctl = harness.Railctl

// pickTokenCompat resolves the credential for un-migrated flat tests:
// RAILWAY_WORKSPACE_TOKEN first, then RAILWAY_TOKEN.
func pickTokenCompat(t *testing.T) string {
	t.Helper()
	if tok := os.Getenv("RAILWAY_WORKSPACE_TOKEN"); tok != "" {
		t.Log("Using RAILWAY_WORKSPACE_TOKEN")
		return tok
	}
	if tok := os.Getenv("RAILWAY_TOKEN"); tok != "" {
		t.Log("Using RAILWAY_TOKEN (RAILWAY_WORKSPACE_TOKEN not set)")
		return tok
	}
	t.Fatal("No RAILWAY_WORKSPACE_TOKEN or RAILWAY_TOKEN set")
	return ""
}

func SetupProject(t *testing.T) *Env {
	t.Helper()
	return harness.SetupProject(t, pickTokenCompat(t))
}

func SetupEnvironment(t *testing.T) *Env {
	t.Helper()
	return harness.SetupEnvironment(t, pickTokenCompat(t))
}

func SetupService(t *testing.T) *Env {
	t.Helper()
	return harness.SetupService(t, pickTokenCompat(t))
}

func AssertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	harness.AssertContains(t, haystack, needle)
}

func AssertNotContains(t *testing.T, haystack, needle string) {
	t.Helper()
	harness.AssertNotContains(t, haystack, needle)
}

func AssertValidJSON(t *testing.T, s string) {
	t.Helper()
	harness.AssertValidJSON(t, s)
}

func AssertValidYAML(t *testing.T, s string) {
	t.Helper()
	harness.AssertValidYAML(t, s)
}
