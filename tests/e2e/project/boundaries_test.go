//go:build e2e

package project

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// TestBoundaries proves the project-token fail-fasts and guardrails: project
// enumeration is denied, -p/-e/-w contradictions fail fast (matching values
// proceed silently), and the token can self-mint sibling tokens within its
// own scope.
//
//	go test -tags e2e -v -run TestBoundaries ./tests/e2e/project/...
func TestBoundaries(t *testing.T) {
	env := fixtureEnv(t)

	t.Run("get_projects_denied", func(t *testing.T) {
		// internal/cmdutil/guard.go RequireWorkspaceScope (wired in
		// internal/cmd/get_projects.go): "cannot list projects with a
		// project token — it is scoped to a single project and environment;
		// use an account or workspace token"
		r := env.RunFail(t, "get", "projects")
		harness.AssertContains(t, r.Stdout+r.Stderr, "scoped to a single project")
	})

	t.Run("p_flag_mismatch_fails", func(t *testing.T) {
		// internal/cmdutil/context.go: a -p value naming a different project
		// than the token's baked scope is a contradiction and fails fast —
		// never warn-and-proceed on the token's own project.
		r := env.RunFail(t, "get", "services", "-p", "some-other-project")
		harness.AssertContains(t, r.Stdout+r.Stderr, "scoped to project")
	})

	t.Run("p_flag_match_ok", func(t *testing.T) {
		// A -p value naming the token's OWN project is consistent: the
		// command proceeds silently, without any "ignored" warning.
		r := env.RunOK(t, "get", "services", "-p", env.ProjectName)
		harness.AssertNotContains(t, r.Stderr, "ignored")
	})

	t.Run("e_flag_mismatch_fails", func(t *testing.T) {
		// Same contradiction fail-fast for -e against the token's baked
		// environment. Fires because `get services` sets NeedEnvironment.
		r := env.RunFail(t, "get", "services", "-e", "some-other-env")
		harness.AssertContains(t, r.Stdout+r.Stderr, "scoped to environment")
	})

	t.Run("e_flag_match_ok", func(t *testing.T) {
		// -e naming the token's own environment proceeds silently.
		r := env.RunOK(t, "get", "services", "-e", fixtureEnvName)
		harness.AssertNotContains(t, r.Stderr, "ignored")
	})

	t.Run("create_environment_denied", func(t *testing.T) {
		// Environment lifecycle is workspace-scope: the RequireWorkspaceScope
		// guard fails fast before any API mutation.
		r := env.RunFail(t, "create", "environment", "should-not-exist")
		harness.AssertContains(t, r.Stdout+r.Stderr, "scoped to a single project")
	})

	t.Run("delete_project_denied", func(t *testing.T) {
		// Project lifecycle is workspace-scope: guard fails fast, and the
		// fixture project must remain untouched (asserted implicitly — every
		// later test still runs against it).
		r := env.RunFail(t, "delete", "project", env.ProjectName, "--yes")
		harness.AssertContains(t, r.Stdout+r.Stderr, "scoped to a single project")
	})

	t.Run("w_flag_mismatch_fails", func(t *testing.T) {
		// internal/api/client.go GetProjectContext → checkWorkspaceHint: a
		// -w value naming a different workspace than the one containing the
		// token's project is a contradiction and fails fast. (The match case
		// is unit-tested; the test cannot know the real workspace name
		// statically.)
		r := env.RunFail(t, "get", "services", "-w", "some-other-workspace")
		harness.AssertContains(t, r.Stdout+r.Stderr, "scoped to workspace")
	})

	t.Run("self_mint", func(t *testing.T) {
		name := harness.UniqueName()

		// Mint with NO flags: the project token scopes the new token to its
		// own project + environment.
		r := env.RunOK(t, "token", "create", name)
		minted := strings.TrimSpace(r.Stdout)
		if len(minted) != 36 {
			t.Errorf("expected stdout to be exactly a 36-char token value, got %d chars:\nstdout: %q",
				len(minted), r.Stdout)
		}

		// The minted sibling shows up in the (flag-free) listing.
		r = env.RunOK(t, "token", "list")
		harness.AssertContains(t, r.Stdout, name)

		// Resolve the minted token's id from the JSON listing and revoke it.
		r = env.RunOK(t, "token", "list", "-o", "json")
		harness.AssertValidJSON(t, r.Stdout)
		var listed []struct {
			Name string `json:"name"`
			ID   string `json:"id"`
		}
		if err := json.Unmarshal([]byte(r.Stdout), &listed); err != nil {
			t.Fatalf("failed to unmarshal token list JSON: %v\nstdout: %s", err, r.Stdout)
		}
		var id string
		for _, tk := range listed {
			if tk.Name == name {
				id = tk.ID
				break
			}
		}
		if id == "" {
			t.Fatalf("minted token %q not found in token list JSON:\n%s", name, r.Stdout)
		}

		env.RunOK(t, "token", "delete", id, "--yes")

		r = env.RunOK(t, "token", "list")
		harness.AssertNotContains(t, r.Stdout, name)
	})

	t.Run("create_service_multi_env_denied", func(t *testing.T) {
		// internal/cmdutil/guard.go GuardServiceCreationScope (wired in
		// internal/cmd/create_service.go): Railway creates service instances
		// in ALL environments (docs/railway-service-creation-behavior.md),
		// and a project token cannot delete the instances it would leak into
		// the other environments — so in a multi-environment project the
		// creation must fail fast, before serviceCreate is ever called.
		//
		// The fixture is otherwise single-env, so the probe environment is
		// created with the WORKSPACE token (a project token cannot create
		// environments anyway) and removed in a defer even on failure —
		// later tests rely on the fixture being single-env again.
		wsToken := os.Getenv("RAILWAY_WORKSPACE_TOKEN")
		if wsToken == "" {
			t.Fatal("RAILWAY_WORKSPACE_TOKEN not set")
		}

		const probeEnv = "leakprobe"
		if _, stderr, code := runCLI(wsToken, "create", "environment", probeEnv, "-p", fixtureProject); code != 0 {
			t.Fatalf("create environment %s failed (exit %d):\n%s", probeEnv, code, stderr)
		}
		defer func() {
			// Best-effort: deleting an environment requires the workspace
			// token anyway (a project token cannot delete environments).
			if _, stderr, code := runCLI(wsToken, "delete", "environment", probeEnv, "--yes", "-p", fixtureProject); code != 0 {
				t.Logf("cleanup: delete environment %s failed (exit %d): %s", probeEnv, code, stderr)
			}
		}()

		// Wait until the probe environment is visible to the project token —
		// the guard's ListEnvironments must see the project as multi-env.
		visible := false
		for i := 0; i < 10; i++ {
			if r := env.Run("get", "environments"); r.ExitCode == 0 && strings.Contains(r.Stdout, probeEnv) {
				visible = true
				break
			}
			time.Sleep(2 * time.Second)
		}
		if !visible {
			t.Fatalf("environment %s not visible to the project token after 10 attempts", probeEnv)
		}

		// Under the PROJECT token, creation must fail fast with the guard's
		// message naming the leak.
		name := harness.UniqueName()
		r := env.RunFail(t, "create", "service", name, "--image", "nginx:1.25-alpine")
		harness.AssertContains(t, r.Stdout+r.Stderr, "multi-environment")
		harness.AssertContains(t, r.Stdout+r.Stderr, "leak")
		harness.AssertContains(t, r.Stdout+r.Stderr, probeEnv)

		// Fail-fast means NO service was created — not even in the token's
		// own environment.
		r = env.RunOK(t, "get", "services")
		harness.AssertNotContains(t, r.Stdout, name)
	})
}
