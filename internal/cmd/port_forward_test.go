package cmd

import (
	"strings"
	"testing"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/sshx"
)

// setPFEnv wires the global flags + injected seams for a port-forward test and
// returns a restore func. Mirrors setExecEnv but for the port-forward globals.
func setPFEnv(t *testing.T, client api.APIClient, runner sshx.Runner, pubKeyPath string) func() {
	t.Helper()
	origClient := newAPIClient
	origRunner := pfRunner
	origDiscover := pfDiscoverPublicKey
	origToken := token
	origProject := project
	origEnv := environment
	origIdentity := pfIdentityFile
	origInstance := pfInstanceID
	origAddress := pfAddress

	token = "test-token"
	project = "my-project"
	environment = "production"
	pfIdentityFile = ""
	pfInstanceID = ""
	pfAddress = "127.0.0.1"
	newAPIClient = func(tkn string) api.APIClient { return client }
	pfRunner = runner
	pfDiscoverPublicKey = func(identityFile string) (string, error) { return pubKeyPath, nil }

	return func() {
		newAPIClient = origClient
		pfRunner = origRunner
		pfDiscoverPublicKey = origDiscover
		token = origToken
		project = origProject
		environment = origEnv
		pfIdentityFile = origIdentity
		pfInstanceID = origInstance
		pfAddress = origAddress
	}
}

func TestRunPortForward_ProjectTokenFailsFast(t *testing.T) {
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
	restore := setPFEnv(t, client, runner, writePubKey(t))
	defer restore()

	err := runPortForward(portForwardCmd, []string{"api", "5432"})
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

func TestRunPortForward_BuildsSingleForwardArgv(t *testing.T) {
	runner := &fakeRunner{}
	client := execTestClient("inst-xyz")
	restore := setPFEnv(t, client, runner, writePubKey(t))
	defer restore()

	if err := runPortForward(portForwardCmd, []string{"api", "5432"}); err != nil {
		t.Fatalf("runPortForward error: %v", err)
	}
	if !runner.called {
		t.Fatal("expected the runner to be invoked")
	}
	joined := strings.Join(runner.gotArgv, " ")
	if !strings.Contains(joined, "-L 127.0.0.1:5432:127.0.0.1:5432") {
		t.Errorf("argv missing the loopback-pinned -L forward: %v", runner.gotArgv)
	}
	if !strings.Contains(joined, "inst-xyz@ssh.railway.com") {
		t.Errorf("argv missing target: %v", runner.gotArgv)
	}
	if !strings.Contains(joined, "-N") {
		t.Errorf("forward argv must contain -N: %v", runner.gotArgv)
	}
}

func TestRunPortForward_MultipleSpecsOneInvocation(t *testing.T) {
	runner := &fakeRunner{}
	client := execTestClient("inst-xyz")
	restore := setPFEnv(t, client, runner, writePubKey(t))
	defer restore()

	if err := runPortForward(portForwardCmd, []string{"api", "5432", "6379:6379"}); err != nil {
		t.Fatalf("runPortForward error: %v", err)
	}
	// Exactly one ssh invocation carrying two -L flags.
	lCount := 0
	for _, a := range runner.gotArgv {
		if a == "-L" {
			lCount++
		}
	}
	if lCount != 2 {
		t.Errorf("expected 2 -L flags in one invocation, got %d: %v", lCount, runner.gotArgv)
	}
	joined := strings.Join(runner.gotArgv, " ")
	if !strings.Contains(joined, "127.0.0.1:5432:127.0.0.1:5432") ||
		!strings.Contains(joined, "127.0.0.1:6379:127.0.0.1:6379") {
		t.Errorf("both forwards must appear: %v", runner.gotArgv)
	}
}

func TestRunPortForward_JumpForm(t *testing.T) {
	runner := &fakeRunner{}
	client := execTestClient("jump-inst")
	restore := setPFEnv(t, client, runner, writePubKey(t))
	defer restore()

	err := runPortForward(portForwardCmd, []string{"api", "6443:kube-apiserver.railway.internal:6443"})
	if err != nil {
		t.Fatalf("runPortForward error: %v", err)
	}
	joined := strings.Join(runner.gotArgv, " ")
	if !strings.Contains(joined, "-L 127.0.0.1:6443:kube-apiserver.railway.internal:6443") {
		t.Errorf("jump form must pass the internal host verbatim: %v", runner.gotArgv)
	}
}

func TestRunPortForward_InvalidSpecFailsBeforeAPI(t *testing.T) {
	runner := &fakeRunner{}
	client := &api.MockClient{
		IsProjectTokenFunc: func() (bool, error) {
			t.Error("token gate must not be reached for a malformed spec")
			return false, nil
		},
	}
	restore := setPFEnv(t, client, runner, writePubKey(t))
	defer restore()

	err := runPortForward(portForwardCmd, []string{"api", "not-a-port"})
	if err == nil {
		t.Fatal("expected an error for a malformed port spec")
	}
	if runner.called {
		t.Error("ssh must not launch for a malformed spec")
	}
}
