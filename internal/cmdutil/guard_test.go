package cmdutil

import (
	"errors"
	"strings"
	"testing"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/types"
)

func TestRequireWorkspaceScope_ProjectToken(t *testing.T) {
	mock := &api.MockClient{
		IsProjectTokenFunc: func() (bool, error) {
			return true, nil
		},
	}

	err := RequireWorkspaceScope(mock, "list projects")
	if err == nil {
		t.Fatal("expected error for project token, got nil")
	}
	if !strings.Contains(err.Error(), "list projects") {
		t.Errorf("error should mention the operation 'list projects': %v", err)
	}
	if !strings.Contains(err.Error(), "account or workspace") {
		t.Errorf("error should suggest an account or workspace token: %v", err)
	}
	if !strings.Contains(err.Error(), "scoped to a single project") {
		t.Errorf("error should explain the token is scoped to a single project: %v", err)
	}
}

func TestRequireWorkspaceScope_NonProjectToken(t *testing.T) {
	mock := &api.MockClient{
		IsProjectTokenFunc: func() (bool, error) {
			return false, nil
		},
	}

	if err := RequireWorkspaceScope(mock, "delete a project"); err != nil {
		t.Fatalf("unexpected error for non-project token: %v", err)
	}
}

func TestRequireWorkspaceScope_DetectionError(t *testing.T) {
	sentinel := errors.New("network error")
	mock := &api.MockClient{
		IsProjectTokenFunc: func() (bool, error) {
			return false, sentinel
		},
	}

	err := RequireWorkspaceScope(mock, "list projects")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected wrapped sentinel error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "failed to check token type") {
		t.Errorf("error should mention token type check failure: %v", err)
	}
}

func TestGuardServiceCreationScope_NonProjectToken(t *testing.T) {
	mock := &api.MockClient{
		IsProjectTokenFunc: func() (bool, error) {
			return false, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			t.Fatal("ListEnvironments should not be called for a non-project token")
			return nil, nil
		},
	}

	if err := GuardServiceCreationScope(mock, "proj-1", "my-project", "env-1", "production"); err != nil {
		t.Fatalf("unexpected error for non-project token: %v", err)
	}
}

func TestGuardServiceCreationScope_ProjectTokenSingleEnv(t *testing.T) {
	mock := &api.MockClient{
		IsProjectTokenFunc: func() (bool, error) {
			return true, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return []types.Environment{
				{ID: "env-1", Name: "production"},
			}, nil
		},
	}

	if err := GuardServiceCreationScope(mock, "proj-1", "my-project", "env-1", "production"); err != nil {
		t.Fatalf("unexpected error for single-environment project: %v", err)
	}
}

func TestGuardServiceCreationScope_ProjectTokenMultiEnv(t *testing.T) {
	mock := &api.MockClient{
		IsProjectTokenFunc: func() (bool, error) {
			return true, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return []types.Environment{
				{ID: "env-1", Name: "production"},
				{ID: "env-2", Name: "staging"},
				{ID: "env-3", Name: "dev"},
			}, nil
		},
	}

	err := GuardServiceCreationScope(mock, "proj-1", "my-project", "env-1", "production")
	if err == nil {
		t.Fatal("expected error for multi-environment project under a project token, got nil")
	}
	if !strings.Contains(err.Error(), "leak") {
		t.Errorf("error should mention the leak: %v", err)
	}
	if !strings.Contains(err.Error(), "multi-environment") {
		t.Errorf("error should mention multi-environment: %v", err)
	}
	if !strings.Contains(err.Error(), "staging") || !strings.Contains(err.Error(), "dev") {
		t.Errorf("error should name the environments it would leak into (staging, dev): %v", err)
	}
	if !strings.Contains(err.Error(), "'production'") {
		t.Errorf("error should name the token's scoped environment: %v", err)
	}
}

func TestGuardServiceCreationScope_ListEnvironmentsError(t *testing.T) {
	sentinel := errors.New("network error")
	mock := &api.MockClient{
		IsProjectTokenFunc: func() (bool, error) {
			return true, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return nil, sentinel
		},
	}

	err := GuardServiceCreationScope(mock, "proj-1", "my-project", "env-1", "production")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected wrapped sentinel error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "failed to list environments") {
		t.Errorf("error should mention the environment listing failure: %v", err)
	}
}

func TestGuardServiceCreationScope_DetectionError(t *testing.T) {
	sentinel := errors.New("network error")
	mock := &api.MockClient{
		IsProjectTokenFunc: func() (bool, error) {
			return false, sentinel
		},
	}

	err := GuardServiceCreationScope(mock, "proj-1", "my-project", "env-1", "production")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected wrapped sentinel error, got: %v", err)
	}
}
