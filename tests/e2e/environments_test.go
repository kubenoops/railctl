//go:build e2e

package e2e

import "testing"

// TestEnvironments exercises environment CRUD operations.
// Setup: creates a project
//
//	go test -tags e2e -v -run TestEnvironments ./tests/e2e/...
func TestEnvironments(t *testing.T) {
	env := SetupProject(t)

	t.Run("get_default_production", func(t *testing.T) {
		r := env.RunOK(t, env.WithP("get", "environments")...)
		AssertContains(t, r.Stdout, "production")
	})

	t.Run("get_json", func(t *testing.T) {
		r := env.RunOK(t, env.WithP("get", "environments", "-o", "json")...)
		AssertValidJSON(t, r.Stdout)
	})

	t.Run("get_yaml", func(t *testing.T) {
		r := env.RunOK(t, env.WithP("get", "environments", "-o", "yaml")...)
		AssertValidYAML(t, r.Stdout)
	})

	t.Run("create", func(t *testing.T) {
		env.RunOK(t, env.WithP("create", "environment", env.EnvName)...)
	})

	t.Run("get_shows_staging", func(t *testing.T) {
		r := env.RunOK(t, env.WithP("get", "environments")...)
		AssertContains(t, r.Stdout, env.EnvName)
	})

	t.Run("describe_table", func(t *testing.T) {
		r := env.RunOK(t, env.WithP("describe", "environment", env.EnvName)...)
		AssertContains(t, r.Stdout, env.EnvName)
	})

	t.Run("describe_json", func(t *testing.T) {
		r := env.RunOK(t, env.WithP("describe", "environment", env.EnvName, "-o", "json")...)
		AssertValidJSON(t, r.Stdout)
	})

	t.Run("describe_yaml", func(t *testing.T) {
		r := env.RunOK(t, env.WithP("describe", "environment", env.EnvName, "-o", "yaml")...)
		AssertValidYAML(t, r.Stdout)
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
		AssertNotContains(t, r.Stdout, env.EnvName)
	})
}
