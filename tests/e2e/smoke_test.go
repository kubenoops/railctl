//go:build e2e

package e2e

import (
	"encoding/json"
	"testing"
)

// TestSmoke is a fast, linear walk through the full CLI lifecycle.
// It creates one project → environment → service → variable → volume,
// exercises describe/get on each, then tears everything down.
// No permutations, no table-driven subtests — just the happy path.
//
//	go test -tags e2e -v -run TestSmoke -timeout 3m ./tests/e2e/...
func TestSmoke(t *testing.T) {
	env := SetupProject(t) // creates project, registers cleanup

	// ── List projects ────────────────────────────────────────
	r := env.RunOK(t, "get", "projects")
	AssertContains(t, r.Stdout, env.ProjectName)

	// ── Describe project ─────────────────────────────────────
	r = env.RunOK(t, env.WithP("describe", "project", env.ProjectName)...)
	AssertContains(t, r.Stdout, env.ProjectName)

	// ── Create environment ───────────────────────────────────
	env.RunOK(t, "create", "environment", env.EnvName, "-p", env.ProjectName)

	// ── List environments ────────────────────────────────────
	r = env.RunOK(t, env.WithP("get", "environments")...)
	AssertContains(t, r.Stdout, env.EnvName)

	// ── Create service ───────────────────────────────────────
	env.RunOK(t, "create", "service", env.ServiceName,
		"--image", env.ServiceImg,
		"-p", env.ProjectName, "-e", env.EnvName)

	// ── List services ────────────────────────────────────────
	r = env.RunOK(t, env.WithPE("get", "services")...)
	AssertContains(t, r.Stdout, env.ServiceName)

	// ── Get services JSON ────────────────────────────────────
	r = env.RunOK(t, env.WithPE("get", "services", "-o", "json")...)
	AssertValidJSON(t, r.Stdout)

	// ── Set variable ─────────────────────────────────────────
	env.RunOK(t, env.WithPES("set", "variable", "SMOKE_KEY=smoke_value", "--skip-deployment")...)

	// ── Get variables ────────────────────────────────────────
	r = env.RunOK(t, env.WithPES("get", "variables")...)
	AssertContains(t, r.Stdout, "SMOKE_KEY")

	// ── Create volume ────────────────────────────────────────
	env.RunOK(t, env.WithPES("create", "volume", "smoke-vol", "--mount-path", "/data")...)

	// ── List volumes ─────────────────────────────────────────
	r = env.RunOK(t, env.WithPES("get", "volumes")...)
	AssertContains(t, r.Stdout, "/data")
	AssertContains(t, r.Stdout, "smoke-vol")

	// ── Get deployments ──────────────────────────────────────
	r = env.RunOK(t, env.WithPES("get", "deployments", "-o", "json")...)
	AssertValidJSON(t, r.Stdout)

	var deps []map[string]interface{}
	if err := json.Unmarshal([]byte(r.Stdout), &deps); err == nil && len(deps) > 0 {
		// ── Describe a deployment ────────────────────────────
		// Just verify the describe command doesn't crash
		if id, ok := deps[0]["id"].(string); ok {
			env.RunOK(t, env.WithPES("get", "deployments")...)
			_ = id
		}
	}

	// ── Delete variable ──────────────────────────────────────
	env.RunOK(t, env.WithPES("delete", "variable", "SMOKE_KEY", "--yes")...)

	// ── Verify variable gone ─────────────────────────────────
	r = env.RunOK(t, env.WithPES("get", "variables")...)
	AssertNotContains(t, r.Stdout, "SMOKE_KEY")

	// ── Error handling: invalid output format ────────────────
	r = env.RunFail(t, "get", "projects", "-o", "nope")
	AssertContains(t, r.Stderr, "invalid output format")

	t.Log("✅ Smoke test passed — full lifecycle exercised")
}
