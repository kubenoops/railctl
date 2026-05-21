package cmd

import (
	"testing"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/types"
)

func TestRunCreateVolume_MissingProject(t *testing.T) {
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

	cmd := createVolumeCmd
	err := cmd.RunE(cmd, []string{"data", "/data"})

	if err == nil {
		t.Error("expected error for missing project")
	}
}

func TestRunCreateVolume_Success(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origService := service
	origToken := token
	origMountPath := volumeMountPath
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		service = origService
		token = origToken
		volumeMountPath = origMountPath
	}()

	var capturedPath string

	token = "test-token"
	volumeMountPath = "/data"
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
			CreateVolumeFunc: func(projectID, environmentID, serviceID, mountPath string) (api.Volume, error) {
				capturedPath = mountPath
				return api.Volume{
					ID:   "vol-1",
					Name: "data",
				}, nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	service = "api"

	cmd := createVolumeCmd
	err := cmd.RunE(cmd, []string{"data"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if capturedPath != "/data" {
		t.Errorf("expected mount path /data, got %q", capturedPath)
	}
}

func TestRunCreateVolume_InvalidMountPath(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origService := service
	origToken := token
	origMountPath := volumeMountPath
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		service = origService
		token = origToken
		volumeMountPath = origMountPath
	}()

	token = "test-token"
	volumeMountPath = "invalid-path"
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
		}
	}
	project = "my-project"
	environment = "production"
	service = "api"

	cmd := createVolumeCmd
	err := cmd.RunE(cmd, []string{"data"})

	if err == nil {
		t.Error("expected error for invalid mount path")
	}
}
