package cmdutil

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/output"
	"github.com/kubenoops/railctl/internal/types"
)

func TestResolveContext_MissingProject(t *testing.T) {
	mock := &api.MockClient{}
	_, err := ResolveContext(mock, ResolveOpts{})
	if err == nil {
		t.Fatal("expected error for missing project")
	}
}

func TestResolveContext_ProjectOnly(t *testing.T) {
	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{{ID: "proj-1", Name: "my-app"}}, nil
		},
	}

	ctx, err := ResolveContext(mock, ResolveOpts{ProjectName: "my-app"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Project.ID != "proj-1" {
		t.Errorf("expected project ID 'proj-1', got %q", ctx.Project.ID)
	}
}

func TestResolveContext_ListProjectsError(t *testing.T) {
	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return nil, errors.New("network error")
		},
	}

	_, err := ResolveContext(mock, ResolveOpts{ProjectName: "my-app"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestResolveContext_ProjectNotFound(t *testing.T) {
	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{{ID: "proj-1", Name: "other"}}, nil
		},
	}

	_, err := ResolveContext(mock, ResolveOpts{ProjectName: "nonexistent"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestResolveContext_ProjectNotFound_ListsAvailable(t *testing.T) {
	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{
				{ID: "proj-1", Name: "api"},
				{ID: "proj-2", Name: "web"},
				{ID: "proj-3", Name: "lingo-deployment"},
			}, nil
		},
	}

	_, err := ResolveContext(mock, ResolveOpts{ProjectName: "nonexistent"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	want := "project 'nonexistent' not found — available: api, web, lingo-deployment"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestResolveContext_EnvironmentNotFound_ListsAvailableAndProject(t *testing.T) {
	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{{ID: "proj-1", Name: "my-app"}}, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return []types.Environment{
				{ID: "env-1", Name: "production"},
				{ID: "env-2", Name: "staging"},
			}, nil
		},
	}

	_, err := ResolveContext(mock, ResolveOpts{
		ProjectName:     "my-app",
		EnvironmentName: "nonexistent",
		NeedEnvironment: true,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	want := "environment 'nonexistent' not found in project 'my-app' — available: production, staging"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestResolveContext_ServiceNotFound_ListsAvailableAndEnvironment(t *testing.T) {
	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{{ID: "proj-1", Name: "my-app"}}, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return []types.Environment{{ID: "env-1", Name: "production"}}, nil
		},
		ListServicesFunc: func(projectID, environmentID string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{
				{ID: "svc-1", Name: "api"},
				{ID: "svc-2", Name: "worker"},
			}, nil
		},
	}

	_, err := ResolveContext(mock, ResolveOpts{
		ProjectName:     "my-app",
		EnvironmentName: "production",
		ServiceName:     "nonexistent",
		NeedEnvironment: true,
		NeedService:     true,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	want := "service 'nonexistent' not found in environment 'production' — available: api, worker"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestResolveContext_WithEnvironment(t *testing.T) {
	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{{ID: "proj-1", Name: "my-app"}}, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return []types.Environment{{ID: "env-1", Name: "production"}}, nil
		},
	}

	ctx, err := ResolveContext(mock, ResolveOpts{
		ProjectName:     "my-app",
		EnvironmentName: "production",
		NeedEnvironment: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Environment.ID != "env-1" {
		t.Errorf("expected env ID 'env-1', got %q", ctx.Environment.ID)
	}
}

func TestResolveContext_MissingEnvironmentName(t *testing.T) {
	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{{ID: "proj-1", Name: "my-app"}}, nil
		},
	}

	_, err := ResolveContext(mock, ResolveOpts{
		ProjectName:     "my-app",
		NeedEnvironment: true,
	})
	if err == nil {
		t.Fatal("expected error for missing environment name")
	}
}

func TestResolveContext_ListEnvironmentsError(t *testing.T) {
	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{{ID: "proj-1", Name: "my-app"}}, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return nil, errors.New("env error")
		},
	}

	_, err := ResolveContext(mock, ResolveOpts{
		ProjectName:     "my-app",
		EnvironmentName: "production",
		NeedEnvironment: true,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestResolveContext_EnvironmentNotFound(t *testing.T) {
	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{{ID: "proj-1", Name: "my-app"}}, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return []types.Environment{{ID: "env-1", Name: "staging"}}, nil
		},
	}

	_, err := ResolveContext(mock, ResolveOpts{
		ProjectName:     "my-app",
		EnvironmentName: "nonexistent",
		NeedEnvironment: true,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestResolveContext_WithService(t *testing.T) {
	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{{ID: "proj-1", Name: "my-app"}}, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return []types.Environment{{ID: "env-1", Name: "production"}}, nil
		},
		ListServicesFunc: func(projectID, environmentID string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{{ID: "svc-1", Name: "web"}}, nil
		},
	}

	ctx, err := ResolveContext(mock, ResolveOpts{
		ProjectName:     "my-app",
		EnvironmentName: "production",
		ServiceName:     "web",
		NeedEnvironment: true,
		NeedService:     true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Service == nil {
		t.Fatal("expected service to be set")
	}
	if ctx.Service.ID != "svc-1" {
		t.Errorf("expected service ID 'svc-1', got %q", ctx.Service.ID)
	}
}

func TestResolveContext_MissingServiceName(t *testing.T) {
	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{{ID: "proj-1", Name: "my-app"}}, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return []types.Environment{{ID: "env-1", Name: "production"}}, nil
		},
	}

	_, err := ResolveContext(mock, ResolveOpts{
		ProjectName:     "my-app",
		EnvironmentName: "production",
		NeedEnvironment: true,
		NeedService:     true,
	})
	if err == nil {
		t.Fatal("expected error for missing service name")
	}
}

func TestResolveContext_ListServicesError(t *testing.T) {
	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{{ID: "proj-1", Name: "my-app"}}, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return []types.Environment{{ID: "env-1", Name: "production"}}, nil
		},
		ListServicesFunc: func(projectID, environmentID string) ([]types.ServiceDetail, error) {
			return nil, errors.New("services error")
		},
	}

	_, err := ResolveContext(mock, ResolveOpts{
		ProjectName:     "my-app",
		EnvironmentName: "production",
		ServiceName:     "web",
		NeedEnvironment: true,
		NeedService:     true,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestResolveContext_ServiceNotFound(t *testing.T) {
	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{{ID: "proj-1", Name: "my-app"}}, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return []types.Environment{{ID: "env-1", Name: "production"}}, nil
		},
		ListServicesFunc: func(projectID, environmentID string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{{ID: "svc-1", Name: "api"}}, nil
		},
	}

	_, err := ResolveContext(mock, ResolveOpts{
		ProjectName:     "my-app",
		EnvironmentName: "production",
		ServiceName:     "nonexistent",
		NeedEnvironment: true,
		NeedService:     true,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// projectTokenMock returns a MockClient simulating a project token baked to
// project proj-1 ("my-app") and environment env-1 ("production"). The
// project has a second environment (env-2, "staging") the token is NOT
// scoped to.
func projectTokenMock() *api.MockClient {
	return &api.MockClient{
		IsProjectTokenFunc:    func() (bool, error) { return true, nil },
		GetProjectContextFunc: func() (string, string, error) { return "proj-1", "env-1", nil },
		GetProjectFunc: func(id string) (types.Project, error) {
			return types.Project{ID: "proj-1", Name: "my-app"}, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return []types.Environment{
				{ID: "env-1", Name: "production"},
				{ID: "env-2", Name: "staging"},
			}, nil
		},
	}
}

// captureStderr runs fn while capturing everything written to os.Stderr.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = old }()
	fn()
	w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestResolveContext_ProjectToken_MatchingProjectFlag(t *testing.T) {
	for _, flag := range []string{"my-app", "proj-1", "my-a"} {
		t.Run(flag, func(t *testing.T) {
			var ctx *Context
			var err error
			stderr := captureStderr(t, func() {
				ctx, err = ResolveContext(projectTokenMock(), ResolveOpts{ProjectName: flag})
			})
			if err != nil {
				t.Fatalf("unexpected error for matching -p %q: %v", flag, err)
			}
			if ctx.Project.ID != "proj-1" {
				t.Errorf("expected project ID 'proj-1', got %q", ctx.Project.ID)
			}
			if stderr != "" {
				t.Errorf("expected no warning output for matching -p, got: %q", stderr)
			}
		})
	}
}

func TestResolveContext_ProjectToken_MismatchedProjectFlag(t *testing.T) {
	_, err := ResolveContext(projectTokenMock(), ResolveOpts{ProjectName: "other-app"})
	if err == nil {
		t.Fatal("expected contradiction error for mismatched -p, got nil")
	}
	for _, want := range []string{"scoped to project", "my-app", "proj-1", "other-app"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err.Error(), want)
		}
	}
}

func TestResolveContext_ProjectToken_MatchingEnvironmentFlag(t *testing.T) {
	for _, flag := range []string{"production", "env-1", "prod"} {
		t.Run(flag, func(t *testing.T) {
			var ctx *Context
			var err error
			stderr := captureStderr(t, func() {
				ctx, err = ResolveContext(projectTokenMock(), ResolveOpts{
					EnvironmentName: flag,
					NeedEnvironment: true,
				})
			})
			if err != nil {
				t.Fatalf("unexpected error for matching -e %q: %v", flag, err)
			}
			if ctx.Environment.ID != "env-1" {
				t.Errorf("expected environment ID 'env-1', got %q", ctx.Environment.ID)
			}
			if stderr != "" {
				t.Errorf("expected no warning output for matching -e, got: %q", stderr)
			}
		})
	}
}

func TestResolveContext_ProjectToken_MismatchedEnvironmentFlag(t *testing.T) {
	// "staging" exists in the project but is NOT the environment the token
	// is scoped to — still a contradiction.
	_, err := ResolveContext(projectTokenMock(), ResolveOpts{
		EnvironmentName: "staging",
		NeedEnvironment: true,
	})
	if err == nil {
		t.Fatal("expected contradiction error for mismatched -e, got nil")
	}
	for _, want := range []string{"scoped to environment", "production", "env-1", "staging"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err.Error(), want)
		}
	}
}

func TestResolveContext_ProjectToken_EnvironmentFlagWithoutNeedEnvironment(t *testing.T) {
	// No environment target to contradict: a stray -e keeps being ignored.
	var err error
	stderr := captureStderr(t, func() {
		_, err = ResolveContext(projectTokenMock(), ResolveOpts{EnvironmentName: "some-other-env"})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(stderr, "scoped to environment") {
		t.Errorf("expected no contradiction error output, got: %q", stderr)
	}
}

func TestResolveContext_ProjectToken_SingleEnvironmentListing(t *testing.T) {
	// The contradiction check's environment listing is reused by the
	// resolution step — exactly one ListEnvironments call.
	mock := projectTokenMock()
	calls := 0
	inner := mock.ListEnvironmentsFunc
	mock.ListEnvironmentsFunc = func(projectID string) ([]types.Environment, error) {
		calls++
		return inner(projectID)
	}
	_, err := ResolveContext(mock, ResolveOpts{
		EnvironmentName: "production",
		NeedEnvironment: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected exactly 1 ListEnvironments call, got %d", calls)
	}
}

func TestPrintResult_JSON(t *testing.T) {
	data := map[string]string{"key": "value"}
	table := output.NewTable("K", "V")
	err := PrintResult(output.FormatJSON, data, table, nil, "empty")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPrintResult_YAML(t *testing.T) {
	data := map[string]string{"key": "value"}
	table := output.NewTable("K", "V")
	err := PrintResult(output.FormatYAML, data, table, nil, "empty")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPrintResult_Table(t *testing.T) {
	table := output.NewTable("NAME", "VALUE")
	table.AddRow("a", "b")
	err := PrintResult(output.FormatTable, nil, table, nil, "No resources found.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPrintResult_TableEmpty(t *testing.T) {
	table := output.NewTable("NAME", "VALUE")
	err := PrintResult(output.FormatTable, nil, table, nil, "No resources found.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPrintResult_Wide(t *testing.T) {
	table := output.NewTable("NAME")
	table.AddRow("a")
	wide := output.NewTable("NAME", "ID")
	wide.AddRow("a", "1")
	err := PrintResult(output.FormatWide, nil, table, wide, "empty")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPrintResult_WideNoWideTable(t *testing.T) {
	table := output.NewTable("NAME")
	table.AddRow("a")
	err := PrintResult(output.FormatWide, nil, table, nil, "empty")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPrintResult_WideEmpty(t *testing.T) {
	table := output.NewTable("NAME")
	wide := output.NewTable("NAME", "ID")
	err := PrintResult(output.FormatWide, nil, table, wide, "No resources found.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
