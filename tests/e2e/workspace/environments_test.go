//go:build e2e

package workspace

import (
	"testing"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// TestEnvironments exercises environment CRUD operations.
// Setup: creates a project
//
//	go test -tags e2e -v -run TestEnvironments ./tests/e2e/workspace/...
func TestEnvironments(t *testing.T) {
	env := harness.SetupProject(t, token)

	t.Run("get_default_production", func(t *testing.T) {
		r := env.RunOK(t, env.WithP("get", "environments")...)
		harness.AssertContains(t, r.Stdout, "production")
	})

	t.Run("get_json", func(t *testing.T) {
		r := env.RunOK(t, env.WithP("get", "environments", "-o", "json")...)
		harness.AssertValidJSON(t, r.Stdout)
	})

	t.Run("get_yaml", func(t *testing.T) {
		r := env.RunOK(t, env.WithP("get", "environments", "-o", "yaml")...)
		harness.AssertValidYAML(t, r.Stdout)
	})

	t.Run("create", func(t *testing.T) {
		env.RunOK(t, env.WithP("create", "environment", env.EnvName)...)
	})

	t.Run("get_shows_staging", func(t *testing.T) {
		r := env.RunOK(t, env.WithP("get", "environments")...)
		harness.AssertContains(t, r.Stdout, env.EnvName)
	})

	t.Run("describe_table", func(t *testing.T) {
		r := env.RunOK(t, env.WithP("describe", "environment", env.EnvName)...)
		harness.AssertContains(t, r.Stdout, env.EnvName)
	})

	t.Run("describe_json", func(t *testing.T) {
		r := env.RunOK(t, env.WithP("describe", "environment", env.EnvName, "-o", "json")...)
		harness.AssertValidJSON(t, r.Stdout)
	})

	t.Run("describe_yaml", func(t *testing.T) {
		r := env.RunOK(t, env.WithP("describe", "environment", env.EnvName, "-o", "yaml")...)
		harness.AssertValidYAML(t, r.Stdout)
	})

	// Error cases
	t.Run("get_no_project", func(t *testing.T) {
		env.RunFail(t, "get", "environments")
	})

	t.Run("get_bad_project", func(t *testing.T) {
		env.RunFail(t, "get", "environments", "-p", "nonexistent-xyz-999")
	})

	t.Run("describe_nonexistent", func(t *testing.T) {
		env.RunFail(t, env.WithP("describe", "environment", "nonexistent-env-xyz")...)
	})

	// Cleanup: delete environment
	t.Run("delete", func(t *testing.T) {
		env.RunOK(t, env.WithP("delete", "environment", env.EnvName, "--yes")...)
	})

	t.Run("verify_deleted", func(t *testing.T) {
		r := env.RunOK(t, env.WithP("get", "environments")...)
		harness.AssertNotContains(t, r.Stdout, env.EnvName)
	})
}
