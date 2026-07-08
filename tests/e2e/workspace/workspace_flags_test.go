//go:build e2e

package workspace

import (
	"regexp"
	"testing"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// TestWorkspaceFlagContradiction verifies the fail-fast semantics of -w with
// a workspace-scoped token: the token is bound to a single workspace (it
// self-identifies via the apiToken introspection), so a -w value naming a
// DIFFERENT workspace is a contradiction and the command fails
// (internal/api/client.go, GetWorkspaceID → checkWorkspaceHint):
//
//	"token is scoped to workspace 'NAME' but -w/--workspace 'VALUE' was
//	 given — refusing to proceed"
//
// A -w value matching the token's own workspace proceeds silently. The real
// workspace name is not statically known to the test, so it is extracted
// from the mismatch error message itself.
func TestWorkspaceFlagContradiction(t *testing.T) {
	harness.RequireBinary(t)
	env := &harness.Env{T: t, Token: token}

	r := env.RunFail(t, "get", "projects", "-w", "some-workspace-name")
	harness.AssertContains(t, r.Stderr, "scoped to workspace")

	// Extract the token's actual workspace name from the error message and
	// prove that a matching -w proceeds silently.
	m := regexp.MustCompile(`scoped to workspace '([^']+)'`).FindStringSubmatch(r.Stderr)
	if m == nil {
		t.Fatalf("could not extract the workspace name from the error:\n%s", r.Stderr)
	}
	wsName := m[1]

	r = env.RunOK(t, "get", "projects", "-w", wsName)
	harness.AssertNotContains(t, r.Stderr, "ignored")
	harness.AssertNotContains(t, r.Stderr, "scoped to workspace")
}
