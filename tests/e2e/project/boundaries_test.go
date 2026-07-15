//go:build e2e

package project

import (
	"testing"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// TestBoundaries proves the project-token fail-fasts and guardrails: project
// enumeration is denied, -p/-e/-w contradictions fail fast (matching values
// proceed silently), and token minting/listing/deletion are refused (Railway
// denies them to project-scoped tokens — not even self-minting works).
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
		// guard fails fast before any API mutation. -p is required by the
		// command's flag validation (which runs before the guard), so pass the
		// fixture project — a matching -p is not a contradiction.
		r := env.RunFail(t, "create", "environment", "should-not-exist", "-p", env.ProjectName)
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

	// Railway denies token minting, listing, and deletion to project-scoped
	// tokens — a project token cannot mint even for its OWN scope. Verified at
	// the API: the raw projectTokenCreate mutation sent with the correct
	// Project-Access-Token header returns "Not Authorized", while the identical
	// input succeeds with a workspace token, so this is Railway's boundary and
	// not a railctl defect. railctl fails fast with an actionable message
	// rather than surfacing the bare API error.
	//
	// This subtest previously asserted the opposite (self-minting worked when
	// it was written). It is now the tripwire: if Railway ever re-allows it,
	// this flips red and the capability matrix + skill get updated.
	t.Run("token_ops_denied", func(t *testing.T) {
		cases := map[string][]string{
			"create": {"token", "create", harness.UniqueName()},
			"list":   {"token", "list"},
			"delete": {"token", "delete", "00000000-0000-0000-0000-000000000000", "--yes"},
		}
		for op, args := range cases {
			r := env.RunFail(t, args...)
			harness.AssertContains(t, r.Stdout+r.Stderr, "project token")
			t.Logf("token %s correctly refused for a project token", op)
		}
	})
}
