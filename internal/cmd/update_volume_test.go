package cmd

import (
	"testing"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/types"
)

func TestRunUpdateVolume_MissingProject(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origToken := token
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		token = origToken
		updateVolumeName = ""
	}()

	token = "test-token"
	updateVolumeName = "new-name"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{}
	}
	project = ""

	cmd := updateVolumeCmd
	err := cmd.RunE(cmd, []string{"vol-123"})

	if err == nil {
		t.Error("expected error for missing project")
	}
}

func TestRunUpdateVolume_Rename(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
		updateVolumeName = ""
	}()

	var capturedName string

	token = "test-token"
	updateVolumeName = "new-volume-name"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListVolumesFunc: func(projectID, environmentID string) ([]api.VolumeInstance, error) {
				return []api.VolumeInstance{
					{
						Volume: api.Volume{
							ID:   "vol-123",
							Name: "old-name",
						},
					},
				}, nil
			},
			UpdateVolumeNameFunc: func(volumeID, name string) error {
				capturedName = name
				return nil
			},
		}
	}
	project = "my-project"
	environment = "production"

	cmd := updateVolumeCmd
	err := cmd.RunE(cmd, []string{"vol-123"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if capturedName != "new-volume-name" {
		t.Errorf("expected name=new-volume-name, got %q", capturedName)
	}
}

func TestRunUpdateVolume_NoFlags(t *testing.T) {
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
		}
	}
	project = "my-project"
	environment = "production"

	cmd := updateVolumeCmd
	err := cmd.RunE(cmd, []string{"vol-123"})

	if err == nil {
		t.Error("expected error when no update flags provided")
	}
}
