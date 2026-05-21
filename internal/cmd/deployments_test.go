package cmd

import (
	"testing"
	"time"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/types"
)

func TestRunGetDeployments_MissingProject(t *testing.T) {
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

	cmd := getDeploymentsCmd
	err := cmd.RunE(cmd, []string{})

	if err == nil {
		t.Error("expected error for missing project")
	}
}

func TestRunGetDeployments_MissingService(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origService := service
	origToken := token
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		service = origService
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
		}
	}
	project = "my-project"
	environment = "production"
	service = ""

	cmd := getDeploymentsCmd
	err := cmd.RunE(cmd, []string{})

	if err == nil {
		t.Error("expected error for missing service")
	}
}

func TestRunGetDeployments_Success(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origService := service
	origToken := token
	origFormat := outputFormat
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		service = origService
		token = origToken
		outputFormat = origFormat
	}()

	token = "test-token"
	outputFormat = ""
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{
					{ID: "svc-1", Name: "api"},
				}, nil
			},
			ListDeploymentsFunc: func(projectID, environmentID, serviceID string, limit int) ([]api.Deployment, error) {
				return []api.Deployment{
					{ID: "dep-1", Status: "SUCCESS", CreatedAt: time.Now(), CreatorName: "user1"},
					{ID: "dep-2", Status: "REMOVED", CreatedAt: time.Now(), CreatorName: "user2"},
				}, nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	service = "api"

	cmd := getDeploymentsCmd
	err := cmd.RunE(cmd, []string{})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunGetDeployments_EmptyList(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origService := service
	origToken := token
	origFormat := outputFormat
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		service = origService
		token = origToken
		outputFormat = origFormat
	}()

	token = "test-token"
	outputFormat = ""
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{
					{ID: "svc-1", Name: "api"},
				}, nil
			},
			ListDeploymentsFunc: func(projectID, environmentID, serviceID string, limit int) ([]api.Deployment, error) {
				return []api.Deployment{}, nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	service = "api"

	cmd := getDeploymentsCmd
	err := cmd.RunE(cmd, []string{})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunGetDeployments_JSONOutput(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origService := service
	origToken := token
	origFormat := outputFormat
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		service = origService
		token = origToken
		outputFormat = origFormat
	}()

	token = "test-token"
	outputFormat = "json"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{
					{ID: "svc-1", Name: "api"},
				}, nil
			},
			ListDeploymentsFunc: func(projectID, environmentID, serviceID string, limit int) ([]api.Deployment, error) {
				return []api.Deployment{
					{ID: "dep-1", Status: "SUCCESS", CreatedAt: time.Now()},
				}, nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	service = "api"

	cmd := getDeploymentsCmd
	err := cmd.RunE(cmd, []string{})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunGetDeployments_WideOutput(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origService := service
	origToken := token
	origFormat := outputFormat
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		service = origService
		token = origToken
		outputFormat = origFormat
	}()

	token = "test-token"
	outputFormat = "wide"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{
					{ID: "svc-1", Name: "api"},
				}, nil
			},
			ListDeploymentsFunc: func(projectID, environmentID, serviceID string, limit int) ([]api.Deployment, error) {
				return []api.Deployment{
					{ID: "dep-1", Status: "SUCCESS", CreatedAt: time.Now(), Image: "nginx:latest"},
					{ID: "dep-2", Status: "REMOVED", CreatedAt: time.Now(), Image: ""},
				}, nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	service = "api"

	cmd := getDeploymentsCmd
	err := cmd.RunE(cmd, []string{})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunCreateDeployment_MissingProject(t *testing.T) {
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

	cmd := createDeploymentCmd
	err := cmd.RunE(cmd, []string{})

	if err == nil {
		t.Error("expected error for missing project")
	}
}

func TestRunCreateDeployment_Success(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origService := service
	origToken := token
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		service = origService
		token = origToken
	}()

	var deployedServiceID, deployedEnvID string

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
				return []types.ServiceDetail{
					{ID: "svc-1", Name: "api"},
				}, nil
			},
			DeployServiceInstanceFunc: func(serviceID, environmentID string) (string, error) {
				deployedServiceID = serviceID
				deployedEnvID = environmentID
				return "new-deployment-id", nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	service = "api"

	cmd := createDeploymentCmd
	err := cmd.RunE(cmd, []string{})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if deployedServiceID != "svc-1" {
		t.Errorf("expected service ID 'svc-1', got %q", deployedServiceID)
	}
	if deployedEnvID != "env-1" {
		t.Errorf("expected env ID 'env-1', got %q", deployedEnvID)
	}
}

func TestRunDeleteDeployment_MissingProject(t *testing.T) {
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

	cmd := deleteDeploymentCmd
	err := cmd.RunE(cmd, []string{"dep-123"})

	if err == nil {
		t.Error("expected error for missing project")
	}
}

func TestRunDeleteDeployment_Success(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origService := service
	origToken := token
	origYes := deleteDeploymentYes
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		service = origService
		token = origToken
		deleteDeploymentYes = origYes
	}()

	var removedID string

	token = "test-token"
	deleteDeploymentYes = true // Skip confirmation
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{
					{ID: "svc-1", Name: "api"},
				}, nil
			},
			ListDeploymentsFunc: func(projectID, environmentID, serviceID string, limit int) ([]api.Deployment, error) {
				return []api.Deployment{
					{ID: "dep-123", Status: "SUCCESS", CreatedAt: time.Now()},
				}, nil
			},
			RemoveDeploymentFunc: func(deploymentID string) error {
				removedID = deploymentID
				return nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	service = "api"

	cmd := deleteDeploymentCmd
	err := cmd.RunE(cmd, []string{"dep-123"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if removedID != "dep-123" {
		t.Errorf("expected removed deployment 'dep-123', got %q", removedID)
	}
}

func TestRunDeleteDeployment_NotFound(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origService := service
	origToken := token
	origYes := deleteDeploymentYes
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		service = origService
		token = origToken
		deleteDeploymentYes = origYes
	}()

	token = "test-token"
	deleteDeploymentYes = true
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{
					{ID: "svc-1", Name: "api"},
				}, nil
			},
			ListDeploymentsFunc: func(projectID, environmentID, serviceID string, limit int) ([]api.Deployment, error) {
				return []api.Deployment{}, nil // No deployments
			},
		}
	}
	project = "my-project"
	environment = "production"
	service = "api"

	cmd := deleteDeploymentCmd
	err := cmd.RunE(cmd, []string{"nonexistent"})

	if err == nil {
		t.Error("expected error for deployment not found")
	}
}

func TestPrintDeploymentsTable(t *testing.T) {
	deployments := []api.Deployment{
		{ID: "dep-1", Status: "SUCCESS", CreatedAt: time.Now(), CreatorName: "user1"},
		{ID: "dep-2", Status: "REMOVED", CreatedAt: time.Now(), CreatorName: ""},
	}

	// Should not panic
	printDeploymentsTable(deployments)
}

func TestPrintDeploymentsWideTable(t *testing.T) {
	deployments := []api.Deployment{
		{ID: "dep-1", Status: "SUCCESS", CreatedAt: time.Now(), CreatorName: "user1", Image: "nginx:latest"},
		{ID: "dep-2", Status: "REMOVED", CreatedAt: time.Now(), CreatorName: "", Image: ""},
	}

	// Should not panic
	printDeploymentsWideTable(deployments)
}
