package cmd

import (
	"strings"
	"testing"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/sshx"
	"github.com/kubenoops/railctl/internal/types"
)

// setPFEnv wires the global flags + injected seams for a port-forward test and
// returns a restore func. Mirrors setExecEnv but for the port-forward globals.
func setPFEnv(t *testing.T, client api.APIClient, runner sshx.Runner) func() {
	t.Helper()
	origClient := newAPIClient
	origRunner := pfRunner
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

	return func() {
		newAPIClient = origClient
		pfRunner = origRunner
		token = origToken
		project = origProject
		environment = origEnv
		pfIdentityFile = origIdentity
		pfInstanceID = origInstance
		pfAddress = origAddress
	}
}

// TestRunPortForward_ProjectTokenProceeds asserts the token gate is gone: a
// project-scoped token no longer fails fast — port-forward resolves the
// instance and runs ssh, because authentication is by the user's SSH key.
func TestRunPortForward_ProjectTokenProceeds(t *testing.T) {
	runner := &fakeRunner{}
	client := execTestClient("inst-xyz")
	// A project-scoped token: ResolveContext derives project/env from the token
	// (GetProjectContext). port-forward has NO SSH token gate, so it must
	// proceed to build argv and run ssh.
	client.IsProjectTokenFunc = func() (bool, error) { return true, nil }
	client.GetProjectContextFunc = func() (string, string, error) { return "proj-1", "env-1", nil }
	client.GetProjectFunc = func(id string) (types.Project, error) {
		return types.Project{ID: "proj-1", Name: "my-project"}, nil
	}

	restore := setPFEnv(t, client, runner)
	defer restore()

	if err := runPortForward(portForwardCmd, []string{"api", "5432"}); err != nil {
		t.Fatalf("runPortForward must not fail under a project token: %v", err)
	}
	if !runner.called {
		t.Error("port-forward should proceed to run ssh under a project token")
	}
	joined := strings.Join(runner.gotArgv, " ")
	if !strings.Contains(joined, "inst-xyz@ssh.railway.com") {
		t.Errorf("argv missing target: %v", runner.gotArgv)
	}
}

func TestRunPortForward_BuildsSingleForwardArgv(t *testing.T) {
	runner := &fakeRunner{}
	client := execTestClient("inst-xyz")
	restore := setPFEnv(t, client, runner)
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
	restore := setPFEnv(t, client, runner)
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

func TestRunPortForward_ThreeFieldRejected(t *testing.T) {
	runner := &fakeRunner{}
	client := execTestClient("api-inst")
	restore := setPFEnv(t, client, runner)
	defer restore()

	// The three-field jump form is unsupported (Railway's relay only forwards
	// to the target's own loopback) — it must fail before launching ssh.
	err := runPortForward(portForwardCmd, []string{"api", "6443:kube-apiserver.railway.internal:6443"})
	if err == nil {
		t.Fatal("expected the three-field spec to be rejected")
	}
	if len(runner.gotArgv) != 0 {
		t.Errorf("ssh must not be launched on a bad spec: %v", runner.gotArgv)
	}
}

func TestRunPortForward_InvalidSpecFailsBeforeAPI(t *testing.T) {
	runner := &fakeRunner{}
	client := &api.MockClient{
		GetServiceInstanceIDFunc: func(environmentID, serviceID string) (string, error) {
			t.Error("instance resolution must not be reached for a malformed spec")
			return "", nil
		},
	}
	restore := setPFEnv(t, client, runner)
	defer restore()

	err := runPortForward(portForwardCmd, []string{"api", "not-a-port"})
	if err == nil {
		t.Fatal("expected an error for a malformed port spec")
	}
	if runner.called {
		t.Error("ssh must not launch for a malformed spec")
	}
}
