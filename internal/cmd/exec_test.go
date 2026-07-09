package cmd

import (
	"context"
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

// execTestClient is a MockClient wired for a successful exec: resolves a
// project/env/service and a service instance. railctl no longer registers or
// inspects SSH keys, so no key funcs are needed.
func execTestClient(instanceID string) *api.MockClient {
	return &api.MockClient{
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
func setExecEnv(t *testing.T, client api.APIClient, runner sshx.Runner) func() {
	t.Helper()
	origClient := newAPIClient
	origRunner := execRunner
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

	return func() {
		newAPIClient = origClient
		execRunner = origRunner
		token = origToken
		project = origProject
		environment = origEnv
		execIdentityFile = origIdentity
		execInstanceID = origInstance
	}
}

// TestRunExec_ProjectTokenProceeds asserts the token gate is gone: a
// project-scoped token no longer fails fast — exec resolves the instance and
// runs ssh, because authentication is by the user's SSH key, not the token.
func TestRunExec_ProjectTokenProceeds(t *testing.T) {
	runner := &fakeRunner{}
	client := execTestClient("instance-xyz")
	// A project-scoped token: ResolveContext derives project/env from the token
	// (GetProjectContext) rather than listing. exec has NO SSH token gate, so it
	// must proceed to build argv and run ssh.
	client.IsProjectTokenFunc = func() (bool, error) { return true, nil }
	client.GetProjectContextFunc = func() (string, string, error) { return "proj-1", "env-1", nil }
	client.GetProjectFunc = func(id string) (types.Project, error) {
		return types.Project{ID: "proj-1", Name: "my-project"}, nil
	}

	restore := setExecEnv(t, client, runner)
	defer restore()

	if err := runExec(execCmd, []string{"api", "true"}); err != nil {
		t.Fatalf("runExec must not fail under a project token: %v", err)
	}
	if !runner.called {
		t.Error("exec should proceed to run ssh under a project token")
	}
	joined := strings.Join(runner.gotArgv, " ")
	if !strings.Contains(joined, "instance-xyz@ssh.railway.com") {
		t.Errorf("argv missing target: %v", runner.gotArgv)
	}
}

func TestRunExec_BuildsInteractiveArgv(t *testing.T) {
	runner := &fakeRunner{}
	client := execTestClient("instance-xyz")
	restore := setExecEnv(t, client, runner)
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
	restore := setExecEnv(t, client, runner)
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
