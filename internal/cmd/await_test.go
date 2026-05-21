package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/kubenoops/railctl/internal/api"
)

func TestAwaitDeployment_ImmediateSuccess(t *testing.T) {
	client := &api.MockClient{
		ListDeploymentsFunc: func(projectID, environmentID, serviceID string, limit int) ([]api.Deployment, error) {
			return []api.Deployment{
				{ID: "dep-123", Status: "SUCCESS", CreatedAt: time.Now()},
			}, nil
		},
	}

	err := awaitDeployment(client, "proj-1", "env-1", "svc-1", "dep-123", "api", 600)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestAwaitDeployment_TransitionToSuccess(t *testing.T) {
	callCount := 0
	client := &api.MockClient{
		ListDeploymentsFunc: func(projectID, environmentID, serviceID string, limit int) ([]api.Deployment, error) {
			callCount++
			var status string
			switch {
			case callCount <= 1:
				status = "BUILDING"
			case callCount <= 2:
				status = "DEPLOYING"
			default:
				status = "SUCCESS"
			}
			return []api.Deployment{
				{ID: "dep-123", Status: status, CreatedAt: time.Now()},
			}, nil
		},
	}

	err := awaitDeployment(client, "proj-1", "env-1", "svc-1", "dep-123", "api", 600)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if callCount < 3 {
		t.Errorf("expected at least 3 poll calls, got %d", callCount)
	}
}

func TestAwaitDeployment_Failed(t *testing.T) {
	client := &api.MockClient{
		ListDeploymentsFunc: func(projectID, environmentID, serviceID string, limit int) ([]api.Deployment, error) {
			return []api.Deployment{
				{ID: "dep-123", Status: "FAILED", CreatedAt: time.Now()},
			}, nil
		},
		GetBuildLogsFunc: func(deploymentID string, limit int) ([]string, error) {
			return []string{"Error: container exited with code 1"}, nil
		},
	}

	err := awaitDeployment(client, "proj-1", "env-1", "svc-1", "dep-123", "api", 600)
	if err == nil {
		t.Error("expected error for failed deployment")
	}
}

func TestAwaitDeployment_DeploymentNotFound(t *testing.T) {
	client := &api.MockClient{
		ListDeploymentsFunc: func(projectID, environmentID, serviceID string, limit int) ([]api.Deployment, error) {
			return []api.Deployment{
				{ID: "other-dep", Status: "SUCCESS", CreatedAt: time.Now()},
			}, nil
		},
	}

	err := awaitDeployment(client, "proj-1", "env-1", "svc-1", "dep-123", "api", 600)
	if err == nil {
		t.Error("expected error for deployment not found")
	}
}

func TestAwaitDeployment_Timeout(t *testing.T) {
	client := &api.MockClient{
		ListDeploymentsFunc: func(projectID, environmentID, serviceID string, limit int) ([]api.Deployment, error) {
			return []api.Deployment{
				{ID: "dep-123", Status: "BUILDING", CreatedAt: time.Now()},
			}, nil
		},
	}

	// Use 1-second timeout so the test completes quickly
	err := awaitDeployment(client, "proj-1", "env-1", "svc-1", "dep-123", "api", 1)
	if err == nil {
		t.Error("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout error message, got: %v", err)
	}
}
