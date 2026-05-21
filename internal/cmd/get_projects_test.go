package cmd

import (
	"testing"
	"time"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/types"
)

func TestProjectsToTable(t *testing.T) {
	projects := []types.Project{
		{
			ID:        "1",
			Name:      "my-app",
			UpdatedAt: time.Now().Add(-2 * time.Hour),
			Services:  []types.Service{{ID: "s1", Name: "api"}},
		},
		{
			ID:        "2",
			Name:      "other-app",
			UpdatedAt: time.Now().Add(-5 * 24 * time.Hour),
			Services: []types.Service{
				{ID: "s1", Name: "api"},
				{ID: "s2", Name: "worker"},
			},
		},
	}

	table := projectsToTable(projects)

	if table.RowCount() != 2 {
		t.Errorf("expected 2 rows, got %d", table.RowCount())
	}
}

func TestProjectsToWideTable(t *testing.T) {
	projects := []types.Project{
		{
			ID:   "1",
			Name: "my-app",
			Environments: []types.Environment{
				{ID: "e1", Name: "production"},
				{ID: "e2", Name: "staging"},
			},
			Services: []types.Service{
				{ID: "s1", Name: "api"},
			},
		},
	}

	table := projectsToWideTable(projects)

	if table.RowCount() != 1 {
		t.Errorf("expected 1 row, got %d", table.RowCount())
	}
}

func TestProjectsToOutput(t *testing.T) {
	projects := []types.Project{
		{
			ID:        "proj-1",
			Name:      "my-app",
			UpdatedAt: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			Environments: []types.Environment{
				{ID: "env-1", Name: "production"},
			},
			Services: []types.Service{
				{ID: "svc-1", Name: "api"},
			},
		},
	}

	result := projectsToOutput(projects)

	if len(result) != 1 {
		t.Fatalf("expected 1 project, got %d", len(result))
	}

	p := result[0]
	if p.Name != "my-app" {
		t.Errorf("expected name 'my-app', got %q", p.Name)
	}
	if p.ID != "proj-1" {
		t.Errorf("expected ID 'proj-1', got %q", p.ID)
	}
	if len(p.Environments) != 1 {
		t.Errorf("expected 1 environment, got %d", len(p.Environments))
	}
	if len(p.Services) != 1 {
		t.Errorf("expected 1 service, got %d", len(p.Services))
	}
	if p.Environments[0].Name != "production" {
		t.Errorf("expected env name 'production', got %q", p.Environments[0].Name)
	}
}

func TestFormatEnvList(t *testing.T) {
	tests := []struct {
		name     string
		envs     []types.Environment
		expected string
	}{
		{
			name:     "empty",
			envs:     nil,
			expected: "",
		},
		{
			name:     "single",
			envs:     []types.Environment{{Name: "production"}},
			expected: "production",
		},
		{
			name: "multiple",
			envs: []types.Environment{
				{Name: "production"},
				{Name: "staging"},
			},
			expected: "production, staging",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := formatEnvList(tc.envs)
			if result != tc.expected {
				t.Errorf("formatEnvList() = %q, expected %q", result, tc.expected)
			}
		})
	}
}

func TestFormatSvcList(t *testing.T) {
	tests := []struct {
		name     string
		svcs     []types.Service
		expected string
	}{
		{
			name:     "empty",
			svcs:     nil,
			expected: "",
		},
		{
			name:     "single",
			svcs:     []types.Service{{Name: "api"}},
			expected: "api",
		},
		{
			name: "multiple",
			svcs: []types.Service{
				{Name: "api"},
				{Name: "worker"},
			},
			expected: "api, worker",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := formatSvcList(tc.svcs)
			if result != tc.expected {
				t.Errorf("formatSvcList() = %q, expected %q", result, tc.expected)
			}
		})
	}
}

func TestProjectDetailToOutput(t *testing.T) {
	project := types.Project{
		ID:        "proj-1",
		Name:      "my-app",
		UpdatedAt: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Environments: []types.Environment{
			{
				ID:   "env-1",
				Name: "production",
				Services: []types.Service{
					{ID: "svc-1", Name: "api"},
				},
			},
			{
				ID:       "env-2",
				Name:     "staging",
				Services: []types.Service{},
			},
		},
		Services: []types.Service{
			{ID: "svc-1", Name: "api"},
		},
	}

	result := projectDetailToOutput(project)

	if result.Name != "my-app" {
		t.Errorf("expected name 'my-app', got %q", result.Name)
	}
	if result.ID != "proj-1" {
		t.Errorf("expected ID 'proj-1', got %q", result.ID)
	}
	if len(result.Environments) != 2 {
		t.Errorf("expected 2 environments, got %d", len(result.Environments))
	}
	if len(result.Environments[0].Services) != 1 {
		t.Errorf("expected 1 service in production, got %d", len(result.Environments[0].Services))
	}
	if result.Environments[0].Services[0].Name != "api" {
		t.Errorf("expected service name 'api', got %q", result.Environments[0].Services[0].Name)
	}
}

func TestPrintProjectDetail(t *testing.T) {
	project := types.Project{
		ID:        "proj-1",
		Name:      "my-app",
		UpdatedAt: time.Now().Add(-1 * time.Hour),
		Environments: []types.Environment{
			{ID: "env-1", Name: "production"},
		},
		Services: []types.Service{
			{ID: "svc-1", Name: "api"},
		},
	}

	// Just verify it doesn't error
	err := printProjectDetail(project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Integration-style test for the output switching logic
func TestGetProjects_OutputFormats(t *testing.T) {
	projects := []types.Project{
		{
			ID:        "1",
			Name:      "test-app",
			UpdatedAt: time.Now(),
			Services:  []types.Service{{ID: "s1", Name: "api"}},
		},
	}

	// Test table format
	tableOut := projectsToTable(projects)
	if tableOut.RowCount() != 1 {
		t.Errorf("table should have 1 row")
	}

	// Test wide format
	wideOut := projectsToWideTable(projects)
	if wideOut.RowCount() != 1 {
		t.Errorf("wide table should have 1 row")
	}

	// Test JSON output structure
	jsonOut := projectsToOutput(projects)
	if len(jsonOut) != 1 {
		t.Errorf("JSON output should have 1 project")
	}
}

func TestRunDeleteProject_Success(t *testing.T) {
	origAPIClient := newAPIClient
	origToken := token
	origYes := deleteProjectYes
	defer func() {
		newAPIClient = origAPIClient
		token = origToken
		deleteProjectYes = origYes
	}()

	var deletedProjectID string

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			DeleteProjectFunc: func(id string) error {
				deletedProjectID = id
				return nil
			},
		}
	}
	deleteProjectYes = true // Skip confirmation

	cmd := deleteProjectCmd
	err := cmd.RunE(cmd, []string{"my-project"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if deletedProjectID != "proj-1" {
		t.Errorf("expected deleted project ID 'proj-1', got %q", deletedProjectID)
	}
}
