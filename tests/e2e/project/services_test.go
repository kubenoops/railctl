//go:build e2e

package project

import (
	"testing"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// TestServices exercises service CRUD operations inside the shared fixture
// project under the minted project token. No -p/-e flags anywhere: the token
// carries the scope.
//
//	go test -tags e2e -v -run TestServices ./tests/e2e/project/...
func TestServices(t *testing.T) {
	env := fixtureEnv(t)
	svcName := harness.UniqueName()
	env.ServiceName = svcName
	t.Cleanup(func() {
		// Safety net if the test dies before its delete subtest — keeps the
		// shared fixture lean; TestMain's teardown would catch it too.
		env.Run("delete", "service", svcName, "--yes")
	})

	t.Run("create", func(t *testing.T) {
		env.RunOK(t, "create", "service", svcName, "--image", env.ServiceImg)
	})

	t.Run("get_table", func(t *testing.T) {
		r := env.RunOK(t, "get", "services")
		harness.AssertContains(t, r.Stdout, svcName)
	})

	t.Run("get_json", func(t *testing.T) {
		r := env.RunOK(t, "get", "services", "-o", "json")
		harness.AssertValidJSON(t, r.Stdout)
		harness.AssertContains(t, r.Stdout, svcName)
	})

	t.Run("get_yaml", func(t *testing.T) {
		r := env.RunOK(t, "get", "services", "-o", "yaml")
		harness.AssertValidYAML(t, r.Stdout)
	})

	t.Run("get_wide", func(t *testing.T) {
		r := env.RunOK(t, "get", "services", "-o", "wide")
		harness.AssertContains(t, r.Stdout, svcName)
	})

	t.Run("describe_table", func(t *testing.T) {
		r := env.RunOK(t, "describe", "service", svcName)
		harness.AssertContains(t, r.Stdout, svcName)
	})

	t.Run("describe_json", func(t *testing.T) {
		r := env.RunOK(t, "describe", "service", svcName, "-o", "json")
		harness.AssertValidJSON(t, r.Stdout)
	})

	t.Run("describe_yaml", func(t *testing.T) {
		r := env.RunOK(t, "describe", "service", svcName, "-o", "yaml")
		harness.AssertValidYAML(t, r.Stdout)
	})

	t.Run("describe_show_values", func(t *testing.T) {
		r := env.RunOK(t, "describe", "service", svcName, "--show-values")
		harness.AssertContains(t, r.Stdout, svcName)
	})

	t.Run("describe_substring", func(t *testing.T) {
		// Substring resolution: the unique name minus its "e2e-" prefix is
		// still a unique substring of this test's service.
		r := env.RunOK(t, "describe", "service", svcName[4:])
		harness.AssertContains(t, r.Stdout, svcName)
	})

	// NOTE (semantics adapted): the flat suite's get_no_project /
	// get_no_environment error cases asserted that omitting -p/-e fails for
	// a workspace token. Under a project token the same commands SUCCEED —
	// the token carries the scope — so those subtests have no L3 equivalent
	// and were dropped; TestBoundaries covers the flag-ignoring behaviour.

	t.Run("describe_nonexistent", func(t *testing.T) {
		env.RunFail(t, "describe", "service", "nonexistent-svc-xyz")
	})

	t.Run("create_no_image", func(t *testing.T) {
		env.RunFail(t, "create", "service", "bad-svc")
	})

	// Domain generation
	t.Run("create_with_generate_domain", func(t *testing.T) {
		domainSvc := harness.UniqueName()
		r := env.RunOK(t, "create", "service", domainSvc,
			"--image", "nginx:1.25-alpine", "--generate-domain", "8080")
		harness.AssertContains(t, r.Stdout, "Domain generated:")
		harness.AssertContains(t, r.Stdout, ".up.railway.app")

		// Cleanup
		env.RunOK(t, "delete", "service", domainSvc, "--yes")
	})

	// TCP proxy generation
	t.Run("create_with_generate_tcp", func(t *testing.T) {
		tcpSvc := harness.UniqueName()
		r := env.RunOK(t, "create", "service", tcpSvc,
			"--image", "postgres:16-alpine", "--generate-tcp", "5432")
		harness.AssertContains(t, r.Stdout, "TCP proxy generated:")

		// Cleanup
		env.RunOK(t, "delete", "service", tcpSvc, "--yes")
	})

	// Cleanup: delete service
	t.Run("delete", func(t *testing.T) {
		env.RunOK(t, "delete", "service", svcName, "--yes")
	})

	t.Run("verify_deleted", func(t *testing.T) {
		r := env.RunOK(t, "get", "services", "-o", "json")
		harness.AssertNotContains(t, r.Stdout, svcName)
	})
}
