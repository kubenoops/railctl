package cmd

import (
	"strings"
	"testing"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/types"
)

// resourceProtectionMock resolves my-project/production with one volume and one
// service, and reports the given shared variables for the environment so the
// delete-protection guard can be exercised end-to-end.
func resourceProtectionMock(sharedVars map[string]string, deletedVolID *string) *api.MockClient {
	return &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return []types.Environment{{ID: "env-1", Name: "production"}}, nil
		},
		GetSharedVariablesFunc: func(projectID, environmentID string) (map[string]string, error) {
			return sharedVars, nil
		},
		ListVolumesFunc: func(projectID, environmentID string) ([]api.VolumeInstance, error) {
			return []api.VolumeInstance{
				{Volume: api.Volume{ID: "vol-123", Name: "data"}},
			}, nil
		},
		DeleteVolumeFunc: func(volumeID string) error {
			if deletedVolID != nil {
				*deletedVolID = volumeID
			}
			return nil
		},
	}
}

// TestRunDeleteVolume_ProtectedEnvBlocks proves a volume (data) cannot be
// deleted from a delete-protected environment, and that the guard fires before
// the DeleteVolume call. --yes is set to prove protection is not a prompt.
func TestRunDeleteVolume_ProtectedEnvBlocks(t *testing.T) {
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

	var deletedVolID string
	token = "test-token"
	deleteVolumeYes = true // --yes skips the prompt, not the protection
	newAPIClient = func(tkn string) api.APIClient {
		return resourceProtectionMock(map[string]string{"DELETE_PROTECTION": "true"}, &deletedVolID)
	}
	project = "my-project"
	environment = "production"

	cmd := deleteVolumeCmd
	err := cmd.RunE(cmd, []string{"data"})

	if err == nil {
		t.Fatal("expected a delete-protection error, got nil")
	}
	if !strings.Contains(err.Error(), "cannot delete volume") {
		t.Errorf("error should name the blocked volume delete, got: %v", err)
	}
	if !strings.Contains(err.Error(), "unprotect environment production") {
		t.Errorf("error should point at the unprotect command, got: %v", err)
	}
	if deletedVolID != "" {
		t.Errorf("DeleteVolume must NOT run on a protected env, but it deleted %q", deletedVolID)
	}
}

// TestRunDeleteVolume_UnprotectedEnvProceeds is the control: with protection
// off, the same volume deletes normally through the guard.
func TestRunDeleteVolume_UnprotectedEnvProceeds(t *testing.T) {
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

	var deletedVolID string
	token = "test-token"
	deleteVolumeYes = true
	newAPIClient = func(tkn string) api.APIClient {
		return resourceProtectionMock(map[string]string{"DELETE_PROTECTION": "false"}, &deletedVolID)
	}
	project = "my-project"
	environment = "production"

	cmd := deleteVolumeCmd
	if err := cmd.RunE(cmd, []string{"data"}); err != nil {
		t.Fatalf("unexpected error on unprotected env: %v", err)
	}
	if deletedVolID != "vol-123" {
		t.Errorf("expected vol-123 to be deleted, got %q", deletedVolID)
	}
}
