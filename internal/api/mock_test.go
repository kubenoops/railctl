package api

import (
	"errors"
	"testing"

	"github.com/kubenoops/railctl/internal/types"
)

func TestMockClient_ImplementsAPIClient(t *testing.T) {
	var _ APIClient = (*MockClient)(nil)
}

func TestMockClient_ListProjects(t *testing.T) {
	t.Run("with func", func(t *testing.T) {
		mock := &MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "1", Name: "test"}}, nil
			},
		}
		projects, err := mock.ListProjects()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(projects) != 1 {
			t.Errorf("expected 1 project, got %d", len(projects))
		}
	})

	t.Run("without func returns nil", func(t *testing.T) {
		mock := &MockClient{}
		projects, err := mock.ListProjects()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if projects != nil {
			t.Errorf("expected nil, got %v", projects)
		}
	})
}

func TestMockClient_GetProject(t *testing.T) {
	t.Run("with func", func(t *testing.T) {
		mock := &MockClient{
			GetProjectFunc: func(id string) (types.Project, error) {
				return types.Project{ID: id, Name: "test"}, nil
			},
		}
		project, err := mock.GetProject("proj-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if project.ID != "proj-1" {
			t.Errorf("expected ID 'proj-1', got %q", project.ID)
		}
	})

	t.Run("without func returns empty", func(t *testing.T) {
		mock := &MockClient{}
		project, err := mock.GetProject("proj-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if project.ID != "" {
			t.Errorf("expected empty ID, got %q", project.ID)
		}
	})
}

func TestMockClient_CreateProject(t *testing.T) {
	t.Run("with func", func(t *testing.T) {
		mock := &MockClient{
			CreateProjectFunc: func(name string) (types.Project, error) {
				return types.Project{Name: name}, nil
			},
		}
		project, err := mock.CreateProject("new-proj")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if project.Name != "new-proj" {
			t.Errorf("expected name 'new-proj', got %q", project.Name)
		}
	})

	t.Run("without func returns empty", func(t *testing.T) {
		mock := &MockClient{}
		project, err := mock.CreateProject("new-proj")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if project.Name != "" {
			t.Errorf("expected empty name, got %q", project.Name)
		}
	})
}

func TestMockClient_DeleteProject(t *testing.T) {
	t.Run("with func returns error", func(t *testing.T) {
		mock := &MockClient{
			DeleteProjectFunc: func(id string) error {
				return errors.New("delete failed")
			},
		}
		err := mock.DeleteProject("proj-1")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("without func returns nil", func(t *testing.T) {
		mock := &MockClient{}
		err := mock.DeleteProject("proj-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestMockClient_ListEnvironments(t *testing.T) {
	t.Run("with func", func(t *testing.T) {
		mock := &MockClient{
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1"}}, nil
			},
		}
		envs, err := mock.ListEnvironments("proj-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(envs) != 1 {
			t.Errorf("expected 1 env, got %d", len(envs))
		}
	})

	t.Run("without func returns nil", func(t *testing.T) {
		mock := &MockClient{}
		envs, err := mock.ListEnvironments("proj-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if envs != nil {
			t.Errorf("expected nil, got %v", envs)
		}
	})
}

func TestMockClient_CreateEnvironment(t *testing.T) {
	t.Run("with func", func(t *testing.T) {
		mock := &MockClient{
			CreateEnvironmentFunc: func(projectID, name string) (types.Environment, error) {
				return types.Environment{Name: name}, nil
			},
		}
		env, err := mock.CreateEnvironment("proj-1", "staging")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if env.Name != "staging" {
			t.Errorf("expected name 'staging', got %q", env.Name)
		}
	})

	t.Run("without func returns empty", func(t *testing.T) {
		mock := &MockClient{}
		env, err := mock.CreateEnvironment("proj-1", "staging")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if env.Name != "" {
			t.Errorf("expected empty name, got %q", env.Name)
		}
	})
}

func TestMockClient_DeleteEnvironment(t *testing.T) {
	t.Run("with func returns error", func(t *testing.T) {
		mock := &MockClient{
			DeleteEnvironmentFunc: func(id string) error {
				return errors.New("delete failed")
			},
		}
		err := mock.DeleteEnvironment("env-1")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("without func returns nil", func(t *testing.T) {
		mock := &MockClient{}
		err := mock.DeleteEnvironment("env-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestMockClient_GetWorkspaceID(t *testing.T) {
	t.Run("with func", func(t *testing.T) {
		mock := &MockClient{
			GetWorkspaceIDFunc: func() (string, error) {
				return "ws-custom", nil
			},
		}
		id, err := mock.GetWorkspaceID()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != "ws-custom" {
			t.Errorf("expected 'ws-custom', got %q", id)
		}
	})

	t.Run("without func returns default", func(t *testing.T) {
		mock := &MockClient{}
		id, err := mock.GetWorkspaceID()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != "mock-workspace-id" {
			t.Errorf("expected 'mock-workspace-id', got %q", id)
		}
	})
}

func TestMockClient_ListServices(t *testing.T) {
	mock := &MockClient{
		ListServicesFunc: func(projectID, environmentID string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{{ID: "svc-1"}}, nil
		},
	}
	services, err := mock.ListServices("proj-1", "env-1")
	if err != nil || len(services) != 1 {
		t.Errorf("ListServices() = %v, %v", services, err)
	}
}

func TestMockClient_GetService(t *testing.T) {
	mock := &MockClient{
		GetServiceFunc: func(id string) (types.ServiceDetail, error) {
			return types.ServiceDetail{ID: id}, nil
		},
	}
	service, err := mock.GetService("svc-1")
	if err != nil || service.ID != "svc-1" {
		t.Errorf("GetService() = %v, %v", service, err)
	}
}

func TestMockClient_CreateService(t *testing.T) {
	mock := &MockClient{
		CreateServiceFunc: func(projectID, environmentID, name, image string, creds *RegistryCredentials) (types.Service, error) {
			return types.Service{Name: name}, nil
		},
	}
	service, err := mock.CreateService("proj-1", "env-1", "web", "nginx:latest", nil)
	if err != nil || service.Name != "web" {
		t.Errorf("CreateService() = %v, %v", service, err)
	}
}

func TestMockClient_UpdateServiceInstance(t *testing.T) {
	mock := &MockClient{UpdateServiceInstanceFunc: func(serviceID, environmentID, image string, creds *RegistryCredentials) error { return nil }}
	if err := mock.UpdateServiceInstance("svc-1", "env-1", "nginx:alpine", nil); err != nil {
		t.Errorf("UpdateServiceInstance() error = %v", err)
	}
}

func TestMockClient_DeployServiceInstance(t *testing.T) {
	mock := &MockClient{DeployServiceInstanceFunc: func(serviceID, environmentID string) (string, error) { return "deploy-123", nil }}
	deployID, err := mock.DeployServiceInstance("svc-1", "env-1")
	if err != nil || deployID != "deploy-123" {
		t.Errorf("DeployServiceInstance() = %v, %v", deployID, err)
	}
}

func TestMockClient_RedeployDeployment(t *testing.T) {
	mock := &MockClient{RedeployDeploymentFunc: func(deploymentID string) error { return nil }}
	if err := mock.RedeployDeployment("deploy-1"); err != nil {
		t.Errorf("RedeployDeployment() error = %v", err)
	}
}

func TestMockClient_DeleteService(t *testing.T) {
	mock := &MockClient{DeleteServiceFunc: func(id string) error { return nil }}
	if err := mock.DeleteService("svc-1"); err != nil {
		t.Errorf("DeleteService() error = %v", err)
	}
}

func TestMockClient_GetBuildLogs(t *testing.T) {
	mock := &MockClient{GetBuildLogsFunc: func(deploymentID string, limit int) ([]string, error) { return []string{"log"}, nil }}
	logs, err := mock.GetBuildLogs("deploy-1", 100)
	if err != nil || len(logs) != 1 {
		t.Errorf("GetBuildLogs() = %v, %v", logs, err)
	}
}

func TestMockClient_GetVariables(t *testing.T) {
	mock := &MockClient{GetVariablesFunc: func(projectID, envID, serviceID string) (map[string]string, error) {
		return map[string]string{"K": "v"}, nil
	}}
	vars, err := mock.GetVariables("proj-1", "env-1", "svc-1")
	if err != nil || len(vars) != 1 {
		t.Errorf("GetVariables() = %v, %v", vars, err)
	}
}

func TestMockClient_SetVariables(t *testing.T) {
	mock := &MockClient{SetVariablesFunc: func(projectID, envID, serviceID string, vars map[string]string, skipDeploys bool) error { return nil }}
	if err := mock.SetVariables("proj-1", "env-1", "svc-1", map[string]string{"K": "v"}, false); err != nil {
		t.Errorf("SetVariables() error = %v", err)
	}
}

func TestMockClient_DeleteVariable(t *testing.T) {
	mock := &MockClient{DeleteVariableFunc: func(projectID, envID, serviceID, name string) error { return nil }}
	if err := mock.DeleteVariable("proj-1", "env-1", "svc-1", "KEY"); err != nil {
		t.Errorf("DeleteVariable() error = %v", err)
	}
}

func TestMockClient_GetSealedVariables(t *testing.T) {
	mock := &MockClient{GetSealedVariablesFunc: func(environmentID, serviceID string) (map[string]bool, error) { return map[string]bool{"K": true}, nil }}
	vars, err := mock.GetSealedVariables("env-1", "svc-1")
	if err != nil || len(vars) != 1 {
		t.Errorf("GetSealedVariables() = %v, %v", vars, err)
	}
}

func TestMockClient_ListDeployments(t *testing.T) {
	mock := &MockClient{ListDeploymentsFunc: func(projectID, environmentID, serviceID string, limit int) ([]Deployment, error) {
		return []Deployment{{ID: "d-1"}}, nil
	}}
	deps, err := mock.ListDeployments("proj-1", "env-1", "svc-1", 10)
	if err != nil || len(deps) != 1 {
		t.Errorf("ListDeployments() = %v, %v", deps, err)
	}
}

func TestMockClient_RemoveDeployment(t *testing.T) {
	mock := &MockClient{RemoveDeploymentFunc: func(deploymentID string) error { return nil }}
	if err := mock.RemoveDeployment("deploy-1"); err != nil {
		t.Errorf("RemoveDeployment() error = %v", err)
	}
}

func TestMockClient_DeleteServiceInstance(t *testing.T) {
	t.Run("with func", func(t *testing.T) {
		mock := &MockClient{DeleteServiceInstanceFunc: func(serviceID, environmentID string) error { return nil }}
		if err := mock.DeleteServiceInstance("svc-1", "env-1"); err != nil {
			t.Errorf("DeleteServiceInstance() error = %v", err)
		}
	})
	t.Run("without func returns nil", func(t *testing.T) {
		mock := &MockClient{}
		if err := mock.DeleteServiceInstance("svc-1", "env-1"); err != nil {
			t.Errorf("DeleteServiceInstance() error = %v", err)
		}
	})
}

func TestMockClient_UpdateServiceInstanceConfig(t *testing.T) {
	t.Run("with func", func(t *testing.T) {
		cmd := "npm start"
		mock := &MockClient{
			UpdateServiceInstanceConfigFunc: func(serviceID, environmentID string, startCommand, restartPolicy *string, maxRetries, replicas *int, healthcheckPath *string, healthcheckTimeout *int) error {
				return nil
			},
		}
		if err := mock.UpdateServiceInstanceConfig("svc-1", "env-1", &cmd, nil, nil, nil, nil, nil); err != nil {
			t.Errorf("UpdateServiceInstanceConfig() error = %v", err)
		}
	})
	t.Run("without func returns nil", func(t *testing.T) {
		mock := &MockClient{}
		if err := mock.UpdateServiceInstanceConfig("svc-1", "env-1", nil, nil, nil, nil, nil, nil); err != nil {
			t.Errorf("UpdateServiceInstanceConfig() error = %v", err)
		}
	})
}

func TestMockClient_GetDeploymentLogs(t *testing.T) {
	t.Run("with func", func(t *testing.T) {
		mock := &MockClient{GetDeploymentLogsFunc: func(deploymentID string, limit int) ([]LogEntry, error) {
			return []LogEntry{{Message: "log1"}}, nil
		}}
		logs, err := mock.GetDeploymentLogs("deploy-1", 100)
		if err != nil || len(logs) != 1 {
			t.Errorf("GetDeploymentLogs() = %v, %v", logs, err)
		}
	})
	t.Run("without func returns nil", func(t *testing.T) {
		mock := &MockClient{}
		logs, err := mock.GetDeploymentLogs("deploy-1", 100)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if logs != nil {
			t.Errorf("expected nil, got %v", logs)
		}
	})
}
