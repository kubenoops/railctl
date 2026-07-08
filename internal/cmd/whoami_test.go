package cmd

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/types"
)

// runWhoamiCapture runs the whoami command with the given mock and output
// format, capturing everything written to os.Stdout (the output.Printer and
// table renderers target the real os.Stdout, not cmd.OutOrStdout()).
func runWhoamiCapture(t *testing.T, mock *api.MockClient, format string) string {
	t.Helper()

	origAPIClient := newAPIClient
	origToken := token
	origOutput := outputFormat
	defer func() {
		newAPIClient = origAPIClient
		token = origToken
		outputFormat = origOutput
	}()

	token = "tok-whoami-secret-value"
	outputFormat = format
	newAPIClient = func(tkn string) api.APIClient { return mock }

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	runErr := whoamiCmd.RunE(whoamiCmd, []string{})

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
	return string(out)
}

// whoamiProjectTokenMock wires a MockClient shaped like a project-scoped
// token: pinned to proj-1/env-1 inside workspace acme.
func whoamiProjectTokenMock() *api.MockClient {
	return &api.MockClient{
		IsProjectTokenFunc: func() (bool, error) { return true, nil },
		GetProjectContextFunc: func() (string, string, error) {
			return "proj-1", "env-1", nil
		},
		TokenWorkspacesFunc: func() ([]api.Workspace, error) {
			return []api.Workspace{{ID: "ws-1", Name: "acme"}}, nil
		},
		GetProjectFunc: func(id string) (types.Project, error) {
			return types.Project{ID: id, Name: "my-project"}, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return []types.Environment{
				{ID: "env-0", Name: "staging"},
				{ID: "env-1", Name: "production"},
			}, nil
		},
	}
}

func TestRunWhoami_ProjectToken(t *testing.T) {
	out := runWhoamiCapture(t, whoamiProjectTokenMock(), "table")

	for _, want := range []string{"project", "acme", "my-project", "production"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "tok-whoami-secret-value") {
		t.Errorf("output leaked the token value:\n%s", out)
	}
}

func TestRunWhoami_WorkspaceToken(t *testing.T) {
	mock := &api.MockClient{
		IsWorkspaceTokenFunc: func() (bool, error) { return true, nil },
		TokenWorkspacesFunc: func() ([]api.Workspace, error) {
			return []api.Workspace{{ID: "ws-1", Name: "acme"}}, nil
		},
	}

	out := runWhoamiCapture(t, mock, "table")

	for _, want := range []string{"workspace", "acme"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "tok-whoami-secret-value") {
		t.Errorf("output leaked the token value:\n%s", out)
	}
}

func TestRunWhoami_AccountToken(t *testing.T) {
	mock := &api.MockClient{
		TokenWorkspacesFunc: func() ([]api.Workspace, error) {
			return []api.Workspace{
				{ID: "ws-1", Name: "acme"},
				{ID: "ws-2", Name: "globex"},
			}, nil
		},
	}

	out := runWhoamiCapture(t, mock, "table")

	for _, want := range []string{"account", "acme", "globex"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "tok-whoami-secret-value") {
		t.Errorf("output leaked the token value:\n%s", out)
	}
}

func TestRunWhoami_JSON(t *testing.T) {
	out := runWhoamiCapture(t, whoamiProjectTokenMock(), "json")

	var got struct {
		Type      string `json:"type"`
		Workspace struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"workspace"`
		Project struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"project"`
		Environment struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"environment"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, out)
	}

	if got.Type != "project" {
		t.Errorf("type = %q, want %q", got.Type, "project")
	}
	if got.Workspace.ID != "ws-1" || got.Workspace.Name != "acme" {
		t.Errorf("unexpected workspace: %+v", got.Workspace)
	}
	if got.Project.ID != "proj-1" || got.Project.Name != "my-project" {
		t.Errorf("unexpected project: %+v", got.Project)
	}
	if got.Environment.ID != "env-1" || got.Environment.Name != "production" {
		t.Errorf("unexpected environment: %+v", got.Environment)
	}
	if strings.Contains(out, "tok-whoami-secret-value") {
		t.Errorf("output leaked the token value:\n%s", out)
	}
}
