//go:build e2e

package project

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// TestBoundaries proves the project-token fail-fasts and guardrails: project
// enumeration is denied, -p/-e cannot re-aim the token, and the token can
// self-mint sibling tokens within its own scope.
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

	t.Run("p_flag_ignored", func(t *testing.T) {
		// internal/cmdutil/context.go: "Warning: -p/RAILCTL_PROJECT ignored
		// — project token is already scoped to a specific project". The
		// command must still succeed, operating on the token's own project.
		r := env.RunOK(t, "get", "services", "-p", "some-other-project")
		harness.AssertContains(t, r.Stderr, "-p/RAILCTL_PROJECT ignored")
	})

	t.Run("e_flag_ignored", func(t *testing.T) {
		// internal/cmdutil/context.go: "Warning: -e/RAILCTL_ENVIRONMENT
		// ignored — project token is already scoped to a specific
		// environment". Fires because `get services` sets NeedEnvironment.
		r := env.RunOK(t, "get", "services", "-e", "some-other-env")
		harness.AssertContains(t, r.Stderr, "-e/RAILCTL_ENVIRONMENT ignored")
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
}
