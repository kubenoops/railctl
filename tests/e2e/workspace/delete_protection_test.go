//go:build e2e

package workspace

import (
	"testing"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// TestDeleteProtection exercises the DELETE_PROTECTION guard end-to-end through
// the CLI: `protect environment` sets the flag, after which `delete environment`
// and `delete project` are refused (no bypass — --yes only skips the prompt);
// `unprotect environment` clears it, after which the delete succeeds.
//
// Both toggles write an environment-level (shared, serviceless) variable, so the
// test uses the workspace token.
//
//	go test -tags e2e -v -run TestDeleteProtection ./tests/e2e/workspace/...
func TestDeleteProtection(t *testing.T) {
	env := harness.SetupEnvironment(t, token)

	t.Run("protect", func(t *testing.T) {
		r := env.RunOK(t, env.WithP("protect", "environment", env.EnvName)...)
		harness.AssertContains(t, r.Stdout, "delete-protected")
	})

	t.Run("delete_environment_refused", func(t *testing.T) {
		r := env.RunFail(t, env.WithP("delete", "environment", env.EnvName, "--yes")...)
		harness.AssertContains(t, r.Stderr, "delete-protected")
		harness.AssertContains(t, r.Stderr, "DELETE_PROTECTION")
	})

	t.Run("delete_project_refused_names_env", func(t *testing.T) {
		r := env.RunFail(t, "delete", "project", env.ProjectName, "--yes")
		harness.AssertContains(t, r.Stderr, "delete-protected")
		harness.AssertContains(t, r.Stderr, env.EnvName)
	})

	t.Run("unprotect", func(t *testing.T) {
		r := env.RunOK(t, env.WithP("unprotect", "environment", env.EnvName)...)
		harness.AssertContains(t, r.Stdout, "no longer delete-protected")
	})

	t.Run("delete_environment_succeeds", func(t *testing.T) {
		env.RunOK(t, env.WithP("delete", "environment", env.EnvName, "--yes")...)
	})

	t.Run("verify_deleted", func(t *testing.T) {
		r := env.RunOK(t, env.WithP("get", "environments")...)
		harness.AssertNotContains(t, r.Stdout, env.EnvName)
	})
}
