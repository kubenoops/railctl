//go:build e2e

package e2e

import "testing"

// TestVariables exercises variable set/get/delete operations.
// Setup: creates project + environment + service
//
//	go test -tags e2e -v -run TestVariables ./tests/e2e/...
func TestVariables(t *testing.T) {
	env := SetupService(t)

	t.Run("set_single", func(t *testing.T) {
		env.RunOK(t, env.WithPES("set", "variable", "PORT=3000")...)
	})

	t.Run("set_multiple", func(t *testing.T) {
		env.RunOK(t, env.WithPES("set", "variable", "NODE_ENV=test", "LOG_LEVEL=debug")...)
	})

	t.Run("set_skip_deployment", func(t *testing.T) {
		env.RunOK(t, env.WithPES("set", "variable", "SKIP_VAR=temp", "--skip-deployment")...)
	})

	t.Run("get_table", func(t *testing.T) {
		r := env.RunOK(t, env.WithPES("get", "variables")...)
		AssertContains(t, r.Stdout, "PORT")
	})

	t.Run("get_json", func(t *testing.T) {
		r := env.RunOK(t, env.WithPES("get", "variables", "-o", "json")...)
		AssertValidJSON(t, r.Stdout)
	})

	t.Run("get_contains_all", func(t *testing.T) {
		r := env.RunOK(t, env.WithPES("get", "variables", "-o", "json")...)
		for _, v := range []string{"PORT", "NODE_ENV", "LOG_LEVEL", "SKIP_VAR"} {
			AssertContains(t, r.Stdout, v)
		}
	})

	t.Run("get_yaml", func(t *testing.T) {
		r := env.RunOK(t, env.WithPES("get", "variables", "-o", "yaml")...)
		AssertValidYAML(t, r.Stdout)
	})

	t.Run("delete", func(t *testing.T) {
		env.RunOK(t, env.WithPES("delete", "variable", "SKIP_VAR", "--yes")...)
	})

	t.Run("verify_deleted", func(t *testing.T) {
		r := env.RunOK(t, env.WithPES("get", "variables", "-o", "json")...)
		AssertNotContains(t, r.Stdout, "SKIP_VAR")
	})

	// Error cases
	t.Run("set_bad_format", func(t *testing.T) {
		env.RunFail(t, env.WithPES("set", "variable", "BADFORMAT")...)
	})

	t.Run("set_empty_key", func(t *testing.T) {
		env.RunFail(t, env.WithPES("set", "variable", "=value")...)
	})

	t.Run("get_no_service", func(t *testing.T) {
		env.RunFail(t, env.WithPE("get", "variables")...)
	})
}
