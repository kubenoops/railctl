package cmd

import (
	"testing"
	"time"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/types"
)

func TestRunLogsService_MissingProject(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origToken := token
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		token = origToken
	}()

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{}
	}
	project = ""

	cmd := logsServiceCmd
	err := cmd.RunE(cmd, []string{"my-service"})

	if err == nil {
		t.Error("expected error for missing project")
	}
}

func TestRunLogsService_MissingEnvironment(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
	}()

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
		}
	}
	project = "my-project"
	environment = ""

	cmd := logsServiceCmd
	err := cmd.RunE(cmd, []string{"my-service"})

	if err == nil {
		t.Error("expected error for missing environment")
	}
}

func TestRunLogsService_ServiceNotFound(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
	}()

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{{ID: "svc-1", Name: "other-service"}}, nil
			},
		}
	}
	project = "my-project"
	environment = "production"

	cmd := logsServiceCmd
	err := cmd.RunE(cmd, []string{"my-service"})

	if err == nil {
		t.Error("expected error for service not found")
	}
}

func TestRunLogsService_NoDeployments(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
	}()

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{{ID: "svc-1", Name: "my-service"}}, nil
			},
			ListDeploymentsFunc: func(projectID, environmentID, serviceID string, limit int) ([]api.Deployment, error) {
				return []api.Deployment{}, nil
			},
		}
	}
	project = "my-project"
	environment = "production"

	cmd := logsServiceCmd
	err := cmd.RunE(cmd, []string{"my-service"})

	if err == nil {
		t.Error("expected error for no deployments found")
	}
}

func TestRunLogsService_Success(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	origTail := logsTail
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
		logsTail = origTail
	}()

	token = "test-token"
	logsTail = 100
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{{ID: "svc-1", Name: "my-service"}}, nil
			},
			ListDeploymentsFunc: func(projectID, environmentID, serviceID string, limit int) ([]api.Deployment, error) {
				return []api.Deployment{
					{ID: "deploy-1", Status: "SUCCESS", CreatedAt: time.Now()},
				}, nil
			},
			GetDeploymentLogsFunc: func(deploymentID string, limit int) ([]api.LogEntry, error) {
				return []api.LogEntry{
					{Timestamp: time.Now(), Message: "Test log message"},
				}, nil
			},
		}
	}
	project = "my-project"
	environment = "production"

	cmd := logsServiceCmd
	err := cmd.RunE(cmd, []string{"my-service"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunLogsService_WithSpecificDeployment(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	origDeploymentFlag := logsDeployment
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
		logsDeployment = origDeploymentFlag
	}()

	token = "test-token"
	logsDeployment = "specific-deploy-id"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{{ID: "svc-1", Name: "my-service"}}, nil
			},
			GetDeploymentLogsFunc: func(deploymentID string, limit int) ([]api.LogEntry, error) {
				if deploymentID != "specific-deploy-id" {
					t.Errorf("expected deployment ID 'specific-deploy-id', got %q", deploymentID)
				}
				return []api.LogEntry{
					{Timestamp: time.Now(), Message: "Specific deployment log"},
				}, nil
			},
		}
	}
	project = "my-project"
	environment = "production"

	cmd := logsServiceCmd
	err := cmd.RunE(cmd, []string{"my-service"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunLogsService_FallbackToNonSuccessDeployment(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
	}()

	var usedDeploymentID string

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{{ID: "svc-1", Name: "my-service"}}, nil
			},
			ListDeploymentsFunc: func(projectID, environmentID, serviceID string, limit int) ([]api.Deployment, error) {
				// Only failed deployment available
				return []api.Deployment{
					{ID: "deploy-failed", Status: "FAILED", CreatedAt: time.Now()},
				}, nil
			},
			GetDeploymentLogsFunc: func(deploymentID string, limit int) ([]api.LogEntry, error) {
				usedDeploymentID = deploymentID
				return []api.LogEntry{
					{Timestamp: time.Now(), Message: "Failed deployment log"},
				}, nil
			},
		}
	}
	project = "my-project"
	environment = "production"

	cmd := logsServiceCmd
	err := cmd.RunE(cmd, []string{"my-service"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if usedDeploymentID != "deploy-failed" {
		t.Errorf("expected to use failing deployment 'deploy-failed', got %q", usedDeploymentID)
	}
}

func TestRunLogsService_EmptyLogs(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
	}()

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{{ID: "svc-1", Name: "my-service"}}, nil
			},
			ListDeploymentsFunc: func(projectID, environmentID, serviceID string, limit int) ([]api.Deployment, error) {
				return []api.Deployment{
					{ID: "deploy-1", Status: "SUCCESS", CreatedAt: time.Now()},
				}, nil
			},
			GetDeploymentLogsFunc: func(deploymentID string, limit int) ([]api.LogEntry, error) {
				return []api.LogEntry{}, nil // Empty logs
			},
		}
	}
	project = "my-project"
	environment = "production"

	cmd := logsServiceCmd
	err := cmd.RunE(cmd, []string{"my-service"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunLogsService_FollowMode(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	origFollow := logsFollow
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
		logsFollow = origFollow
	}()

	callCount := 0
	token = "test-token"
	logsFollow = true
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{{ID: "svc-1", Name: "my-service"}}, nil
			},
			ListDeploymentsFunc: func(projectID, environmentID, serviceID string, limit int) ([]api.Deployment, error) {
				return []api.Deployment{
					{ID: "deploy-1", Status: "SUCCESS", CreatedAt: time.Now()},
				}, nil
			},
			GetDeploymentLogsFunc: func(deploymentID string, limit int) ([]api.LogEntry, error) {
				callCount++
				// Return logs on first call, empty on subsequent to avoid infinite loop in test
				if callCount == 1 {
					return []api.LogEntry{
						{Timestamp: time.Now(), Message: "Initial log"},
					}, nil
				}
				return []api.LogEntry{}, nil
			},
		}
	}
	project = "my-project"
	environment = "production"

	// Note: We can't fully test follow mode in unit tests since it runs indefinitely
	// This test just verifies the followLogs function can be called without errors
	// and that it makes the initial API call
	cmd := logsServiceCmd

	// Run in a goroutine and cancel after a short time to avoid hanging
	done := make(chan error, 1)
	go func() {
		done <- cmd.RunE(cmd, []string{"my-service"})
	}()

	// Wait briefly to ensure at least one API call is made
	time.Sleep(100 * time.Millisecond)

	if callCount < 1 {
		t.Error("Expected at least one API call in follow mode")
	}
}
