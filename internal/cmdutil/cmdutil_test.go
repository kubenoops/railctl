package cmdutil

import (
	"errors"
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
