//go:build e2e

package project

import (
	"testing"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// TestVariables exercises variable set/get/delete operations inside the
// shared fixture project under the minted project token (no -p/-e flags;
// -s kept — service selection is orthogonal to token scope).
//
//	go test -tags e2e -v -run TestVariables ./tests/e2e/project/...
func TestVariables(t *testing.T) {
	env := fixtureEnv(t)
	svc := createService(t, env)

	t.Run("set_single", func(t *testing.T) {
		env.RunOK(t, "set", "variable", "PORT=3000", "-s", svc)
	})

	t.Run("set_multiple", func(t *testing.T) {
		env.RunOK(t, "set", "variable", "NODE_ENV=test", "LOG_LEVEL=debug", "-s", svc)
	})

	t.Run("set_skip_deployment", func(t *testing.T) {
		env.RunOK(t, "set", "variable", "SKIP_VAR=temp", "--skip-deployment", "-s", svc)
	})

	t.Run("get_table", func(t *testing.T) {
		r := env.RunOK(t, "get", "variables", "-s", svc)
		harness.AssertContains(t, r.Stdout, "PORT")
	})

	t.Run("get_json", func(t *testing.T) {
		r := env.RunOK(t, "get", "variables", "-o", "json", "-s", svc)
		harness.AssertValidJSON(t, r.Stdout)
	})

	t.Run("get_contains_all", func(t *testing.T) {
		r := env.RunOK(t, "get", "variables", "-o", "json", "-s", svc)
		for _, v := range []string{"PORT", "NODE_ENV", "LOG_LEVEL", "SKIP_VAR"} {
			harness.AssertContains(t, r.Stdout, v)
		}
	})

	t.Run("get_yaml", func(t *testing.T) {
		r := env.RunOK(t, "get", "variables", "-o", "yaml", "-s", svc)
		harness.AssertValidYAML(t, r.Stdout)
	})

	t.Run("delete", func(t *testing.T) {
		env.RunOK(t, "delete", "variable", "SKIP_VAR", "--yes", "-s", svc)
	})

	t.Run("verify_deleted", func(t *testing.T) {
		r := env.RunOK(t, "get", "variables", "-o", "json", "-s", svc)
		harness.AssertNotContains(t, r.Stdout, "SKIP_VAR")
	})

	// Error cases
	t.Run("set_bad_format", func(t *testing.T) {
		env.RunFail(t, "set", "variable", "BADFORMAT", "-s", svc)
	})

	t.Run("set_empty_key", func(t *testing.T) {
		env.RunFail(t, "set", "variable", "=value", "-s", svc)
	})

	t.Run("get_no_service", func(t *testing.T) {
		env.RunFail(t, "get", "variables")
	})
}
