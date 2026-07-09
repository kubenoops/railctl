package cmd

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/sshx"
	"github.com/kubenoops/railctl/internal/types"
)

// fakeRunner records the argv it was asked to run without launching ssh.
type fakeRunner struct {
	called  bool
	gotArgv []string
	ret     error
}

func (f *fakeRunner) Run(_ context.Context, argv []string, _ sshx.Stdio) error {
	f.called = true
	f.gotArgv = argv
	return f.ret
}

// execTestClient is a MockClient wired for a successful account-token exec:
// resolves a project/env/service and a service instance.
func execTestClient(instanceID string) *api.MockClient {
	return &api.MockClient{
		IsProjectTokenFunc: func() (bool, error) { return false, nil },
		GetWorkspaceIDFunc: func() (string, error) { return "ws-1", nil },
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return []types.Environment{{ID: "env-1", Name: "production"}}, nil
		},
		ListServicesFunc: func(projectID, environmentID string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{{ID: "svc-1", Name: "api"}}, nil
		},
		GetServiceInstanceIDFunc: func(environmentID, serviceID string) (string, error) {
			return instanceID, nil
		},
	}
}

// setExecEnv wires the global flags + injected seams for an exec test and
// returns a restore func.
func setExecEnv(t *testing.T, client api.APIClient, runner sshx.Runner, pubKeyPath string) func() {
	t.Helper()
	origClient := newAPIClient
	origRunner := execRunner
	origDiscover := execDiscoverPublicKey
	origToken := token
	origProject := project
	origEnv := environment
	origIdentity := execIdentityFile
	origInstance := execInstanceID

	token = "test-token"
	project = "my-project"
	environment = "production"
	execIdentityFile = ""
	execInstanceID = ""
	newAPIClient = func(tkn string) api.APIClient { return client }
	execRunner = runner
	execDiscoverPublicKey = func(identityFile string) (string, error) { return pubKeyPath, nil }

	return func() {
		newAPIClient = origClient
		execRunner = origRunner
		execDiscoverPublicKey = origDiscover
		token = origToken
		project = origProject
		environment = origEnv
		execIdentityFile = origIdentity
		execInstanceID = origInstance
	}
}

// writePubKey drops a valid ed25519 public key file and returns its path.
func writePubKey(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/id_ed25519.pub"
	content := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl test@example"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunExec_ProjectTokenFailsFast(t *testing.T) {
	runner := &fakeRunner{}
	var registerCalled bool
	client := &api.MockClient{
		IsProjectTokenFunc: func() (bool, error) { return true, nil },
		RegisterSSHKeyFunc: func(name, publicKey, workspaceID string) (api.SSHKey, error) {
			registerCalled = true
			return api.SSHKey{}, nil
		},
		GetServiceInstanceIDFunc: func(environmentID, serviceID string) (string, error) {
			t.Error("GetServiceInstanceID must not be called under a project token")
			return "", nil
		},
	}
	restore := setExecEnv(t, client, runner, writePubKey(t))
	defer restore()

	err := runExec(execCmd, []string{"api"})
	if err == nil {
		t.Fatal("expected a fail-fast error for a project token")
	}
	if !strings.Contains(err.Error(), "account or workspace token") {
		t.Errorf("error should explain the token requirement, got: %v", err)
	}
	if runner.called {
		t.Error("ssh must not be launched under a project token")
	}
	if registerCalled {
		t.Error("no key registration must happen under a project token")
	}
}

func TestRunExec_BuildsInteractiveArgv(t *testing.T) {
	runner := &fakeRunner{}
	client := execTestClient("instance-xyz")
	// No existing keys → registration happens, but argv is what we assert.
	restore := setExecEnv(t, client, runner, writePubKey(t))
	defer restore()

	if err := runExec(execCmd, []string{"api"}); err != nil {
		t.Fatalf("runExec error: %v", err)
	}
	if !runner.called {
		t.Fatal("expected the runner to be invoked")
	}
	joined := strings.Join(runner.gotArgv, " ")
	if !strings.Contains(joined, "instance-xyz@ssh.railway.com") {
		t.Errorf("argv missing target: %v", runner.gotArgv)
	}
	// No command → no `--` separator.
	for _, a := range runner.gotArgv {
		if a == "--" {
			t.Errorf("interactive form should not contain '--': %v", runner.gotArgv)
		}
	}
}

func TestRunExec_BuildsCommandArgv(t *testing.T) {
	runner := &fakeRunner{}
	client := execTestClient("instance-xyz")
	restore := setExecEnv(t, client, runner, writePubKey(t))
	defer restore()

	if err := runExec(execCmd, []string{"api", "ls", "-la"}); err != nil {
		t.Fatalf("runExec error: %v", err)
	}
	joined := strings.Join(runner.gotArgv, " ")
	if !strings.Contains(joined, "-- ls -la") {
		t.Errorf("expected '-- ls -la' in argv, got: %v", runner.gotArgv)
	}
	if !strings.Contains(joined, "instance-xyz@ssh.railway.com -- ls -la") {
		t.Errorf("target must precede the command: %v", runner.gotArgv)
	}
}

func TestRunExec_IdempotentKeyRegistration(t *testing.T) {
	runner := &fakeRunner{}
	client := execTestClient("instance-xyz")

	// Precompute the fingerprint of the test pubkey so ListSSHKeys reports it.
	pubPath := writePubKey(t)
	pub, _ := sshx.ReadPublicKey(pubPath)
	fp, _ := sshx.Fingerprint(pub)

	var registerCalled bool
	client.ListSSHKeysFunc = func(workspaceID string) ([]api.SSHKey, error) {
		return []api.SSHKey{{ID: "k1", Name: "existing", Fingerprint: fp}}, nil
	}
	client.RegisterSSHKeyFunc = func(name, publicKey, workspaceID string) (api.SSHKey, error) {
		registerCalled = true
		return api.SSHKey{}, nil
	}

	restore := setExecEnv(t, client, runner, pubPath)
	defer restore()

	if err := runExec(execCmd, []string{"api", "true"}); err != nil {
		t.Fatalf("runExec error: %v", err)
	}
	if registerCalled {
		t.Error("RegisterSSHKey must be skipped when the fingerprint is already registered")
	}
	if !runner.called {
		t.Error("exec should still proceed to run ssh")
	}
}

func TestRunExec_RegistersWhenAbsent(t *testing.T) {
	runner := &fakeRunner{}
	client := execTestClient("instance-xyz")

	var registerCalled bool
	client.ListSSHKeysFunc = func(workspaceID string) ([]api.SSHKey, error) {
		return []api.SSHKey{{ID: "other", Fingerprint: "SHA256:different"}}, nil
	}
	client.RegisterSSHKeyFunc = func(name, publicKey, workspaceID string) (api.SSHKey, error) {
		registerCalled = true
		if workspaceID != "ws-1" {
			t.Errorf("workspaceID = %q, want ws-1", workspaceID)
		}
		return api.SSHKey{ID: "new", Fingerprint: "SHA256:new"}, nil
	}

	restore := setExecEnv(t, client, runner, writePubKey(t))
	defer restore()

	if err := runExec(execCmd, []string{"api", "true"}); err != nil {
		t.Fatalf("runExec error: %v", err)
	}
	if !registerCalled {
		t.Error("RegisterSSHKey must be called when the fingerprint is absent")
	}
}
