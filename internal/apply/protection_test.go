package apply

import (
	"io"
	"testing"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/config"
	"github.com/kubenoops/railctl/internal/diff"
)

func TestApply_EnvironmentDeleteProtection_SetsAndPreserves(t *testing.T) {
	var captured map[string]string
	mock := &api.MockClient{
		GetSharedVariablesFunc: func(projectID, environmentID string) (map[string]string, error) {
			return map[string]string{"OTHER": "keep"}, nil
		},
		SetSharedVariablesFunc: func(projectID, environmentID string, variables map[string]string) error {
			captured = variables
			return nil
		},
	}

	cs := &diff.ChangeSet{
		Environment: &diff.EnvironmentChange{DeleteProtection: true},
	}

	result := Apply(mock, cs, "proj-1", "env-1", map[string]config.ServiceConfig{}, Opts{Output: io.Discard})
	if len(result.Errors) != 0 {
		t.Fatalf("expected no errors, got %v", result.Errors)
	}
	if captured["DELETE_PROTECTION"] != "true" {
		t.Errorf("expected DELETE_PROTECTION=true, got %q", captured["DELETE_PROTECTION"])
	}
	if captured["OTHER"] != "keep" {
		t.Errorf("expected OTHER preserved, got %v", captured)
	}
}

func TestApply_EnvironmentDeleteProtection_ClearWritesFalse(t *testing.T) {
	var captured map[string]string
	mock := &api.MockClient{
		GetSharedVariablesFunc: func(projectID, environmentID string) (map[string]string, error) {
			return map[string]string{"DELETE_PROTECTION": "true"}, nil
		},
		SetSharedVariablesFunc: func(projectID, environmentID string, variables map[string]string) error {
			captured = variables
			return nil
		},
	}

	cs := &diff.ChangeSet{
		Environment: &diff.EnvironmentChange{DeleteProtection: false, CurrentDeleteProtection: true},
	}

	result := Apply(mock, cs, "proj-1", "env-1", map[string]config.ServiceConfig{}, Opts{Output: io.Discard})
	if len(result.Errors) != 0 {
		t.Fatalf("expected no errors, got %v", result.Errors)
	}
	if captured["DELETE_PROTECTION"] != "false" {
		t.Errorf("expected DELETE_PROTECTION=false, got %q", captured["DELETE_PROTECTION"])
	}
}

func TestApply_NilEnvironment_NeverWritesSharedVars(t *testing.T) {
	setCalled := false
	mock := &api.MockClient{
		GetSharedVariablesFunc: func(projectID, environmentID string) (map[string]string, error) {
			t.Fatal("GetSharedVariables must not be called when Environment is nil")
			return nil, nil
		},
		SetSharedVariablesFunc: func(projectID, environmentID string, variables map[string]string) error {
			setCalled = true
			return nil
		},
	}

	// No environment change, no service changes.
	cs := &diff.ChangeSet{}
	result := Apply(mock, cs, "proj-1", "env-1", map[string]config.ServiceConfig{}, Opts{Output: io.Discard})
	if len(result.Errors) != 0 {
		t.Fatalf("expected no errors, got %v", result.Errors)
	}
	if setCalled {
		t.Error("expected SetSharedVariables never called when Environment is nil")
	}
}

func TestApply_EnvironmentDeleteProtection_DryRun(t *testing.T) {
	setCalled := false
	mock := &api.MockClient{
		GetSharedVariablesFunc: func(projectID, environmentID string) (map[string]string, error) {
			t.Fatal("dry-run must not read shared variables")
			return nil, nil
		},
		SetSharedVariablesFunc: func(projectID, environmentID string, variables map[string]string) error {
			setCalled = true
			return nil
		},
	}
	cs := &diff.ChangeSet{Environment: &diff.EnvironmentChange{DeleteProtection: true}}
	Apply(mock, cs, "proj-1", "env-1", map[string]config.ServiceConfig{}, Opts{Output: io.Discard, DryRun: true})
	if setCalled {
		t.Error("dry-run must not write shared variables")
	}
}
