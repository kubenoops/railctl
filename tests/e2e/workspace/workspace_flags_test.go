//go:build e2e

package workspace

import (
	"testing"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// TestWorkspaceFlagIgnored verifies that passing -w with a workspace-scoped
// token does not break the command: the token is already bound to a single
// workspace, so the CLI warns on stderr and proceeds.
//
// The warning is emitted during token-type detection (internal/api/client.go,
// Probe 2) via client.WarnFn, which the cmd layer wires to stderr:
//
//	"Warning: -w/RAILCTL_WORKSPACE ignored — workspace token is already
//	 scoped to a specific workspace"
func TestWorkspaceFlagIgnored(t *testing.T) {
	harness.RequireBinary(t)
	env := &harness.Env{T: t, Token: token}

	r := env.RunOK(t, "get", "projects", "-w", "some-workspace-name")
	harness.AssertContains(t, r.Stderr, "-w/RAILCTL_WORKSPACE ignored")
	harness.AssertContains(t, r.Stderr, "workspace token is already scoped")
}
