package cmd

import (
	"testing"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/types"
)

func TestRunGetVolumes_MissingProject(t *testing.T) {
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

	cmd := getVolumesCmd
	err := cmd.RunE(cmd, []string{})

	if err == nil {
		t.Error("expected error for missing project")
	}
}

func TestRunGetVolumes_Success(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	origFormat := outputFormat
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
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
			ListVolumesFunc: func(projectID, environmentID string) ([]api.VolumeInstance, error) {
				mountPath := "/data"
				serviceID := "svc-1"
				return []api.VolumeInstance{
					{
						Volume: api.Volume{
							ID:   "vol-1",
							Name: "data",
						},
						MountPath: mountPath,
						ServiceID: &serviceID,
					},
				}, nil
			},
			ListServicesFunc: func(projectID, environmentID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{
					{ID: "svc-1", Name: "api"},
				}, nil
			},
		}
	}
	project = "my-project"
	environment = "production"

	cmd := getVolumesCmd
	err := cmd.RunE(cmd, []string{})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunGetVolumes_EmptyList(t *testing.T) {
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
			ListVolumesFunc: func(projectID, environmentID string) ([]api.VolumeInstance, error) {
				return []api.VolumeInstance{}, nil
			},
		}
	}
	project = "my-project"
	environment = "production"

	cmd := getVolumesCmd
	err := cmd.RunE(cmd, []string{})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func strPtr(s string) *string {
	return &s
}
