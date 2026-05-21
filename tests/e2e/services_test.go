//go:build e2e

package e2e

import "testing"

// TestServices exercises service CRUD operations.
// Setup: creates project + environment
//
//	go test -tags e2e -v -run TestServices ./tests/e2e/...
func TestServices(t *testing.T) {
	env := SetupEnvironment(t)

	t.Run("create", func(t *testing.T) {
		env.RunOK(t, env.WithPE("create", "service", env.ServiceName,
			"--image", env.ServiceImg)...)
	})

	t.Run("get_table", func(t *testing.T) {
		r := env.RunOK(t, env.WithPE("get", "services")...)
		AssertContains(t, r.Stdout, env.ServiceName)
	})

	t.Run("get_json", func(t *testing.T) {
		r := env.RunOK(t, env.WithPE("get", "services", "-o", "json")...)
		AssertValidJSON(t, r.Stdout)
		AssertContains(t, r.Stdout, env.ServiceName)
	})

	t.Run("get_yaml", func(t *testing.T) {
		r := env.RunOK(t, env.WithPE("get", "services", "-o", "yaml")...)
		AssertValidYAML(t, r.Stdout)
	})

	t.Run("get_wide", func(t *testing.T) {
		r := env.RunOK(t, env.WithPE("get", "services", "-o", "wide")...)
		AssertContains(t, r.Stdout, env.ServiceName)
	})

	t.Run("describe_table", func(t *testing.T) {
		r := env.RunOK(t, env.WithPE("describe", "service", env.ServiceName)...)
		AssertContains(t, r.Stdout, env.ServiceName)
	})

	t.Run("describe_json", func(t *testing.T) {
		r := env.RunOK(t, env.WithPE("describe", "service", env.ServiceName, "-o", "json")...)
		AssertValidJSON(t, r.Stdout)
	})

	t.Run("describe_yaml", func(t *testing.T) {
		r := env.RunOK(t, env.WithPE("describe", "service", env.ServiceName, "-o", "yaml")...)
		AssertValidYAML(t, r.Stdout)
	})

	t.Run("describe_show_values", func(t *testing.T) {
		r := env.RunOK(t, env.WithPE("describe", "service", env.ServiceName, "--show-values")...)
		AssertContains(t, r.Stdout, env.ServiceName)
	})

	t.Run("describe_substring", func(t *testing.T) {
		r := env.RunOK(t, env.WithPE("describe", "service", "we")...)
		AssertContains(t, r.Stdout, env.ServiceName)
	})

	// Error cases
	t.Run("get_no_project", func(t *testing.T) {
		env.RunFail(t, "get", "services", "-e", env.EnvName)
	})

	t.Run("get_no_environment", func(t *testing.T) {
		env.RunFail(t, "get", "services", "-p", env.ProjectName)
	})

	t.Run("describe_nonexistent", func(t *testing.T) {
		env.RunFail(t, env.WithPE("describe", "service", "nonexistent-svc-xyz")...)
	})

	t.Run("create_no_image", func(t *testing.T) {
		env.RunFail(t, env.WithPE("create", "service", "bad-svc")...)
	})

	// Domain generation
	t.Run("create_with_generate_domain", func(t *testing.T) {
		svcName := "domain-svc"
		r := env.RunOK(t, env.WithPE("create", "service", svcName,
			"--image", "nginx:1.25-alpine", "--generate-domain", "8080")...)
		AssertContains(t, r.Stdout, "Domain generated:")
		AssertContains(t, r.Stdout, ".up.railway.app")

		// Cleanup
		env.RunOK(t, env.WithPE("delete", "service", svcName, "--yes")...)
	})

	// TCP proxy generation
	t.Run("create_with_generate_tcp", func(t *testing.T) {
		svcName := "tcp-svc"
		r := env.RunOK(t, env.WithPE("create", "service", svcName,
			"--image", "postgres:16-alpine", "--generate-tcp", "5432")...)
		AssertContains(t, r.Stdout, "TCP proxy generated:")

		// Cleanup
		env.RunOK(t, env.WithPE("delete", "service", svcName, "--yes")...)
	})

	// Cleanup: delete service
	t.Run("delete", func(t *testing.T) {
		env.RunOK(t, env.WithPE("delete", "service", env.ServiceName, "--yes")...)
	})

	t.Run("verify_deleted", func(t *testing.T) {
		r := env.RunOK(t, env.WithPE("get", "services", "-o", "json")...)
		AssertNotContains(t, r.Stdout, env.ServiceName)
	})
}
