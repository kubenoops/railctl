package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/types"
)

// tokenTestMock returns a MockClient wired with one project ("my-project")
// and one environment ("production"). IsProjectToken defaults to false.
func tokenTestMock() *api.MockClient {
	return &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return []types.Environment{{ID: "env-1", Name: "production"}}, nil
		},
	}
}

func TestRunTokenCreate_Success(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	origOutput := outputFormat
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
		outputFormat = origOutput
	}()

	var capProject, capEnv, capName string
	mock := tokenTestMock()
	mock.CreateProjectTokenFunc = func(projectID, environmentID, name string) (string, error) {
		capProject, capEnv, capName = projectID, environmentID, name
		return "tok-secret-value", nil
	}

	token = "test-token"
	project = "my-project"
	environment = "production"
	outputFormat = "table"
	newAPIClient = func(tkn string) api.APIClient { return mock }

	var stdout, stderr bytes.Buffer
	tokenCreateCmd.SetOut(&stdout)
	tokenCreateCmd.SetErr(&stderr)
	defer func() { tokenCreateCmd.SetOut(nil); tokenCreateCmd.SetErr(nil) }()

	if err := tokenCreateCmd.RunE(tokenCreateCmd, []string{"ci"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capProject != "proj-1" || capEnv != "env-1" || capName != "ci" {
		t.Errorf("unexpected mint args: project=%q env=%q name=%q", capProject, capEnv, capName)
	}
	if strings.TrimSpace(stdout.String()) != "tok-secret-value" {
		t.Errorf("stdout = %q, want just the token", stdout.String())
	}
	if !strings.Contains(stderr.String(), "will not be shown again") {
		t.Errorf("stderr missing the store-now note: %q", stderr.String())
	}
	if strings.Contains(stderr.String(), "tok-secret-value") {
		t.Errorf("stderr leaked the token value: %q", stderr.String())
	}
}

func TestRunTokenCreate_ProjectTokenRejected(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origToken := token
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		token = origToken
	}()

	minted := false
	mock := tokenTestMock()
	mock.IsProjectTokenFunc = func() (bool, error) { return true, nil }
	mock.CreateProjectTokenFunc = func(projectID, environmentID, name string) (string, error) {
		minted = true
		return "should-not-happen", nil
	}

	token = "test-token"
	project = "my-project"
	newAPIClient = func(tkn string) api.APIClient { return mock }

	err := tokenCreateCmd.RunE(tokenCreateCmd, []string{"ci"})
	if err == nil {
		t.Fatal("expected an error when run with a project-scoped token")
	}
	if !strings.Contains(err.Error(), "account or workspace token") {
		t.Errorf("error = %q, want it to mention 'account or workspace token'", err.Error())
	}
	if minted {
		t.Error("CreateProjectToken must not be called when using a project token")
	}
}
