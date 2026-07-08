//go:build e2e

package project

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// TestEdgeCases exercises error handling, debug flags, and env var
// behaviour inside the shared fixture project under the minted project
// token.
//
//	go test -tags e2e -v -run TestEdgeCases ./tests/e2e/project/...
func TestEdgeCases(t *testing.T) {
	env := fixtureEnv(t)
	svc := createService(t, env)

	t.Run("invalid_output_format", func(t *testing.T) {
		// Format validation happens before token-scope checks
		// (internal/cmd/get_projects.go), so the error is the same for
		// every token type.
		r := env.RunFail(t, "get", "projects", "-o", "invalid-format")
		harness.AssertContains(t, r.Stderr, "invalid output format")
	})

	t.Run("debug_flag", func(t *testing.T) {
		// Under a project token `get projects` exits non-zero (scoped
		// error, see TestBoundaries); as in the flat suite we only verify
		// --debug does not crash the binary.
		r := env.Run("get", "projects", "--debug")
		_ = r
	})

	t.Run("env_var_RAILCTL_PROJECT", func(t *testing.T) {
		// Semantics adapted for L3: with a project token RAILCTL_PROJECT is
		// ignored-with-warning (internal/cmdutil/context.go) and the command
		// must still succeed on the token's own scope.
		cmd := exec.Command(harness.Railctl, "--token", env.Token, "get", "environments")
		cmd.Env = []string{"RAILCTL_PROJECT=some-other-project"}
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Errorf("get environments with RAILCTL_PROJECT set should still succeed under a project token: %v\nstderr: %s",
				err, stderr.String())
		}
		harness.AssertContains(t, stderr.String(), "-p/RAILCTL_PROJECT ignored")
	})

	t.Run("env_var_RAILCTL_ENVIRONMENT", func(t *testing.T) {
		// Semantics adapted for L3: RAILCTL_ENVIRONMENT is ignored-with-
		// warning; the command must still succeed on the token's own scope.
		cmd := exec.Command(harness.Railctl, "--token", env.Token, "get", "services")
		cmd.Env = []string{"RAILCTL_ENVIRONMENT=some-other-env"}
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Errorf("get services with RAILCTL_ENVIRONMENT set should still succeed under a project token: %v\nstderr: %s",
				err, stderr.String())
		}
		harness.AssertContains(t, stderr.String(), "-e/RAILCTL_ENVIRONMENT ignored")
	})

	t.Run("env_var_RAILCTL_SERVICE", func(t *testing.T) {
		// RAILCTL_SERVICE stays meaningful under a project token: the token
		// scopes project + environment, service selection is still the
		// caller's (no RAILCTL_PROJECT/ENVIRONMENT needed).
		cmd := exec.Command(harness.Railctl, "--token", env.Token, "get", "variables")
		cmd.Env = []string{"RAILCTL_SERVICE=" + svc}
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &bytes.Buffer{}
		if err := cmd.Run(); err != nil {
			t.Errorf("RAILCTL_SERVICE env var didn't work: %v", err)
		}
	})

	t.Run("environment_arg_ignored", func(t *testing.T) {
		// Adapted from the flat suite's environment_substring_resolution:
		// under a project token the <name> argument of `describe
		// environment` is ignored-with-warning (the token pins the
		// environment) and the token's own environment is described.
		// Environment substring resolution itself is a workspace-token
		// behaviour; service substring resolution is covered in
		// TestServices/describe_substring.
		r := env.RunOK(t, "describe", "environment", "some-other-env")
		harness.AssertContains(t, r.Stderr, "-e/RAILCTL_ENVIRONMENT ignored")
		harness.AssertContains(t, r.Stdout, fixtureEnvName)
	})
}
