//go:build e2e

package project

import (
	"testing"
	"time"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// TestUpdateService exercises all update service flags inside the shared
// fixture project under the minted project token (no -p/-e flags).
//
//	go test -tags e2e -v -run TestUpdateService ./tests/e2e/project/...
func TestUpdateService(t *testing.T) {
	env := fixtureEnv(t)
	svc := createService(t, env)

	t.Run("update_image", func(t *testing.T) {
		env.RunOK(t, "update", "service", svc, "--image", "nginx:1.26-alpine")
		time.Sleep(2 * time.Second)
	})

	t.Run("update_image_skip_deployment", func(t *testing.T) {
		env.RunOK(t, "update", "service", svc,
			"--image", "nginx:1.25-alpine", "--skip-deployment")
	})

	t.Run("update_start_command", func(t *testing.T) {
		env.RunOK(t, "update", "service", svc,
			"--start-command", "nginx -g 'daemon off;'")
	})

	t.Run("update_restart_policy", func(t *testing.T) {
		env.RunOK(t, "update", "service", svc,
			"--restart-policy", "ON_FAILURE")
	})

	t.Run("update_restart_policy_max_retries", func(t *testing.T) {
		env.RunOK(t, "update", "service", svc,
			"--restart-policy", "ON_FAILURE", "--max-retries", "3")
	})

	t.Run("update_replicas", func(t *testing.T) {
		env.RunOK(t, "update", "service", svc,
			"--replicas", "1")
	})

	t.Run("update_healthcheck_path", func(t *testing.T) {
		env.RunOK(t, "update", "service", svc,
			"--healthcheck-path", "/health")
	})

	t.Run("update_healthcheck_timeout", func(t *testing.T) {
		env.RunOK(t, "update", "service", svc,
			"--healthcheck-path", "/health", "--healthcheck-timeout", "120")
	})

	t.Run("update_combined", func(t *testing.T) {
		env.RunOK(t, "update", "service", svc,
			"--image", "nginx:1.25-alpine",
			"--replicas", "1",
			"--healthcheck-path", "/")
	})

	// Domain generation
	t.Run("generate_domain", func(t *testing.T) {
		r := env.RunOK(t, "update", "service", svc,
			"--generate-domain", "5678")
		harness.AssertContains(t, r.Stdout, ".up.railway.app")
	})

	t.Run("generate_domain_idempotent", func(t *testing.T) {
		r := env.RunOK(t, "update", "service", svc,
			"--generate-domain", "5678")
		harness.AssertContains(t, r.Stdout, "Domain already exists:")
		harness.AssertContains(t, r.Stdout, ".up.railway.app")
	})

	// TCP proxy generation
	t.Run("generate_tcp", func(t *testing.T) {
		r := env.RunOK(t, "update", "service", svc,
			"--generate-tcp", "5432")
		harness.AssertContains(t, r.Stdout, "TCP proxy generated:")
	})

	t.Run("generate_tcp_idempotent", func(t *testing.T) {
		r := env.RunOK(t, "update", "service", svc,
			"--generate-tcp", "5432")
		harness.AssertContains(t, r.Stdout, "TCP proxy already exists:")
	})

	// Error cases
	t.Run("no_flags", func(t *testing.T) {
		env.RunFail(t, "update", "service", svc)
	})

	t.Run("invalid_restart_policy", func(t *testing.T) {
		env.RunFail(t, "update", "service", svc,
			"--restart-policy", "INVALID")
	})

	t.Run("max_retries_no_policy", func(t *testing.T) {
		env.RunFail(t, "update", "service", svc,
			"--max-retries", "3")
	})

	t.Run("replicas_zero", func(t *testing.T) {
		env.RunFail(t, "update", "service", svc,
			"--replicas", "0")
	})

	t.Run("nonexistent_service", func(t *testing.T) {
		env.RunFail(t, "update", "service", "nonexistent-svc-xyz",
			"--image", "nginx")
	})
}
