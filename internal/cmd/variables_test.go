package cmd

import (
	"testing"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/types"
)

func TestRunGetVariables_MissingProject(t *testing.T) {
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
		return &api.MockClient{}
	}
	project = ""

	cmd := getVariablesCmd
	err := cmd.RunE(cmd, []string{})

	if err == nil {
		t.Error("expected error for missing project")
	}
}

func TestRunGetVariables_MissingEnvironment(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
	}()

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
		}
	}
	project = "my-project"
	environment = ""

	cmd := getVariablesCmd
	err := cmd.RunE(cmd, []string{})

	if err == nil {
		t.Error("expected error for missing environment")
	}
}

func TestRunGetVariables_MissingService(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origService := service
	origToken := token
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		service = origService
		token = origToken
	}()

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	service = ""

	cmd := getVariablesCmd
	err := cmd.RunE(cmd, []string{})

	if err == nil {
		t.Error("expected error for missing service")
	}
}

func TestRunGetVariables_Success(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origService := service
	origToken := token
	origFormat := outputFormat
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		service = origService
		token = origToken
		outputFormat = origFormat
	}()

	token = "test-token"
	outputFormat = ""
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{
					{ID: "svc-1", Name: "api"},
				}, nil
			},
			GetVariablesFunc: func(projectID, environmentID, serviceID string) (map[string]string, error) {
				return map[string]string{
					"PORT":     "3000",
					"NODE_ENV": "production",
				}, nil
			},
			GetSealedVariablesFunc: func(environmentID, serviceID string) (map[string]bool, error) {
				return map[string]bool{
					"PORT":     false,
					"NODE_ENV": false,
				}, nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	service = "api"

	cmd := getVariablesCmd
	err := cmd.RunE(cmd, []string{})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunGetVariables_WithSealedVars(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origService := service
	origToken := token
	origFormat := outputFormat
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		service = origService
		token = origToken
		outputFormat = origFormat
	}()

	token = "test-token"
	outputFormat = ""
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{
					{ID: "svc-1", Name: "api"},
				}, nil
			},
			GetVariablesFunc: func(projectID, environmentID, serviceID string) (map[string]string, error) {
				return map[string]string{
					"PORT": "3000",
				}, nil
			},
			GetSealedVariablesFunc: func(environmentID, serviceID string) (map[string]bool, error) {
				return map[string]bool{
					"PORT":       false,
					"API_SECRET": true, // Sealed var not in GetVariables response
				}, nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	service = "api"

	cmd := getVariablesCmd
	err := cmd.RunE(cmd, []string{})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunGetVariables_JSONOutput(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origService := service
	origToken := token
	origFormat := outputFormat
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		service = origService
		token = origToken
		outputFormat = origFormat
	}()

	token = "test-token"
	outputFormat = "json"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{
					{ID: "svc-1", Name: "api"},
				}, nil
			},
			GetVariablesFunc: func(projectID, environmentID, serviceID string) (map[string]string, error) {
				return map[string]string{"PORT": "3000"}, nil
			},
			GetSealedVariablesFunc: func(environmentID, serviceID string) (map[string]bool, error) {
				return map[string]bool{"PORT": false}, nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	service = "api"

	cmd := getVariablesCmd
	err := cmd.RunE(cmd, []string{})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunGetVariables_YAMLOutput(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origService := service
	origToken := token
	origFormat := outputFormat
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		service = origService
		token = origToken
		outputFormat = origFormat
	}()

	token = "test-token"
	outputFormat = "yaml"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{
					{ID: "svc-1", Name: "api"},
				}, nil
			},
			GetVariablesFunc: func(projectID, environmentID, serviceID string) (map[string]string, error) {
				return map[string]string{"PORT": "3000"}, nil
			},
			GetSealedVariablesFunc: func(environmentID, serviceID string) (map[string]bool, error) {
				return map[string]bool{"PORT": false}, nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	service = "api"

	cmd := getVariablesCmd
	err := cmd.RunE(cmd, []string{})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunGetVariables_EmptyVars(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origService := service
	origToken := token
	origFormat := outputFormat
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		service = origService
		token = origToken
		outputFormat = origFormat
	}()

	token = "test-token"
	outputFormat = ""
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{
					{ID: "svc-1", Name: "api"},
				}, nil
			},
			GetVariablesFunc: func(projectID, environmentID, serviceID string) (map[string]string, error) {
				return map[string]string{}, nil
			},
			GetSealedVariablesFunc: func(environmentID, serviceID string) (map[string]bool, error) {
				return map[string]bool{}, nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	service = "api"

	cmd := getVariablesCmd
	err := cmd.RunE(cmd, []string{})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunSetVariable_MissingProject(t *testing.T) {
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
		return &api.MockClient{}
	}
	project = ""

	cmd := setVariableCmd
	err := cmd.RunE(cmd, []string{"KEY=VALUE"})

	if err == nil {
		t.Error("expected error for missing project")
	}
}

func TestRunSetVariable_InvalidFormat(t *testing.T) {
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
		return &api.MockClient{}
	}
	project = "my-project"

	cmd := setVariableCmd
	err := cmd.RunE(cmd, []string{"INVALID_NO_EQUALS"})

	if err == nil {
		t.Error("expected error for invalid KEY=VALUE format")
	}
	if err != nil && err.Error() != "invalid variable format 'INVALID_NO_EQUALS': expected KEY=VALUE" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunSetVariable_EmptyKey(t *testing.T) {
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
		return &api.MockClient{}
	}
	project = "my-project"

	cmd := setVariableCmd
	err := cmd.RunE(cmd, []string{"=value"})

	if err == nil {
		t.Error("expected error for empty key")
	}
}

func TestRunSetVariable_Success(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origService := service
	origToken := token
	origSkip := skipDeploymentFlag
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		service = origService
		token = origToken
		skipDeploymentFlag = origSkip
	}()

	var capturedVars map[string]string
	var capturedSkip bool

	token = "test-token"
	skipDeploymentFlag = false
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{
					{ID: "svc-1", Name: "api"},
				}, nil
			},
			SetVariablesFunc: func(projectID, environmentID, serviceID string, variables map[string]string, skipDeploys bool) error {
				capturedVars = variables
				capturedSkip = skipDeploys
				return nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	service = "api"

	cmd := setVariableCmd
	err := cmd.RunE(cmd, []string{"PORT=3000"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if capturedVars["PORT"] != "3000" {
		t.Errorf("expected PORT=3000, got %v", capturedVars)
	}
	if capturedSkip != false {
		t.Error("expected skipDeploys=false")
	}
}

func TestRunSetVariable_MultipleVars(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origService := service
	origToken := token
	origSkip := skipDeploymentFlag
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		service = origService
		token = origToken
		skipDeploymentFlag = origSkip
	}()

	var capturedVars map[string]string

	token = "test-token"
	skipDeploymentFlag = true
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{
					{ID: "svc-1", Name: "api"},
				}, nil
			},
			SetVariablesFunc: func(projectID, environmentID, serviceID string, variables map[string]string, skipDeploys bool) error {
				capturedVars = variables
				return nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	service = "api"

	cmd := setVariableCmd
	err := cmd.RunE(cmd, []string{"PORT=3000", "NODE_ENV=production"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(capturedVars) != 2 {
		t.Errorf("expected 2 variables, got %d", len(capturedVars))
	}
}

func TestRunDeleteVariable_MissingProject(t *testing.T) {
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
		return &api.MockClient{}
	}
	project = ""

	cmd := deleteVariableCmd
	err := cmd.RunE(cmd, []string{"PORT"})

	if err == nil {
		t.Error("expected error for missing project")
	}
}

func TestRunDeleteVariable_Success(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origService := service
	origToken := token
	origYes := deleteVariableYes
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		service = origService
		token = origToken
		deleteVariableYes = origYes
	}()

	var deletedKey string

	token = "test-token"
	deleteVariableYes = true // Skip confirmation
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{
					{ID: "svc-1", Name: "api"},
				}, nil
			},
			DeleteVariableFunc: func(projectID, environmentID, serviceID, name string) error {
				deletedKey = name
				return nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	service = "api"

	cmd := deleteVariableCmd
	err := cmd.RunE(cmd, []string{"OLD_VAR"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if deletedKey != "OLD_VAR" {
		t.Errorf("expected deleted key 'OLD_VAR', got %q", deletedKey)
	}
}

func TestMaskValue(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"ab", "**************"},
		{"abcd", "ab************"},
		{"abcde", "ab************"},
		{"mysecretpassword", "my************"},
	}

	for _, tc := range tests {
		result := api.MaskValue(tc.input)
		if result != tc.expected {
			t.Errorf("MaskValue(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

func TestIsSensitiveKey(t *testing.T) {
	tests := []struct {
		key      string
		expected bool
	}{
		{"PORT", false},
		{"NODE_ENV", false},
		{"API_KEY", true},
		{"DATABASE_PASSWORD", true},
		{"AWS_SECRET_ACCESS_KEY", true},
		{"AUTH_TOKEN", true},
		{"PRIVATE_KEY", true},
		{"MY_CREDENTIAL", true},
		{"NORMAL_VAR", false},
		{"PATH", false},   // word boundary prevents false positive
		{"AUTHOR", false}, // word boundary prevents false positive
	}

	for _, tc := range tests {
		result := api.IsSensitiveKey(tc.key)
		if result != tc.expected {
			t.Errorf("IsSensitiveKey(%q) = %v, expected %v", tc.key, result, tc.expected)
		}
	}
}

func TestServiceDetailToOutput_WithSealedVars(t *testing.T) {
	svc := types.ServiceDetail{
		ID:   "svc-1",
		Name: "api",
	}

	vars := map[string]string{
		"PORT":       "3000",
		"API_SECRET": "",
	}
	sealedMap := map[string]bool{
		"PORT":       false,
		"API_SECRET": true,
	}

	result := serviceDetailToOutput(svc, "my-project", "production", vars, sealedMap, false)

	if result.Variables["API_SECRET"] != "[SEALED]" {
		t.Errorf("expected sealed variable to show [SEALED], got %q", result.Variables["API_SECRET"])
	}
	if result.Variables["PORT"] != "3000" {
		t.Errorf("expected PORT=3000, got %q", result.Variables["PORT"])
	}
}

func TestServiceDetailToOutput_WithSensitiveVarsMasked(t *testing.T) {
	svc := types.ServiceDetail{
		ID:   "svc-1",
		Name: "api",
	}

	vars := map[string]string{
		"PORT":              "3000",
		"DATABASE_PASSWORD": "supersecret123",
	}
	sealedMap := map[string]bool{
		"PORT":              false,
		"DATABASE_PASSWORD": false,
	}

	result := serviceDetailToOutput(svc, "my-project", "production", vars, sealedMap, false)

	if result.Variables["PORT"] != "3000" {
		t.Errorf("expected PORT=3000, got %q", result.Variables["PORT"])
	}
	// Password should be masked
	if result.Variables["DATABASE_PASSWORD"] == "supersecret123" {
		t.Error("expected DATABASE_PASSWORD to be masked")
	}
}

func TestServiceDetailToOutput_ShowValues(t *testing.T) {
	svc := types.ServiceDetail{
		ID:   "svc-1",
		Name: "api",
	}

	vars := map[string]string{
		"DATABASE_PASSWORD": "supersecret123",
	}
	sealedMap := map[string]bool{
		"DATABASE_PASSWORD": false,
	}

	result := serviceDetailToOutput(svc, "my-project", "production", vars, sealedMap, true)

	// With showValues=true, password should NOT be masked
	if result.Variables["DATABASE_PASSWORD"] != "supersecret123" {
		t.Errorf("expected DATABASE_PASSWORD to be unmasked, got %q", result.Variables["DATABASE_PASSWORD"])
	}
}
