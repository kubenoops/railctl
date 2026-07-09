package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/types"
)

// protectTestMock returns a MockClient wired with one project ("my-project")
// and one environment ("production") whose shared variables start as the given
// map. It records every SetSharedVariables call into *captured. IsProjectToken
// defaults to false (a workspace-scoped token).
func protectTestMock(initial map[string]string, captured *map[string]string) *api.MockClient {
	// Copy so the "live" map is independent of the caller's literal.
	live := make(map[string]string, len(initial))
	for k, v := range initial {
		live[k] = v
	}
	return &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return []types.Environment{{ID: "env-1", Name: "production"}}, nil
		},
		GetSharedVariablesFunc: func(projectID, environmentID string) (map[string]string, error) {
			out := make(map[string]string, len(live))
			for k, v := range live {
				out[k] = v
			}
			return out, nil
		},
		SetSharedVariablesFunc: func(projectID, environmentID string, variables map[string]string) error {
			*captured = variables
			return nil
		},
	}
}

// runProtectTest installs the mock, sets flags, and invokes the given RunE.
func runProtectTest(t *testing.T, mock *api.MockClient, cmdFn func() error, out string, format string) (string, error) {
	t.Helper()
	origAPIClient := newAPIClient
	origProject := project
	origToken := token
	origOutput := outputFormat
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		token = origToken
		outputFormat = origOutput
	}()

	token = "test-token"
	project = "my-project"
	outputFormat = format
	newAPIClient = func(tkn string) api.APIClient { return mock }

	return "", cmdFn()
}

func TestProtectEnvironment_SetsTrueAndPreservesOthers(t *testing.T) {
	var captured map[string]string
	mock := protectTestMock(map[string]string{"OTHER": "keep-me"}, &captured)

	var stdout bytes.Buffer
	protectEnvironmentCmd.SetOut(&stdout)
	defer protectEnvironmentCmd.SetOut(nil)

	_, err := runProtectTest(t, mock, func() error {
		return runProtectEnvironment(protectEnvironmentCmd, []string{"production"})
	}, "", "table")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := captured["DELETE_PROTECTION"]; got != "true" {
		t.Errorf("expected DELETE_PROTECTION=true, got %q", got)
	}
	if got := captured["OTHER"]; got != "keep-me" {
		t.Errorf("expected OTHER shared variable preserved, got %q", got)
	}
	if !strings.Contains(stdout.String(), "now delete-protected") {
		t.Errorf("expected success message, got: %q", stdout.String())
	}
}

func TestUnprotectEnvironment_SetsFalseAndPreservesOthers(t *testing.T) {
	var captured map[string]string
	mock := protectTestMock(map[string]string{
		"OTHER":             "keep-me",
		"DELETE_PROTECTION": "true",
	}, &captured)

	var stdout bytes.Buffer
	unprotectEnvironmentCmd.SetOut(&stdout)
	defer unprotectEnvironmentCmd.SetOut(nil)

	_, err := runProtectTest(t, mock, func() error {
		return runUnprotectEnvironment(unprotectEnvironmentCmd, []string{"production"})
	}, "", "table")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := captured["DELETE_PROTECTION"]; got != "false" {
		t.Errorf("expected DELETE_PROTECTION=false, got %q", got)
	}
	if got := captured["OTHER"]; got != "keep-me" {
		t.Errorf("expected OTHER shared variable preserved, got %q", got)
	}
	if !strings.Contains(stdout.String(), "no longer delete-protected") {
		t.Errorf("expected success message, got: %q", stdout.String())
	}
}

func TestProtectEnvironment_Idempotent(t *testing.T) {
	// Already protected: protecting again re-asserts true and keeps others.
	var captured map[string]string
	mock := protectTestMock(map[string]string{
		"OTHER":             "keep-me",
		"DELETE_PROTECTION": "true",
	}, &captured)

	var stdout bytes.Buffer
	protectEnvironmentCmd.SetOut(&stdout)
	defer protectEnvironmentCmd.SetOut(nil)

	_, err := runProtectTest(t, mock, func() error {
		return runProtectEnvironment(protectEnvironmentCmd, []string{"production"})
	}, "", "table")
	if err != nil {
		t.Fatalf("unexpected error on idempotent protect: %v", err)
	}
	if got := captured["DELETE_PROTECTION"]; got != "true" {
		t.Errorf("expected DELETE_PROTECTION=true, got %q", got)
	}
	if got := captured["OTHER"]; got != "keep-me" {
		t.Errorf("expected OTHER preserved, got %q", got)
	}
}

func TestProtectEnvironment_RejectsProjectToken(t *testing.T) {
	var captured map[string]string
	mock := protectTestMock(nil, &captured)
	mock.IsProjectTokenFunc = func() (bool, error) { return true, nil }

	_, err := runProtectTest(t, mock, func() error {
		return runProtectEnvironment(protectEnvironmentCmd, []string{"production"})
	}, "", "table")
	if err == nil {
		t.Fatal("expected error with a project token, got nil")
	}
	if !strings.Contains(err.Error(), "project token") {
		t.Errorf("expected project-token error, got: %v", err)
	}
	if captured != nil {
		t.Errorf("expected no shared-variable write with a project token, got %v", captured)
	}
}

func TestProtectEnvironment_JSONOutput(t *testing.T) {
	var captured map[string]string
	mock := protectTestMock(nil, &captured)

	var stdout bytes.Buffer
	protectEnvironmentCmd.SetOut(&stdout)
	defer protectEnvironmentCmd.SetOut(nil)

	_, err := runProtectTest(t, mock, func() error {
		return runProtectEnvironment(protectEnvironmentCmd, []string{"production"})
	}, "", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured["DELETE_PROTECTION"] != "true" {
		t.Errorf("expected DELETE_PROTECTION=true, got %q", captured["DELETE_PROTECTION"])
	}
}
