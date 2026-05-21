package cmd

import (
	"testing"
	"time"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/output"
	"github.com/kubenoops/railctl/internal/types"
)

func TestEnvironmentsToTable(t *testing.T) {
	envs := []types.Environment{
		{ID: "env-1", Name: "production", ServiceCount: 3, UpdatedAt: time.Now().Add(-1 * time.Hour)},
		{ID: "env-2", Name: "staging", ServiceCount: 1, UpdatedAt: time.Now().Add(-2 * time.Hour)},
	}

	table := environmentsToTable(envs)
	if table.RowCount() != 2 {
		t.Errorf("expected 2 rows, got %d", table.RowCount())
	}
}

func TestEnvironmentsToTable_Empty(t *testing.T) {
	envs := []types.Environment{}

	table := environmentsToTable(envs)
	if table.RowCount() != 0 {
		t.Errorf("expected 0 rows, got %d", table.RowCount())
	}
}

func TestEnvironmentsToTable_WithZeroTime(t *testing.T) {
	envs := []types.Environment{
		{ID: "env-1", Name: "production", ServiceCount: 0},
	}

	table := environmentsToTable(envs)
	if table.RowCount() != 1 {
		t.Errorf("expected 1 row, got %d", table.RowCount())
	}
}

func TestEnvironmentsToOutput(t *testing.T) {
	now := time.Now()
	envs := []types.Environment{
		{ID: "env-1", Name: "production", ServiceCount: 3, UpdatedAt: now},
		{ID: "env-2", Name: "staging", ServiceCount: 1, UpdatedAt: now},
	}

	result := environmentsToOutput(envs)
	if len(result) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result))
	}
	if result[0].Name != "production" {
		t.Errorf("expected 'production', got %q", result[0].Name)
	}
	if result[0].ID != "env-1" {
		t.Errorf("expected 'env-1', got %q", result[0].ID)
	}
	if result[0].ServiceCount != 3 {
		t.Errorf("expected ServiceCount 3, got %d", result[0].ServiceCount)
	}
	if result[0].UpdatedAt == "" {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestEnvironmentsToOutput_ZeroTime(t *testing.T) {
	envs := []types.Environment{
		{ID: "env-1", Name: "production", ServiceCount: 0},
	}

	result := environmentsToOutput(envs)
	if len(result) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result))
	}
	if result[0].UpdatedAt != "" {
		t.Errorf("expected empty UpdatedAt for zero time, got %q", result[0].UpdatedAt)
	}
}

func TestEnvDetailToOutput(t *testing.T) {
	now := time.Now()
	env := types.Environment{
		ID:           "env-1",
		Name:         "production",
		ServiceCount: 2,
		Services: []types.Service{
			{ID: "svc-1", Name: "api"},
			{ID: "svc-2", Name: "web"},
		},
		UpdatedAt: now,
	}

	result := envDetailToOutput(env, "my-project")

	if result.Name != "production" {
		t.Errorf("expected 'production', got %q", result.Name)
	}
	if result.ID != "env-1" {
		t.Errorf("expected 'env-1', got %q", result.ID)
	}
	if result.Project != "my-project" {
		t.Errorf("expected 'my-project', got %q", result.Project)
	}
	if len(result.Services) != 2 {
		t.Errorf("expected 2 services, got %d", len(result.Services))
	}
	if result.Services[0].Name != "api" {
		t.Errorf("expected first service 'api', got %q", result.Services[0].Name)
	}
	if result.UpdatedAt == "" {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestEnvDetailToOutput_ZeroTime(t *testing.T) {
	env := types.Environment{
		ID:           "env-1",
		Name:         "production",
		ServiceCount: 0,
	}

	result := envDetailToOutput(env, "my-project")

	if result.UpdatedAt != "" {
		t.Errorf("expected empty UpdatedAt for zero time, got %q", result.UpdatedAt)
	}
	if len(result.Services) != 0 {
		t.Errorf("expected 0 services, got %d", len(result.Services))
	}
}

func TestPrintEnvironmentDetail(t *testing.T) {
	env := types.Environment{
		ID:           "env-1",
		Name:         "production",
		ServiceCount: 2,
		Services: []types.Service{
			{ID: "svc-1", Name: "api"},
			{ID: "svc-2", Name: "web"},
		},
		UpdatedAt: time.Now().Add(-1 * time.Hour),
	}

	// Just verify it doesn't error
	err := printEnvironmentDetail(env, "my-project")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDescribeEnvironmentCmd_Exists(t *testing.T) {
	if describeEnvironmentCmd == nil {
		t.Error("describeEnvironmentCmd is nil")
	}
	if describeEnvironmentCmd.Use != "environment <name>" {
		t.Errorf("expected Use 'environment <name>', got %q", describeEnvironmentCmd.Use)
	}
}

func TestCreateAndDeleteCommands_Exist(t *testing.T) {
	// Test that create and delete commands are properly registered
	if createCmd == nil {
		t.Error("createCmd is nil")
	}
	if deleteCmd == nil {
		t.Error("deleteCmd is nil")
	}

	// Check for expected subcommands
	createSubcmds := createCmd.Commands()
	if len(createSubcmds) < 2 {
		t.Errorf("expected at least 2 create subcommands, got %d", len(createSubcmds))
	}

	deleteSubcmds := deleteCmd.Commands()
	if len(deleteSubcmds) < 2 {
		t.Errorf("expected at least 2 delete subcommands, got %d", len(deleteSubcmds))
	}
}

func TestGetEnvironmentsCmd_Exists(t *testing.T) {
	if getEnvironmentsCmd == nil {
		t.Error("getEnvironmentsCmd is nil")
	}
	if getEnvironmentsCmd.Use != "environments" {
		t.Errorf("expected Use 'environments', got %q", getEnvironmentsCmd.Use)
	}
}

func TestCreateProjectCmd_Exists(t *testing.T) {
	if createProjectCmd == nil {
		t.Error("createProjectCmd is nil")
	}
	if createProjectCmd.Use != "project NAME" {
		t.Errorf("expected Use 'project NAME', got %q", createProjectCmd.Use)
	}
}

func TestDeleteProjectCmd_Exists(t *testing.T) {
	if deleteProjectCmd == nil {
		t.Error("deleteProjectCmd is nil")
	}
	if deleteProjectCmd.Use != "project NAME" {
		t.Errorf("expected Use 'project NAME', got %q", deleteProjectCmd.Use)
	}
}

func TestCreateEnvironmentCmd_Exists(t *testing.T) {
	if createEnvironmentCmd == nil {
		t.Error("createEnvironmentCmd is nil")
	}
	if createEnvironmentCmd.Use != "environment NAME" {
		t.Errorf("expected Use 'environment NAME', got %q", createEnvironmentCmd.Use)
	}
}

func TestDeleteEnvironmentCmd_Exists(t *testing.T) {
	if deleteEnvironmentCmd == nil {
		t.Error("deleteEnvironmentCmd is nil")
	}
	if deleteEnvironmentCmd.Use != "environment NAME" {
		t.Errorf("expected Use 'environment NAME', got %q", deleteEnvironmentCmd.Use)
	}
}

func TestNewPrinterUsage(t *testing.T) {
	// Test that printer is created correctly for different formats
	formats := []output.Format{
		output.FormatTable,
		output.FormatWide,
		output.FormatJSON,
		output.FormatYAML,
	}

	for _, format := range formats {
		printer := output.NewPrinter(format)
		if printer.Format() != format {
			t.Errorf("expected format %v, got %v", format, printer.Format())
		}
	}
}

func TestRunDeleteEnvironment_Success(t *testing.T) {
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
			DeleteEnvironmentFunc: func(id string) error {
				deletedEnvID = id
				return nil
			},
		}
	}
	project = "my-project"
	deleteEnvironmentYes = true // Skip confirmation

	cmd := deleteEnvironmentCmd
	err := cmd.RunE(cmd, []string{"staging"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if deletedEnvID != "env-1" {
		t.Errorf("expected deleted env ID 'env-1', got %q", deletedEnvID)
	}
}

func TestRunDeleteEnvironment_EnvNotFound(t *testing.T) {
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
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{}, nil
			},
		}
	}
	project = "my-project"

	cmd := deleteEnvironmentCmd
	err := cmd.RunE(cmd, []string{"nonexistent"})

	if err == nil {
		t.Error("expected error for missing environment")
	}
}
