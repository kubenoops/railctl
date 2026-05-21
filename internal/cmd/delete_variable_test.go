package cmd

import (
	"fmt"
	"testing"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/types"
)

func TestRunDeleteVariable_MissingEnvironment(t *testing.T) {
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

	cmd := deleteVariableCmd
	err := cmd.RunE(cmd, []string{"VAR_NAME"})

	if err == nil {
		t.Error("expected error for missing environment")
	}
}

func TestRunDeleteVariable_APIError(t *testing.T) {
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
			ListServicesFunc: func(projectID, environmentID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{
					{ID: "svc-1", Name: "api"},
				}, nil
			},
			DeleteVariableFunc: func(projectID, environmentID, serviceID, name string) error {
				return fmt.Errorf("API error")
			},
		}
	}
	project = "my-project"
	environment = "production"
	service = "api"

	cmd := deleteVariableCmd
	err := cmd.RunE(cmd, []string{"VAR_NAME"})

	if err == nil {
		t.Error("expected error from API")
	}
}
