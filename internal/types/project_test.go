package types

import (
	"testing"
	"time"
)

func TestProject_EnvironmentCount(t *testing.T) {
	tests := []struct {
		name     string
		project  Project
		expected int
	}{
		{
			name:     "no environments",
			project:  Project{},
			expected: 0,
		},
		{
			name: "multiple environments",
			project: Project{
				Environments: []Environment{
					{ID: "1", Name: "prod"},
					{ID: "2", Name: "staging"},
				},
			},
			expected: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.project.EnvironmentCount()
			if result != tc.expected {
				t.Errorf("EnvironmentCount() = %d, expected %d", result, tc.expected)
			}
		})
	}
}

func TestProject_ServiceCount(t *testing.T) {
	tests := []struct {
		name     string
		project  Project
		expected int
	}{
		{
			name:     "no services",
			project:  Project{},
			expected: 0,
		},
		{
			name: "multiple services",
			project: Project{
				Services: []Service{
					{ID: "1", Name: "api"},
					{ID: "2", Name: "worker"},
					{ID: "3", Name: "frontend"},
				},
			},
			expected: 3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.project.ServiceCount()
			if result != tc.expected {
				t.Errorf("ServiceCount() = %d, expected %d", result, tc.expected)
			}
		})
	}
}

func TestProject_EnvironmentNames(t *testing.T) {
	tests := []struct {
		name     string
		project  Project
		expected string
	}{
		{
			name:     "no environments",
			project:  Project{},
			expected: "",
		},
		{
			name: "single environment",
			project: Project{
				Environments: []Environment{{ID: "1", Name: "production"}},
			},
			expected: "production",
		},
		{
			name: "multiple environments",
			project: Project{
				Environments: []Environment{
					{ID: "1", Name: "production"},
					{ID: "2", Name: "staging"},
				},
			},
			expected: "production, staging",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.project.EnvironmentNames()
			if result != tc.expected {
				t.Errorf("EnvironmentNames() = %q, expected %q", result, tc.expected)
			}
		})
	}
}

func TestProject_ServiceNames(t *testing.T) {
	tests := []struct {
		name     string
		project  Project
		expected string
	}{
		{
			name:     "no services",
			project:  Project{},
			expected: "",
		},
		{
			name: "single service",
			project: Project{
				Services: []Service{{ID: "1", Name: "api"}},
			},
			expected: "api",
		},
		{
			name: "multiple services",
			project: Project{
				Services: []Service{
					{ID: "1", Name: "api"},
					{ID: "2", Name: "worker"},
				},
			},
			expected: "api, worker",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.project.ServiceNames()
			if result != tc.expected {
				t.Errorf("ServiceNames() = %q, expected %q", result, tc.expected)
			}
		})
	}
}

func TestProject_FullStructure(t *testing.T) {
	p := Project{
		ID:        "proj-123",
		Name:      "my-app",
		UpdatedAt: time.Now(),
		Environments: []Environment{
			{ID: "env-1", Name: "production"},
		},
		Services: []Service{
			{ID: "svc-1", Name: "api"},
		},
	}

	if p.ID != "proj-123" {
		t.Errorf("ID = %q, expected 'proj-123'", p.ID)
	}
	if p.Name != "my-app" {
		t.Errorf("Name = %q, expected 'my-app'", p.Name)
	}
	if p.EnvironmentCount() != 1 {
		t.Errorf("EnvironmentCount() = %d, expected 1", p.EnvironmentCount())
	}
	if p.ServiceCount() != 1 {
		t.Errorf("ServiceCount() = %d, expected 1", p.ServiceCount())
	}
}
