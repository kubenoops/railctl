package cmd

import (
	"strings"
	"testing"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/types"
)

// logsSyntaxMock returns a mock where log fetching succeeds for service "api"
// and records the service the command resolved.
func logsSyntaxMock(resolved *string) *api.MockClient {
	return &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{{ID: "p1", Name: "my-project"}}, nil
		},
		ListEnvironmentsFunc: func(string) ([]types.Environment, error) {
			return []types.Environment{{ID: "e1", Name: "production"}}, nil
		},
		ListServicesFunc: func(string, string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{{ID: "s1", Name: "api"}}, nil
		},
		ListDeploymentsFunc: func(_, _, serviceID string, _ int) ([]api.Deployment, error) {
			*resolved = serviceID
			return []api.Deployment{{ID: "d1", Status: "SUCCESS"}}, nil
		},
		GetDeploymentLogsFunc: func(string, int) ([]api.LogEntry, error) {
			return []api.LogEntry{}, nil
		},
	}
}

// TestLogs_BothSyntaxes: `logs <service>` is canonical; `logs service <service>`
// is tolerated for compatibility with the previously documented form.
func TestLogs_BothSyntaxes(t *testing.T) {
	origAPIClient, origProject, origEnvironment, origToken := newAPIClient, project, environment, token
	defer func() {
		newAPIClient, project, environment, token = origAPIClient, origProject, origEnvironment, origToken
	}()
	token, project, environment = "test-token", "my-project", "production"

	for _, args := range [][]string{{"api"}, {"service", "api"}} {
		var resolved string
		mock := logsSyntaxMock(&resolved)
		newAPIClient = func(string) api.APIClient { return mock }
		if err := logsServiceCmd.RunE(logsServiceCmd, args); err != nil {
			t.Fatalf("args %v: unexpected error: %v", args, err)
		}
		if resolved != "s1" {
			t.Errorf("args %v: resolved service = %q, want s1", args, resolved)
		}
	}

	// Two args where the first is not "service" is a clear mistake.
	var resolved string
	mock := logsSyntaxMock(&resolved)
	newAPIClient = func(string) api.APIClient { return mock }
	err := logsServiceCmd.RunE(logsServiceCmd, []string{"api", "extra"})
	if err == nil || !strings.Contains(err.Error(), "usage: railctl logs <service>") {
		t.Errorf("bogus two-arg form: err = %v, want usage error", err)
	}
}
