package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/types"
)

// newDeleteProtectionMock builds a MockClient for delete-protection tests.
// sharedVars maps environmentID -> shared variables for that environment.
func newDeleteProtectionMock(sharedVars map[string]map[string]string, deletedEnvID, deletedProjectID *string) *api.MockClient {
	return &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return []types.Environment{
				{ID: "env-1", Name: "staging"},
				{ID: "env-2", Name: "production"},
			}, nil
		},
		GetSharedVariablesFunc: func(projectID, environmentID string) (map[string]string, error) {
			if vars, ok := sharedVars[environmentID]; ok {
				return vars, nil
			}
			return map[string]string{}, nil
		},
		DeleteEnvironmentFunc: func(id string) error {
			*deletedEnvID = id
			return nil
		},
		DeleteProjectFunc: func(id string) error {
			*deletedProjectID = id
			return nil
		},
	}
}

func TestRunDeleteEnvironment_Protected(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origToken := token
	origYes := deleteEnvironmentYes
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		token = origToken
		deleteEnvironmentYes = origYes
	}()

	var deletedEnvID, deletedProjectID string

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return newDeleteProtectionMock(map[string]map[string]string{
			"env-1": {"DELETE_PROTECTION": "true"},
		}, &deletedEnvID, &deletedProjectID)
	}
	project = "my-project"
	// Deliberately no --yes: the protection check must fire BEFORE the
	// confirmation prompt. If it ran after, the prompt would read EOF from
	// the test's stdin and RunE would return nil ("Cancelled.").
	deleteEnvironmentYes = false

	cmd := deleteEnvironmentCmd
	err := cmd.RunE(cmd, []string{"staging"})

	if err == nil {
		t.Fatal("expected delete-protection error, got nil")
	}
	if !strings.Contains(err.Error(), "delete-protected") {
		t.Errorf("expected error to say delete-protected, got: %v", err)
	}
	if !strings.Contains(err.Error(), "DELETE_PROTECTION") {
		t.Errorf("expected error to mention DELETE_PROTECTION, got: %v", err)
	}
	if deletedEnvID != "" {
		t.Errorf("expected DeleteEnvironment NOT to be called, but it deleted %q", deletedEnvID)
	}
}

func TestRunDeleteEnvironment_ProtectedYesDoesNotOverride(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origToken := token
	origYes := deleteEnvironmentYes
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		token = origToken
		deleteEnvironmentYes = origYes
	}()

	var deletedEnvID, deletedProjectID string

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return newDeleteProtectionMock(map[string]map[string]string{
			"env-1": {"DELETE_PROTECTION": "1"},
		}, &deletedEnvID, &deletedProjectID)
	}
	project = "my-project"
	deleteEnvironmentYes = true // --yes skips the prompt, not the protection

	cmd := deleteEnvironmentCmd
	err := cmd.RunE(cmd, []string{"staging"})

	if err == nil {
		t.Fatal("expected delete-protection error despite --yes, got nil")
	}
	if !strings.Contains(err.Error(), "delete-protected") {
		t.Errorf("expected error to say delete-protected, got: %v", err)
	}
	if deletedEnvID != "" {
		t.Errorf("expected DeleteEnvironment NOT to be called, but it deleted %q", deletedEnvID)
	}
}

func TestRunDeleteEnvironment_UnprotectedProceeds(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origToken := token
	origYes := deleteEnvironmentYes
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		token = origToken
		deleteEnvironmentYes = origYes
	}()

	var deletedEnvID, deletedProjectID string

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return newDeleteProtectionMock(map[string]map[string]string{
			"env-1": {"DELETE_PROTECTION": "false"},
		}, &deletedEnvID, &deletedProjectID)
	}
	project = "my-project"
	deleteEnvironmentYes = true

	cmd := deleteEnvironmentCmd
	err := cmd.RunE(cmd, []string{"staging"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deletedEnvID != "env-1" {
		t.Errorf("expected deleted env ID 'env-1', got %q", deletedEnvID)
	}
}

func TestRunDeleteEnvironment_SharedVarsReadErrorFailsClosed(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origToken := token
	origYes := deleteEnvironmentYes
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		token = origToken
		deleteEnvironmentYes = origYes
	}()

	var deletedEnvID string

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{
					{ID: "env-1", Name: "staging"},
					{ID: "env-2", Name: "production"},
				}, nil
			},
			GetSharedVariablesFunc: func(projectID, environmentID string) (map[string]string, error) {
				return nil, errors.New("Not Authorized")
			},
			DeleteEnvironmentFunc: func(id string) error {
				deletedEnvID = id
				return nil
			},
		}
	}
	project = "my-project"
	deleteEnvironmentYes = true

	cmd := deleteEnvironmentCmd
	err := cmd.RunE(cmd, []string{"staging"})

	if err == nil {
		t.Fatal("expected fail-closed error when shared variables cannot be read, got nil")
	}
	if !strings.Contains(err.Error(), "delete protection") {
		t.Errorf("expected error to mention delete protection, got: %v", err)
	}
	if deletedEnvID != "" {
		t.Errorf("expected DeleteEnvironment NOT to be called, but it deleted %q", deletedEnvID)
	}
}

func TestRunDeleteProject_ProtectedEnvironmentBlocks(t *testing.T) {
	origAPIClient := newAPIClient
	origToken := token
	origYes := deleteProjectYes
	defer func() {
		newAPIClient = origAPIClient
		token = origToken
		deleteProjectYes = origYes
	}()

	var deletedEnvID, deletedProjectID string

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return newDeleteProtectionMock(map[string]map[string]string{
			"env-2": {"DELETE_PROTECTION": "yes"},
		}, &deletedEnvID, &deletedProjectID)
	}
	// Deliberately no --yes: protection must fire before the prompt.
	deleteProjectYes = false

	cmd := deleteProjectCmd
	err := cmd.RunE(cmd, []string{"my-project"})

	if err == nil {
		t.Fatal("expected delete-protection error, got nil")
	}
	if !strings.Contains(err.Error(), "delete-protected") {
		t.Errorf("expected error to say delete-protected, got: %v", err)
	}
	if !strings.Contains(err.Error(), "production") {
		t.Errorf("expected error to name protected environment 'production', got: %v", err)
	}
	if strings.Contains(err.Error(), "staging") {
		t.Errorf("expected error not to name unprotected environment 'staging', got: %v", err)
	}
	if deletedProjectID != "" {
		t.Errorf("expected DeleteProject NOT to be called, but it deleted %q", deletedProjectID)
	}
}

func TestRunDeleteProject_UnprotectedProceeds(t *testing.T) {
	origAPIClient := newAPIClient
	origToken := token
	origYes := deleteProjectYes
	defer func() {
		newAPIClient = origAPIClient
		token = origToken
		deleteProjectYes = origYes
	}()

	var deletedEnvID, deletedProjectID string

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return newDeleteProtectionMock(nil, &deletedEnvID, &deletedProjectID)
	}
	deleteProjectYes = true

	cmd := deleteProjectCmd
	err := cmd.RunE(cmd, []string{"my-project"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deletedProjectID != "proj-1" {
		t.Errorf("expected deleted project ID 'proj-1', got %q", deletedProjectID)
	}
}
