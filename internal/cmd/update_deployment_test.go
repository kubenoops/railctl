package cmd

import (
	"testing"

	"github.com/kubenoops/railctl/internal/api"
)

func TestRunUpdateDeployment_MissingSetActiveFlag(t *testing.T) {
	origAPIClient := newAPIClient
	origToken := token
	defer func() {
		newAPIClient = origAPIClient
		token = origToken
		setActive = false
	}()

	token = "test-token"
	setActive = false
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{}
	}

	cmd := updateDeploymentCmd
	err := cmd.RunE(cmd, []string{"deploy-123"})

	if err == nil {
		t.Error("expected error when --set-active flag is not provided")
	}
}

func TestRunUpdateDeployment_Success(t *testing.T) {
	origAPIClient := newAPIClient
	origToken := token
	defer func() {
		newAPIClient = origAPIClient
		token = origToken
		setActive = false
	}()

	var redeployedID string

	token = "test-token"
	setActive = true
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			RedeployDeploymentFunc: func(deploymentID string) error {
				redeployedID = deploymentID
				return nil
			},
		}
	}

	cmd := updateDeploymentCmd
	err := cmd.RunE(cmd, []string{"deploy-123"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if redeployedID != "deploy-123" {
		t.Errorf("expected deployment ID deploy-123, got %q", redeployedID)
	}
}
