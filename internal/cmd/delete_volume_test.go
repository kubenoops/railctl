package cmd

import (
	"testing"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/types"
)

func TestRunDeleteVolume_MissingProject(t *testing.T) {
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

	cmd := deleteVolumeCmd
	err := cmd.RunE(cmd, []string{"vol-123"})

	if err == nil {
		t.Error("expected error for missing project")
	}
}

func TestRunDeleteVolume_Success(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	origYes := deleteVolumeYes
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
		deleteVolumeYes = origYes
	}()

	var deletedID string

	token = "test-token"
	deleteVolumeYes = true
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
							Name: "data",
						},
					},
				}, nil
			},
			DeleteVolumeFunc: func(volumeID string) error {
				deletedID = volumeID
				return nil
			},
		}
	}
	project = "my-project"
	environment = "production"

	cmd := deleteVolumeCmd
	err := cmd.RunE(cmd, []string{"vol-123"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if deletedID != "vol-123" {
		t.Errorf("expected deleted ID vol-123, got %q", deletedID)
	}
}

func TestRunDeleteVolume_NotFound(t *testing.T) {
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

	cmd := deleteVolumeCmd
	err := cmd.RunE(cmd, []string{"nonexistent"})

	if err == nil {
		t.Error("expected error for volume not found")
	}
}
