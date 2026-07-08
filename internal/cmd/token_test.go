package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
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

func TestRunTokenList_JSON(t *testing.T) {
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

	called := false
	mock := tokenTestMock()
	mock.ListProjectTokensFunc = func(projectID string) ([]api.ProjectToken, error) {
		called = true
		if projectID != "proj-1" {
			t.Errorf("expected projectId proj-1, got %q", projectID)
		}
		return []api.ProjectToken{
			{ID: "t1", Name: "ci", EnvironmentID: "env-1", CreatedAt: "2026-07-01T00:00:00Z", DisplayToken: "tok-****"},
		}, nil
	}

	token = "test-token"
	project = "my-project"
	environment = ""
	outputFormat = "json"
	newAPIClient = func(tkn string) api.APIClient { return mock }

	if err := tokenListCmd.RunE(tokenListCmd, []string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected ListProjectTokens to be called")
	}
}

func TestRunTokenDelete_Success(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origToken := token
	origYes := tokenDeleteYes
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		token = origToken
		tokenDeleteYes = origYes
	}()

	var capturedID string
	mock := tokenTestMock()
	mock.ListProjectTokensFunc = func(projectID string) ([]api.ProjectToken, error) {
		return []api.ProjectToken{{ID: "t1", Name: "ci", EnvironmentID: "env-1"}}, nil
	}
	mock.DeleteProjectTokenFunc = func(tokenID string) error {
		capturedID = tokenID
		return nil
	}

	token = "test-token"
	project = "my-project"
	tokenDeleteYes = true
	newAPIClient = func(tkn string) api.APIClient { return mock }

	if err := tokenDeleteCmd.RunE(tokenDeleteCmd, []string{"t1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedID != "t1" {
		t.Errorf("expected delete of t1, got %q", capturedID)
	}
}

func TestRunTokenDelete_Cancelled(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origToken := token
	origYes := tokenDeleteYes
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		token = origToken
		tokenDeleteYes = origYes
		tokenDeleteCmd.SetIn(nil)
	}()

	deleteCalled := false
	mock := tokenTestMock()
	mock.ListProjectTokensFunc = func(projectID string) ([]api.ProjectToken, error) {
		return []api.ProjectToken{{ID: "t1", Name: "ci", EnvironmentID: "env-1"}}, nil
	}
	mock.DeleteProjectTokenFunc = func(tokenID string) error {
		deleteCalled = true
		return nil
	}

	token = "test-token"
	project = "my-project"
	tokenDeleteYes = false
	tokenDeleteCmd.SetIn(strings.NewReader("n\n"))
	newAPIClient = func(tkn string) api.APIClient { return mock }

	if err := tokenDeleteCmd.RunE(tokenDeleteCmd, []string{"t1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleteCalled {
		t.Error("expected delete to be cancelled, but DeleteProjectToken was called")
	}
}

func TestRunTokenDelete_NotFound(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origToken := token
	origYes := tokenDeleteYes
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		token = origToken
		tokenDeleteYes = origYes
	}()

	mock := tokenTestMock()
	mock.ListProjectTokensFunc = func(projectID string) ([]api.ProjectToken, error) {
		return []api.ProjectToken{{ID: "t1", Name: "ci", EnvironmentID: "env-1"}}, nil
	}

	token = "test-token"
	project = "my-project"
	tokenDeleteYes = true
	newAPIClient = func(tkn string) api.APIClient { return mock }

	if err := tokenDeleteCmd.RunE(tokenDeleteCmd, []string{"nonexistent"}); err == nil {
		t.Error("expected error for unknown token id")
	}
}

func TestRunTokenCreate_JSON(t *testing.T) {
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

	mock := tokenTestMock()
	mock.CreateProjectTokenFunc = func(projectID, environmentID, name string) (string, error) {
		return "tok-secret-value", nil
	}

	token = "test-token"
	project = "my-project"
	environment = "production"
	outputFormat = "json"
	newAPIClient = func(tkn string) api.APIClient { return mock }

	// The JSON payload is written by output.NewPrinter, which targets the real
	// os.Stdout — not cmd.OutOrStdout(). Redirect os.Stdout through a pipe so we
	// can capture it. stderr still flows through the cobra command's sink.
	var stderr bytes.Buffer
	tokenCreateCmd.SetErr(&stderr)
	defer func() { tokenCreateCmd.SetOut(nil); tokenCreateCmd.SetErr(nil) }()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	runErr := tokenCreateCmd.RunE(tokenCreateCmd, []string{"ci"})

	// Restore stdout, then close the writer and drain the pipe.
	os.Stdout = oldStdout
	if cerr := w.Close(); cerr != nil {
		t.Fatalf("closing pipe writer: %v", cerr)
	}
	out, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatalf("reading pipe: %v", readErr)
	}

	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}

	// The JSON payload (with the token) must be on stdout and parseable.
	var got struct {
		Name          string `json:"name"`
		ProjectID     string `json:"projectId"`
		EnvironmentID string `json:"environmentId"`
		Token         string `json:"token"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, string(out))
	}
	if got.Token != "tok-secret-value" || got.Name != "ci" || got.ProjectID != "proj-1" || got.EnvironmentID != "env-1" {
		t.Errorf("unexpected JSON payload: %+v", got)
	}
	// The token must never leak to stderr.
	if strings.Contains(stderr.String(), "tok-secret-value") {
		t.Errorf("stderr leaked the token value: %q", stderr.String())
	}
}

func TestFormatTokenTime(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", "-"},
		{"valid RFC3339", "2026-07-01T13:45:00Z", "2026-07-01 13:45"},
		{"invalid falls back to raw", "not-a-time", "not-a-time"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatTokenTime(tt.in); got != tt.want {
				t.Errorf("formatTokenTime(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
