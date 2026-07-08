//go:build e2e

package e2e

import (
	"bytes"
	"os/exec"
	"testing"
)

// TestEdgeCases exercises error handling, debug flags, and env var overrides.
// Setup: creates project + environment + service (to test env var overrides)
//
//	go test -tags e2e -v -run TestEdgeCases ./tests/e2e/...
func TestEdgeCases(t *testing.T) {
	env := SetupService(t)

	t.Run("invalid_output_format", func(t *testing.T) {
		r := env.RunFail(t, "get", "projects", "-o", "invalid-format")
		AssertContains(t, r.Stderr, "invalid output format")
	})

	t.Run("debug_flag", func(t *testing.T) {
		r := env.Run("get", "projects", "--debug")
		// Debug output goes to stderr — just verify it doesn't crash
		_ = r
	})

	t.Run("env_var_RAILCTL_PROJECT", func(t *testing.T) {
		cmd := exec.Command(railctl, "--token", env.Token, "get", "environments")
		cmd.Env = []string{"RAILCTL_PROJECT=" + env.ProjectName}
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &bytes.Buffer{}
		if err := cmd.Run(); err != nil {
			t.Errorf("RAILCTL_PROJECT env var didn't work: %v", err)
		}
	})

	t.Run("env_var_RAILCTL_ENVIRONMENT", func(t *testing.T) {
		cmd := exec.Command(railctl, "--token", env.Token, "get", "services")
		cmd.Env = []string{
			"RAILCTL_PROJECT=" + env.ProjectName,
			"RAILCTL_ENVIRONMENT=" + env.EnvName,
		}
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &bytes.Buffer{}
		if err := cmd.Run(); err != nil {
			t.Errorf("RAILCTL_ENVIRONMENT env var didn't work: %v", err)
		}
	})

	t.Run("env_var_RAILCTL_SERVICE", func(t *testing.T) {
		cmd := exec.Command(railctl, "--token", env.Token, "get", "variables")
		cmd.Env = []string{
			"RAILCTL_PROJECT=" + env.ProjectName,
			"RAILCTL_ENVIRONMENT=" + env.EnvName,
			"RAILCTL_SERVICE=" + env.ServiceName,
		}
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &bytes.Buffer{}
		if err := cmd.Run(); err != nil {
			t.Errorf("RAILCTL_SERVICE env var didn't work: %v", err)
		}
	})

	t.Run("environment_substring_resolution", func(t *testing.T) {
		// "stag" should match "staging"
		r := env.RunOK(t, env.WithP("describe", "environment", "stag")...)
		AssertContains(t, r.Stdout, env.EnvName)
	})
}
