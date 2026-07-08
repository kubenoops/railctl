package cmd

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/types"
)

// Helper to set up mock client for tests
func withMockClient(mock *api.MockClient, fn func()) {
	original := newAPIClient
	newAPIClient = func(tkn string) api.APIClient {
		return mock
	}
	defer func() { newAPIClient = original }()
	fn()
}

func TestGetToken_WithFlag(t *testing.T) {
	originalToken := token
	defer func() { token = originalToken }()

	token = "test-token-from-flag"
	result, err := getToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "test-token-from-flag" {
		t.Errorf("expected 'test-token-from-flag', got %q", result)
	}
}

func TestGetToken_WithEnvVar(t *testing.T) {
	originalToken := token
	originalEnv := os.Getenv("RAILWAY_TOKEN")
	defer func() {
		token = originalToken
		os.Setenv("RAILWAY_TOKEN", originalEnv)
	}()

	token = ""
	os.Setenv("RAILWAY_TOKEN", "test-token-from-env")

	result, err := getToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "test-token-from-env" {
		t.Errorf("expected 'test-token-from-env', got %q", result)
	}
}

func TestGetToken_NoToken(t *testing.T) {
	originalToken := token
	originalEnv := os.Getenv("RAILWAY_TOKEN")
	defer func() {
		token = originalToken
		os.Setenv("RAILWAY_TOKEN", originalEnv)
	}()

	token = ""
	os.Unsetenv("RAILWAY_TOKEN")

	_, err := getToken()
	if err == nil {
		t.Error("expected error when no token provided")
	}
}

func TestGetProject_WithFlag(t *testing.T) {
	originalProject := project
	defer func() { project = originalProject }()

	project = "my-project"
	result := getProject()
	if result != "my-project" {
		t.Errorf("expected 'my-project', got %q", result)
	}
}

func TestGetProject_WithEnvVar(t *testing.T) {
	originalProject := project
	originalEnv := os.Getenv("RAILCTL_PROJECT")
	defer func() {
		project = originalProject
		os.Setenv("RAILCTL_PROJECT", originalEnv)
	}()

	project = ""
	os.Setenv("RAILCTL_PROJECT", "env-project")

	result := getProject()
	if result != "env-project" {
		t.Errorf("expected 'env-project', got %q", result)
	}
}

func TestGetEnvironment_WithFlag(t *testing.T) {
	originalEnv := environment
	defer func() { environment = originalEnv }()

	environment = "production"
	result := getEnvironment()
	if result != "production" {
		t.Errorf("expected 'production', got %q", result)
	}
}

func TestGetEnvironment_WithEnvVar(t *testing.T) {
	originalEnv := environment
	originalEnvVar := os.Getenv("RAILCTL_ENVIRONMENT")
	defer func() {
		environment = originalEnv
		os.Setenv("RAILCTL_ENVIRONMENT", originalEnvVar)
	}()

	environment = ""
	os.Setenv("RAILCTL_ENVIRONMENT", "staging")

	result := getEnvironment()
	if result != "staging" {
		t.Errorf("expected 'staging', got %q", result)
	}
}

func TestGetService_WithFlag(t *testing.T) {
	originalSvc := service
	defer func() { service = originalSvc }()

	service = "api"
	result := getService()
	if result != "api" {
		t.Errorf("expected 'api', got %q", result)
	}
}

func TestGetService_WithEnvVar(t *testing.T) {
	originalSvc := service
	originalEnvVar := os.Getenv("RAILCTL_SERVICE")
	defer func() {
		service = originalSvc
		os.Setenv("RAILCTL_SERVICE", originalEnvVar)
	}()

	service = ""
	os.Setenv("RAILCTL_SERVICE", "worker")

	result := getService()
	if result != "worker" {
		t.Errorf("expected 'worker', got %q", result)
	}
}

func TestRootCmd_Exists(t *testing.T) {
	if rootCmd == nil {
		t.Error("rootCmd is nil")
	}
	if rootCmd.Use != "railctl" {
		t.Errorf("expected Use 'railctl', got %q", rootCmd.Use)
	}
}

func TestRootCmd_HasSubcommands(t *testing.T) {
	subcommands := rootCmd.Commands()
	if len(subcommands) < 3 {
		t.Errorf("expected at least 3 subcommands (get, describe, create), got %d", len(subcommands))
	}

	expectedCmds := []string{"get", "describe", "create", "delete"}
	for _, expected := range expectedCmds {
		found := false
		for _, cmd := range subcommands {
			if cmd.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected subcommand %q not found", expected)
		}
	}
}

// Tests for runGetProjects with mock client
func TestRunGetProjects_Success(t *testing.T) {
	// Save original values
	originalToken := token
	originalFormat := outputFormat
	defer func() {
		token = originalToken
		outputFormat = originalFormat
	}()

	token = "test-token"
	outputFormat = "table"

	mockProjects := []types.Project{
		{ID: "proj-1", Name: "my-app", UpdatedAt: time.Now()},
		{ID: "proj-2", Name: "other-app", UpdatedAt: time.Now()},
	}

	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return mockProjects, nil
		},
	}

	withMockClient(mock, func() {
		err := runGetProjects(nil, nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestRunGetProjects_APIError(t *testing.T) {
	originalToken := token
	defer func() { token = originalToken }()

	token = "test-token"

	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return nil, errors.New("API error")
		},
	}

	withMockClient(mock, func() {
		err := runGetProjects(nil, nil)
		if err == nil {
			t.Error("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "API error") {
			t.Errorf("expected 'API error', got %q", err.Error())
		}
	})
}

// Tests for runGetEnvironments with mock client
func TestRunGetEnvironments_Success(t *testing.T) {
	originalToken := token
	originalProject := project
	originalFormat := outputFormat
	defer func() {
		token = originalToken
		project = originalProject
		outputFormat = originalFormat
	}()

	token = "test-token"
	project = "my-app"
	outputFormat = "table"

	mockProjects := []types.Project{
		{ID: "proj-1", Name: "my-app"},
	}
	mockEnvs := []types.Environment{
		{ID: "env-1", Name: "production", ServiceCount: 2},
		{ID: "env-2", Name: "staging", ServiceCount: 0},
	}

	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return mockProjects, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			if projectID != "proj-1" {
				t.Errorf("expected projectID 'proj-1', got %q", projectID)
			}
			return mockEnvs, nil
		},
	}

	withMockClient(mock, func() {
		err := runGetEnvironments(nil, nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestRunGetEnvironments_NoProject(t *testing.T) {
	originalToken := token
	originalProject := project
	defer func() {
		token = originalToken
		project = originalProject
	}()

	token = "test-token"
	project = ""
	os.Unsetenv("RAILCTL_PROJECT")

	mock := &api.MockClient{}

	withMockClient(mock, func() {
		err := runGetEnvironments(nil, nil)
		if err == nil {
			t.Error("expected error for missing project")
		}
		if !strings.Contains(err.Error(), "project is required") {
			t.Errorf("expected 'project is required' error, got %q", err.Error())
		}
	})
}

// Tests for runCreateProject with mock client
func TestRunCreateProject_Success(t *testing.T) {
	originalToken := token
	defer func() { token = originalToken }()

	token = "test-token"

	mock := &api.MockClient{
		CreateProjectFunc: func(name string) (types.Project, error) {
			if name != "new-project" {
				t.Errorf("expected name 'new-project', got %q", name)
			}
			return types.Project{
				ID:   "proj-new",
				Name: "new-project",
				Environments: []types.Environment{
					{ID: "env-1", Name: "production"},
				},
			}, nil
		},
	}

	withMockClient(mock, func() {
		err := runCreateProject(nil, []string{"new-project"})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestRunCreateProject_APIError(t *testing.T) {
	originalToken := token
	defer func() { token = originalToken }()

	token = "test-token"

	mock := &api.MockClient{
		CreateProjectFunc: func(name string) (types.Project, error) {
			return types.Project{}, errors.New("plan limit reached")
		},
	}

	withMockClient(mock, func() {
		err := runCreateProject(nil, []string{"new-project"})
		if err == nil {
			t.Error("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "plan limit") {
			t.Errorf("expected 'plan limit' error, got %q", err.Error())
		}
	})
}

// Tests for runDeleteProject with mock client
func TestRunDeleteProject_ProjectNotFound(t *testing.T) {
	originalToken := token
	defer func() { token = originalToken }()

	token = "test-token"

	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{
				{ID: "proj-1", Name: "my-app"},
			}, nil
		},
	}

	withMockClient(mock, func() {
		err := runDeleteProject(nil, []string{"nonexistent"})
		if err == nil {
			t.Error("expected error for nonexistent project")
		}
	})
}

// Tests for runDescribeProject with mock client
func TestRunDescribeProject_Success(t *testing.T) {
	originalToken := token
	originalFormat := outputFormat
	defer func() {
		token = originalToken
		outputFormat = originalFormat
	}()

	token = "test-token"
	outputFormat = "table"

	mockProjects := []types.Project{
		{ID: "proj-1", Name: "my-app"},
	}
	mockProject := types.Project{
		ID:   "proj-1",
		Name: "my-app",
		Environments: []types.Environment{
			{ID: "env-1", Name: "production"},
		},
		Services: []types.Service{
			{ID: "svc-1", Name: "api"},
		},
	}

	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return mockProjects, nil
		},
		GetProjectFunc: func(id string) (types.Project, error) {
			if id != "proj-1" {
				t.Errorf("expected id 'proj-1', got %q", id)
			}
			return mockProject, nil
		},
	}

	withMockClient(mock, func() {
		err := runDescribeProject(nil, []string{"my-app"})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestRunDescribeProject_NotFound(t *testing.T) {
	originalToken := token
	defer func() { token = originalToken }()

	token = "test-token"

	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{}, nil
		},
	}

	withMockClient(mock, func() {
		err := runDescribeProject(nil, []string{"nonexistent"})
		if err == nil {
			t.Error("expected error for nonexistent project")
		}
	})
}

// Tests for runDescribeEnvironment with mock client
func TestRunDescribeEnvironment_Success(t *testing.T) {
	originalToken := token
	originalProject := project
	originalFormat := outputFormat
	defer func() {
		token = originalToken
		project = originalProject
		outputFormat = originalFormat
	}()

	token = "test-token"
	project = "my-app"
	outputFormat = "table"

	mockProjects := []types.Project{
		{ID: "proj-1", Name: "my-app"},
	}
	mockEnvs := []types.Environment{
		{ID: "env-1", Name: "production", ServiceCount: 2, UpdatedAt: time.Now()},
	}

	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return mockProjects, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return mockEnvs, nil
		},
	}

	withMockClient(mock, func() {
		err := runDescribeEnvironment(nil, []string{"production"})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// Tests for runCreateEnvironment with mock client
func TestRunCreateEnvironment_Success(t *testing.T) {
	originalToken := token
	originalProject := project
	defer func() {
		token = originalToken
		project = originalProject
	}()

	token = "test-token"
	project = "my-app"

	mockProjects := []types.Project{
		{ID: "proj-1", Name: "my-app"},
	}

	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return mockProjects, nil
		},
		CreateEnvironmentFunc: func(projectID, name string) (types.Environment, error) {
			if projectID != "proj-1" {
				t.Errorf("expected projectID 'proj-1', got %q", projectID)
			}
			if name != "staging" {
				t.Errorf("expected name 'staging', got %q", name)
			}
			return types.Environment{ID: "env-new", Name: "staging"}, nil
		},
	}

	withMockClient(mock, func() {
		err := runCreateEnvironment(nil, []string{"staging"})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// Tests for runDeleteEnvironment with mock client
func TestRunDeleteEnvironment_LastEnvGuard(t *testing.T) {
	originalToken := token
	originalProject := project
	defer func() {
		token = originalToken
		project = originalProject
	}()

	token = "test-token"
	project = "my-app"

	mockProjects := []types.Project{
		{ID: "proj-1", Name: "my-app"},
	}
	// Only one environment - should be blocked
	mockEnvs := []types.Environment{
		{ID: "env-1", Name: "production"},
	}

	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return mockProjects, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return mockEnvs, nil
		},
	}

	withMockClient(mock, func() {
		// Note: We can't easily test the interactive confirmation here,
		// but we can test that it rejects deleting the last environment
		err := runDeleteEnvironment(nil, []string{"production"})
		if err == nil {
			t.Error("expected error for last environment guard")
		}
		if !strings.Contains(err.Error(), "last environment") {
			t.Errorf("expected 'last environment' error, got %q", err.Error())
		}
	})
}

// Tests for different output formats
func TestRunGetProjects_JSONFormat(t *testing.T) {
	originalToken := token
	originalFormat := outputFormat
	defer func() {
		token = originalToken
		outputFormat = originalFormat
	}()

	token = "test-token"
	outputFormat = "json"

	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{
				{ID: "proj-1", Name: "my-app", UpdatedAt: time.Now()},
			}, nil
		},
	}

	withMockClient(mock, func() {
		err := runGetProjects(nil, nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestRunGetProjects_YAMLFormat(t *testing.T) {
	originalToken := token
	originalFormat := outputFormat
	defer func() {
		token = originalToken
		outputFormat = originalFormat
	}()

	token = "test-token"
	outputFormat = "yaml"

	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{
				{ID: "proj-1", Name: "my-app"},
			}, nil
		},
	}

	withMockClient(mock, func() {
		err := runGetProjects(nil, nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestRunGetProjects_WideFormat(t *testing.T) {
	originalToken := token
	originalFormat := outputFormat
	defer func() {
		token = originalToken
		outputFormat = originalFormat
	}()

	token = "test-token"
	outputFormat = "wide"

	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{
				{
					ID:   "proj-1",
					Name: "my-app",
					Environments: []types.Environment{
						{ID: "env-1", Name: "production"},
					},
					Services: []types.Service{
						{ID: "svc-1", Name: "api"},
					},
				},
			}, nil
		},
	}

	withMockClient(mock, func() {
		err := runGetProjects(nil, nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestRunGetEnvironments_JSONFormat(t *testing.T) {
	originalToken := token
	originalProject := project
	originalFormat := outputFormat
	defer func() {
		token = originalToken
		project = originalProject
		outputFormat = originalFormat
	}()

	token = "test-token"
	project = "my-app"
	outputFormat = "json"

	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{{ID: "proj-1", Name: "my-app"}}, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return []types.Environment{
				{ID: "env-1", Name: "production", ServiceCount: 2},
			}, nil
		},
	}

	withMockClient(mock, func() {
		err := runGetEnvironments(nil, nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestRunGetEnvironments_YAMLFormat(t *testing.T) {
	originalToken := token
	originalProject := project
	originalFormat := outputFormat
	defer func() {
		token = originalToken
		project = originalProject
		outputFormat = originalFormat
	}()

	token = "test-token"
	project = "my-app"
	outputFormat = "yaml"

	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{{ID: "proj-1", Name: "my-app"}}, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return []types.Environment{
				{ID: "env-1", Name: "production"},
			}, nil
		},
	}

	withMockClient(mock, func() {
		err := runGetEnvironments(nil, nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestRunDescribeProject_JSONFormat(t *testing.T) {
	originalToken := token
	originalFormat := outputFormat
	defer func() {
		token = originalToken
		outputFormat = originalFormat
	}()

	token = "test-token"
	outputFormat = "json"

	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{{ID: "proj-1", Name: "my-app"}}, nil
		},
		GetProjectFunc: func(id string) (types.Project, error) {
			return types.Project{
				ID:   "proj-1",
				Name: "my-app",
				Environments: []types.Environment{
					{ID: "env-1", Name: "production"},
				},
			}, nil
		},
	}

	withMockClient(mock, func() {
		err := runDescribeProject(nil, []string{"my-app"})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestRunDescribeProject_YAMLFormat(t *testing.T) {
	originalToken := token
	originalFormat := outputFormat
	defer func() {
		token = originalToken
		outputFormat = originalFormat
	}()

	token = "test-token"
	outputFormat = "yaml"

	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{{ID: "proj-1", Name: "my-app"}}, nil
		},
		GetProjectFunc: func(id string) (types.Project, error) {
			return types.Project{ID: "proj-1", Name: "my-app"}, nil
		},
	}

	withMockClient(mock, func() {
		err := runDescribeProject(nil, []string{"my-app"})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestRunDescribeEnvironment_JSONFormat(t *testing.T) {
	originalToken := token
	originalProject := project
	originalFormat := outputFormat
	defer func() {
		token = originalToken
		project = originalProject
		outputFormat = originalFormat
	}()

	token = "test-token"
	project = "my-app"
	outputFormat = "json"

	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{{ID: "proj-1", Name: "my-app"}}, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return []types.Environment{
				{ID: "env-1", Name: "production", ServiceCount: 2, UpdatedAt: time.Now()},
			}, nil
		},
	}

	withMockClient(mock, func() {
		err := runDescribeEnvironment(nil, []string{"production"})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestRunDescribeEnvironment_YAMLFormat(t *testing.T) {
	originalToken := token
	originalProject := project
	originalFormat := outputFormat
	defer func() {
		token = originalToken
		project = originalProject
		outputFormat = originalFormat
	}()

	token = "test-token"
	project = "my-app"
	outputFormat = "yaml"

	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{{ID: "proj-1", Name: "my-app"}}, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return []types.Environment{
				{ID: "env-1", Name: "production"},
			}, nil
		},
	}

	withMockClient(mock, func() {
		err := runDescribeEnvironment(nil, []string{"production"})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// Tests for error paths
func TestRunGetEnvironments_ListProjectsError(t *testing.T) {
	originalToken := token
	originalProject := project
	defer func() {
		token = originalToken
		project = originalProject
	}()

	token = "test-token"
	project = "my-app"

	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return nil, errors.New("API unavailable")
		},
	}

	withMockClient(mock, func() {
		err := runGetEnvironments(nil, nil)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestRunGetEnvironments_ListEnvironmentsError(t *testing.T) {
	originalToken := token
	originalProject := project
	defer func() {
		token = originalToken
		project = originalProject
	}()

	token = "test-token"
	project = "my-app"

	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{{ID: "proj-1", Name: "my-app"}}, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return nil, errors.New("failed to list environments")
		},
	}

	withMockClient(mock, func() {
		err := runGetEnvironments(nil, nil)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestRunDescribeEnvironment_NoEnvName(t *testing.T) {
	originalToken := token
	originalProject := project
	originalEnv := environment
	defer func() {
		token = originalToken
		project = originalProject
		environment = originalEnv
	}()

	token = "test-token"
	project = "my-app"
	environment = ""
	os.Unsetenv("RAILCTL_ENVIRONMENT")

	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{{ID: "proj-1", Name: "my-app"}}, nil
		},
	}

	withMockClient(mock, func() {
		err := runDescribeEnvironment(nil, []string{})
		if err == nil {
			t.Error("expected error for missing env name")
		}
		if !strings.Contains(err.Error(), "-e/--environment is required") {
			t.Errorf("expected '-e/--environment is required' error, got %q", err.Error())
		}
	})
}

func TestRunDescribeEnvironment_NoProject(t *testing.T) {
	originalToken := token
	originalProject := project
	defer func() {
		token = originalToken
		project = originalProject
	}()

	token = "test-token"
	project = ""
	os.Unsetenv("RAILCTL_PROJECT")

	mock := &api.MockClient{}

	withMockClient(mock, func() {
		err := runDescribeEnvironment(nil, []string{"production"})
		if err == nil {
			t.Error("expected error for missing project")
		}
		if !strings.Contains(err.Error(), "-p/--project is required") {
			t.Errorf("expected '-p/--project is required' error, got %q", err.Error())
		}
	})
}

func TestRunCreateEnvironment_NoProject(t *testing.T) {
	originalToken := token
	originalProject := project
	defer func() {
		token = originalToken
		project = originalProject
	}()

	token = "test-token"
	project = ""
	os.Unsetenv("RAILCTL_PROJECT")

	mock := &api.MockClient{}

	withMockClient(mock, func() {
		err := runCreateEnvironment(nil, []string{"staging"})
		if err == nil {
			t.Error("expected error for missing project")
		}
	})
}

func TestRunCreateEnvironment_APIError(t *testing.T) {
	originalToken := token
	originalProject := project
	defer func() {
		token = originalToken
		project = originalProject
	}()

	token = "test-token"
	project = "my-app"

	mock := &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{{ID: "proj-1", Name: "my-app"}}, nil
		},
		CreateEnvironmentFunc: func(projectID, name string) (types.Environment, error) {
			return types.Environment{}, errors.New("environment limit reached")
		},
	}

	withMockClient(mock, func() {
		err := runCreateEnvironment(nil, []string{"staging"})
		if err == nil {
			t.Error("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "environment limit") {
			t.Errorf("expected 'environment limit' error, got %q", err.Error())
		}
	})
}
