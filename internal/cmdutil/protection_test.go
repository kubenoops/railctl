package cmdutil

import (
	"errors"
	"strings"
	"testing"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/types"
)

func sharedVarsClient(vars map[string]string) *api.MockClient {
	return &api.MockClient{
		GetSharedVariablesFunc: func(projectID, environmentID string) (map[string]string, error) {
			return vars, nil
		},
	}
}

func TestCheckDeleteProtection_TruthyValues(t *testing.T) {
	truthy := []string{"true", "TRUE", "True", " true ", "1", "yes", "YES", "on", "ON", "\ton\n"}

	for _, value := range truthy {
		t.Run(value, func(t *testing.T) {
			client := sharedVarsClient(map[string]string{DeleteProtectionVar: value})
			env := types.Environment{ID: "env-1", Name: "production"}

			err := CheckDeleteProtection(client, "proj-1", env)
			if err == nil {
				t.Fatalf("expected error for DELETE_PROTECTION=%q, got nil", value)
			}
			if !strings.Contains(err.Error(), DeleteProtectionVar) {
				t.Errorf("expected error to mention %s, got: %v", DeleteProtectionVar, err)
			}
			if !strings.Contains(err.Error(), "production") {
				t.Errorf("expected error to name the environment, got: %v", err)
			}
			if !strings.Contains(err.Error(), "delete-protected") {
				t.Errorf("expected error to say delete-protected, got: %v", err)
			}
		})
	}
}

func TestCheckDeleteProtection_FalsyValues(t *testing.T) {
	falsy := []string{"false", "FALSE", "0", "no", "off", "OFF", "", "  ", "garbage", "enabled", "2"}

	for _, value := range falsy {
		t.Run("value="+value, func(t *testing.T) {
			client := sharedVarsClient(map[string]string{DeleteProtectionVar: value})
			env := types.Environment{ID: "env-1", Name: "production"}

			if err := CheckDeleteProtection(client, "proj-1", env); err != nil {
				t.Errorf("expected nil for DELETE_PROTECTION=%q, got: %v", value, err)
			}
		})
	}
}

func TestCheckDeleteProtection_VariableAbsent(t *testing.T) {
	client := sharedVarsClient(map[string]string{"OTHER_VAR": "true"})
	env := types.Environment{ID: "env-1", Name: "production"}

	if err := CheckDeleteProtection(client, "proj-1", env); err != nil {
		t.Errorf("expected nil when DELETE_PROTECTION is absent, got: %v", err)
	}
}

func TestCheckDeleteProtection_NoSharedVariables(t *testing.T) {
	client := sharedVarsClient(map[string]string{})
	env := types.Environment{ID: "env-1", Name: "production"}

	if err := CheckDeleteProtection(client, "proj-1", env); err != nil {
		t.Errorf("expected nil with no shared variables, got: %v", err)
	}
}

func TestCheckDeleteProtection_ReadErrorFailsClosed(t *testing.T) {
	apiErr := errors.New("boom")
	client := &api.MockClient{
		GetSharedVariablesFunc: func(projectID, environmentID string) (map[string]string, error) {
			return nil, apiErr
		},
	}
	env := types.Environment{ID: "env-1", Name: "production"}

	err := CheckDeleteProtection(client, "proj-1", env)
	if err == nil {
		t.Fatal("expected error when shared variables cannot be read (fail closed), got nil")
	}
	if !errors.Is(err, apiErr) {
		t.Errorf("expected wrapped API error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "production") {
		t.Errorf("expected error to name the environment, got: %v", err)
	}
}

func TestCheckDeleteProtection_PassesIDs(t *testing.T) {
	var gotProjectID, gotEnvironmentID string
	client := &api.MockClient{
		GetSharedVariablesFunc: func(projectID, environmentID string) (map[string]string, error) {
			gotProjectID = projectID
			gotEnvironmentID = environmentID
			return map[string]string{}, nil
		},
	}
	env := types.Environment{ID: "env-42", Name: "staging"}

	if err := CheckDeleteProtection(client, "proj-7", env); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotProjectID != "proj-7" {
		t.Errorf("expected projectID 'proj-7', got %q", gotProjectID)
	}
	if gotEnvironmentID != "env-42" {
		t.Errorf("expected environmentID 'env-42', got %q", gotEnvironmentID)
	}
}

func TestCheckProjectDeleteProtection_CollectsAllProtected(t *testing.T) {
	client := &api.MockClient{
		GetSharedVariablesFunc: func(projectID, environmentID string) (map[string]string, error) {
			switch environmentID {
			case "env-1":
				return map[string]string{DeleteProtectionVar: "true"}, nil
			case "env-3":
				return map[string]string{DeleteProtectionVar: "yes"}, nil
			default:
				return map[string]string{}, nil
			}
		},
	}
	project := types.Project{ID: "proj-1", Name: "my-app"}
	envs := []types.Environment{
		{ID: "env-1", Name: "production"},
		{ID: "env-2", Name: "staging"},
		{ID: "env-3", Name: "qa"},
	}

	err := CheckProjectDeleteProtection(client, project, envs)
	if err == nil {
		t.Fatal("expected error for project with protected environments, got nil")
	}
	for _, want := range []string{"my-app", "production", "qa", DeleteProtectionVar, "delete-protected"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected error to contain %q, got: %v", want, err)
		}
	}
	if strings.Contains(err.Error(), "staging") {
		t.Errorf("expected error not to name unprotected environment 'staging', got: %v", err)
	}
}

func TestCheckProjectDeleteProtection_NoneProtected(t *testing.T) {
	client := sharedVarsClient(map[string]string{DeleteProtectionVar: "false"})
	project := types.Project{ID: "proj-1", Name: "my-app"}
	envs := []types.Environment{
		{ID: "env-1", Name: "production"},
		{ID: "env-2", Name: "staging"},
	}

	if err := CheckProjectDeleteProtection(client, project, envs); err != nil {
		t.Errorf("expected nil for unprotected project, got: %v", err)
	}
}

func TestCheckProjectDeleteProtection_NoEnvironments(t *testing.T) {
	client := sharedVarsClient(map[string]string{})
	project := types.Project{ID: "proj-1", Name: "my-app"}

	if err := CheckProjectDeleteProtection(client, project, nil); err != nil {
		t.Errorf("expected nil for project with no environments, got: %v", err)
	}
}

func TestCheckProjectDeleteProtection_ReadErrorFailsClosed(t *testing.T) {
	apiErr := errors.New("boom")
	client := &api.MockClient{
		GetSharedVariablesFunc: func(projectID, environmentID string) (map[string]string, error) {
			return nil, apiErr
		},
	}
	project := types.Project{ID: "proj-1", Name: "my-app"}
	envs := []types.Environment{{ID: "env-1", Name: "production"}}

	err := CheckProjectDeleteProtection(client, project, envs)
	if err == nil {
		t.Fatal("expected error when shared variables cannot be read (fail closed), got nil")
	}
	if !errors.Is(err, apiErr) {
		t.Errorf("expected wrapped API error, got: %v", err)
	}
}

func TestIsTruthy(t *testing.T) {
	cases := []struct {
		value string
		want  bool
	}{
		{"true", true},
		{"TRUE", true},
		{" True ", true},
		{"1", true},
		{"yes", true},
		{"YES", true},
		{"on", true},
		{"On", true},
		{"", false},
		{"   ", false},
		{"false", false},
		{"0", false},
		{"no", false},
		{"off", false},
		{"garbage", false},
		{"truee", false},
		{"y", false},
		{"enabled", false},
	}

	for _, tc := range cases {
		if got := isTruthy(tc.value); got != tc.want {
			t.Errorf("isTruthy(%q) = %v, want %v", tc.value, got, tc.want)
		}
	}
}
