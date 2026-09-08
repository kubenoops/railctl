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

// REMOVED with a strictly newer deployment present means the tracked rollout
// was superseded — await must follow the replacement, not fail. This is the
// live failure mode where Railway's reconciliation supersedes railctl's own
// deploy trigger (observed 2026-09-03).
func TestAwaitDeployment_FollowsSupersedingDeployment(t *testing.T) {
	callCount := 0
	base := time.Date(2026, 9, 3, 15, 0, 0, 0, time.UTC)
	client := &api.MockClient{
		ListDeploymentsFunc: func(projectID, environmentID, serviceID string, limit int) ([]api.Deployment, error) {
			callCount++
			replacementStatus := "BUILDING"
			if callCount > 1 {
				replacementStatus = "SUCCESS"
			}
			return []api.Deployment{
				{ID: "dep-new1", Status: replacementStatus, CreatedAt: base.Add(10 * time.Second)},
				{ID: "dep-123", Status: "REMOVED", CreatedAt: base},
			}, nil
		},
	}

	err := awaitDeployment(client, "proj-1", "env-1", "svc-1", "dep-123", "api", 600)
	if err != nil {
		t.Errorf("expected await to follow the superseding deployment, got: %v", err)
	}
}

// REMOVED with nothing newer to follow is a genuine failure — and an OLDER
// SUCCESS deployment must never be re-targeted (that would report success on
// a stale roll).
func TestAwaitDeployment_RemovedWithoutNewerDeployment(t *testing.T) {
	base := time.Date(2026, 9, 3, 15, 0, 0, 0, time.UTC)
	client := &api.MockClient{
		ListDeploymentsFunc: func(projectID, environmentID, serviceID string, limit int) ([]api.Deployment, error) {
			return []api.Deployment{
				{ID: "dep-old", Status: "SUCCESS", CreatedAt: base.Add(-10 * time.Second)},
				{ID: "dep-123", Status: "REMOVED", CreatedAt: base},
			}, nil
		},
	}

	err := awaitDeployment(client, "proj-1", "env-1", "svc-1", "dep-123", "api", 600)
	if err == nil {
		t.Error("expected error when a removed deployment has no newer replacement")
	}
	if !strings.Contains(err.Error(), "REMOVED") {
		t.Errorf("expected REMOVED in error, got: %v", err)
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
