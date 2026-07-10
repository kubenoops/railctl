//go:build e2e

package workspace

import (
	"testing"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// TestLeastPrivilegeHint exercises the stderr least-privilege nudge: running a
// project-scoped operation with a broad (workspace) token in text output mode
// prints a hint pointing at 'railctl token create' and RAILCTL_NO_HINTS, while
// json output mode and RAILCTL_NO_HINTS=1 both suppress it.
//
//	go test -tags e2e -v -run TestLeastPrivilegeHint ./tests/e2e/workspace/...
func TestLeastPrivilegeHint(t *testing.T) {
	env := harness.SetupProject(t, token)

	t.Run("text_mode_emits_hint", func(t *testing.T) {
		r := env.Run("get", "services", "-p", env.ProjectName, "-e", "production")
		if r.ExitCode != 0 {
			t.Fatalf("railctl get services failed (exit %d):\nstderr: %s", r.ExitCode, r.Stderr)
		}
		harness.AssertContains(t, r.Stderr, "RAILCTL_NO_HINTS")
	})

	t.Run("json_mode_no_hint", func(t *testing.T) {
		r := env.Run("get", "services", "-p", env.ProjectName, "-e", "production", "-o", "json")
		harness.AssertNotContains(t, r.Stderr, "RAILCTL_NO_HINTS")
	})

	t.Run("env_var_silences", func(t *testing.T) {
		r := env.RunEnv([]string{"RAILCTL_NO_HINTS=1"}, "get", "services", "-p", env.ProjectName, "-e", "production")
		harness.AssertNotContains(t, r.Stderr, "RAILCTL_NO_HINTS")
	})
}
