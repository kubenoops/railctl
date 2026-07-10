//go:build e2e

package project

import (
	"strings"
	"testing"
	"time"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// deployReadySSHService creates a service in the fixture project, triggers a
// deterministic deployment, waits for it to reach SUCCESS, and lets the
// container + SSH relay settle. exec/port-forward need a live container on the
// far side of the relay, so both tests build their target this way. All
// commands run flag-free under the group's PROJECT token.
func deployReadySSHService(t *testing.T, env *harness.Env) string {
	t.Helper()
	svc := createService(t, env)
	// `create service` does not reliably deploy by itself; trigger explicitly.
	env.RunOK(t, "create", "deployment", "-s", svc)
	if err := harness.WaitForDeploymentSuccess(env, svc); err != nil {
		t.Fatalf("service %s never became ready: %v", svc, err)
	}
	// The relay can briefly race a just-SUCCESS container (an exec landing
	// before the process is accepting sessions returns empty); let it settle.
	time.Sleep(12 * time.Second)
	return svc
}

// TestExec proves `railctl exec` end-to-end over Railway's SSH relay under the
// group's PROJECT token — the v1.1.0 headline: the old project-token gate is
// gone, so a project token plus a pre-registered SSH key reaches the container.
// The key is registered with the bootstrap workspace token (a project token
// cannot register keys) and revoked at the end.
func TestExec(t *testing.T) {
	env := fixtureEnv(t)
	key := harness.RegisterEphemeralSSHKey(t, bootstrapToken) // t.Skip if no ssh/ssh-keygen
	defer key.Revoke(t)

	svc := deployReadySSHService(t, env)

	t.Run("run_command_under_project_token", func(t *testing.T) {
		// Flag-free: the project token carries the scope. -i points at the
		// ephemeral key; ssh authenticates with it, not the token.
		r := env.RunOK(t, "exec", svc, "-i", key.PrivateKeyPath, "--", "hostname")
		if strings.TrimSpace(r.Stdout) == "" {
			t.Errorf("expected non-empty hostname from the container, got empty\nstderr: %s", r.Stderr)
		}
	})

	t.Run("argv_preserved_kubectl_style", func(t *testing.T) {
		// Shell metacharacters must survive the relay: this is the regression
		// guard for the argv-quoting fix. Without it the remote shell re-splits
		// the command and the first echo's output is lost.
		r := env.RunOK(t, "exec", svc, "-i", key.PrivateKeyPath, "--", "sh", "-c", "echo LINE1; echo LINE2")
		harness.AssertContains(t, r.Stdout, "LINE1")
		harness.AssertContains(t, r.Stdout, "LINE2")
	})
}
