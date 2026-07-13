//go:build e2e

package project

import (
	"testing"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// TestProtectUnprotect proves delete-protection can be toggled with the group's
// PROJECT token — the v1.2.0 removal of the workspace/account gate. A project
// token writes its OWN environment's shared variable (authorized under the
// Project-Access-Token header), so protect/unprotect run flag-free, and
// enforcement (a blocked service delete) holds while protected.
//
// The subtests run the full loop under the project token and always unprotect
// on cleanup so the shared fixture environment is never left protected (which
// would block TestMain's teardown).
func TestProtectUnprotect(t *testing.T) {
	env := fixtureEnv(t)
	svc := createService(t, env)

	// Fail-safe: clear protection no matter how the subtests end.
	t.Cleanup(func() { env.Run("unprotect", "environment", env.EnvName) })

	t.Run("protect_under_project_token", func(t *testing.T) {
		r := env.RunOK(t, "protect", "environment", env.EnvName)
		harness.AssertContains(t, r.Stdout, "delete-protected")
	})

	t.Run("delete_blocked_while_protected", func(t *testing.T) {
		r := env.RunFail(t, "delete", "service", svc, "--yes")
		harness.AssertContains(t, r.Stderr, "delete-protected")
	})

	t.Run("unprotect_under_project_token", func(t *testing.T) {
		r := env.RunOK(t, "unprotect", "environment", env.EnvName)
		harness.AssertContains(t, r.Stdout, "no longer delete-protected")
	})

	t.Run("delete_succeeds_after_unprotect", func(t *testing.T) {
		env.RunOK(t, "delete", "service", svc, "--yes")
	})
}
