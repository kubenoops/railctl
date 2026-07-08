package cmdutil

import (
	"errors"
	"strings"
	"testing"

	"github.com/kubenoops/railctl/internal/api"
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
