//go:build e2e

package e2e

import (
	"testing"
	"time"
)

// TestUpdateService exercises all update service flags.
// Setup: creates project + environment + service
//
//	go test -tags e2e -v -run TestUpdateService ./tests/e2e/...
func TestUpdateService(t *testing.T) {
	env := SetupService(t)

	t.Run("update_image", func(t *testing.T) {
		env.RunOK(t, env.WithPE("update", "service", env.ServiceName, "--image", "nginx:1.26-alpine")...)
		time.Sleep(2 * time.Second)
	})

	t.Run("update_image_skip_deployment", func(t *testing.T) {
		env.RunOK(t, env.WithPE("update", "service", env.ServiceName,
			"--image", "nginx:1.25-alpine", "--skip-deployment")...)
	})

	t.Run("update_start_command", func(t *testing.T) {
		env.RunOK(t, env.WithPE("update", "service", env.ServiceName,
			"--start-command", "nginx -g 'daemon off;'")...)
	})

	t.Run("update_restart_policy", func(t *testing.T) {
		env.RunOK(t, env.WithPE("update", "service", env.ServiceName,
			"--restart-policy", "ON_FAILURE")...)
	})

	t.Run("update_restart_policy_max_retries", func(t *testing.T) {
		env.RunOK(t, env.WithPE("update", "service", env.ServiceName,
			"--restart-policy", "ON_FAILURE", "--max-retries", "3")...)
	})

	t.Run("update_replicas", func(t *testing.T) {
		env.RunOK(t, env.WithPE("update", "service", env.ServiceName,
			"--replicas", "1")...)
	})

	t.Run("update_healthcheck_path", func(t *testing.T) {
		env.RunOK(t, env.WithPE("update", "service", env.ServiceName,
			"--healthcheck-path", "/health")...)
	})

	t.Run("update_healthcheck_timeout", func(t *testing.T) {
		env.RunOK(t, env.WithPE("update", "service", env.ServiceName,
			"--healthcheck-path", "/health", "--healthcheck-timeout", "120")...)
	})

	t.Run("update_combined", func(t *testing.T) {
		env.RunOK(t, env.WithPE("update", "service", env.ServiceName,
			"--image", "nginx:1.25-alpine",
			"--replicas", "1",
			"--healthcheck-path", "/")...)
	})

	// Domain generation
	t.Run("generate_domain", func(t *testing.T) {
		r := env.RunOK(t, env.WithPE("update", "service", env.ServiceName,
			"--generate-domain", "5678")...)
		AssertContains(t, r.Stdout, ".up.railway.app")
	})

	t.Run("generate_domain_idempotent", func(t *testing.T) {
		r := env.RunOK(t, env.WithPE("update", "service", env.ServiceName,
			"--generate-domain", "5678")...)
		AssertContains(t, r.Stdout, "Domain already exists:")
		AssertContains(t, r.Stdout, ".up.railway.app")
	})

	// TCP proxy generation
	t.Run("generate_tcp", func(t *testing.T) {
		r := env.RunOK(t, env.WithPE("update", "service", env.ServiceName,
			"--generate-tcp", "5432")...)
		AssertContains(t, r.Stdout, "TCP proxy generated:")
	})

	t.Run("generate_tcp_idempotent", func(t *testing.T) {
		r := env.RunOK(t, env.WithPE("update", "service", env.ServiceName,
			"--generate-tcp", "5432")...)
		AssertContains(t, r.Stdout, "TCP proxy already exists:")
	})

	// Error cases
	t.Run("no_flags", func(t *testing.T) {
		env.RunFail(t, env.WithPE("update", "service", env.ServiceName)...)
	})

	t.Run("invalid_restart_policy", func(t *testing.T) {
		env.RunFail(t, env.WithPE("update", "service", env.ServiceName,
			"--restart-policy", "INVALID")...)
	})

	t.Run("max_retries_no_policy", func(t *testing.T) {
		env.RunFail(t, env.WithPE("update", "service", env.ServiceName,
			"--max-retries", "3")...)
	})

	t.Run("replicas_zero", func(t *testing.T) {
		env.RunFail(t, env.WithPE("update", "service", env.ServiceName,
			"--replicas", "0")...)
	})

	t.Run("nonexistent_service", func(t *testing.T) {
		env.RunFail(t, env.WithPE("update", "service", "nonexistent-svc-xyz",
			"--image", "nginx")...)
	})
}
