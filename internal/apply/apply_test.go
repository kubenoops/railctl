package apply

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/config"
	"github.com/kubenoops/railctl/internal/diff"
	"github.com/kubenoops/railctl/internal/types"
)

func TestApply_CreateService(t *testing.T) {
	createCalled := false
	mock := &api.MockClient{
		CreateServiceFunc: func(projectID, envID, name, image string, creds *api.RegistryCredentials) (types.Service, error) {
			createCalled = true
			return types.Service{ID: "svc-1", Name: name}, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return []types.Environment{{ID: "env-1", Name: "production"}}, nil
		},
		DeleteServiceInstanceFunc: func(serviceID, environmentID string) error {
			return nil
		},
	}

	cs := &diff.ChangeSet{
		Changes: []diff.ResourceChange{
			{
				Type:        diff.ChangeCreate,
				ServiceName: "web",
				Fields: []diff.FieldDiff{
					{Path: "image", Desired: "node:20-alpine"},
				},
			},
		},
	}

	configMap := map[string]config.ServiceConfig{
		"web": {Name: "web", Image: "node:20-alpine"},
	}

	result := Apply(mock, cs, "proj-1", "env-1", configMap, Opts{Output: io.Discard})

	if !createCalled {
		t.Error("expected CreateService to be called")
	}
	if len(result.Created) != 1 || result.Created[0] != "web" {
		t.Errorf("expected Created=[web], got %v", result.Created)
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected no errors, got %v", result.Errors)
	}
}

func TestApply_CreateServiceWithVariables(t *testing.T) {
	setVarsCalled := false
	var capturedVars map[string]string

	mock := &api.MockClient{
		CreateServiceFunc: func(projectID, envID, name, image string, creds *api.RegistryCredentials) (types.Service, error) {
			return types.Service{ID: "svc-1", Name: name}, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return []types.Environment{{ID: "env-1", Name: "production"}}, nil
		},
		SetVariablesFunc: func(projectID, envID, serviceID string, variables map[string]string, skipDeploys bool) error {
			setVarsCalled = true
			capturedVars = variables
			if !skipDeploys {
				t.Error("expected skipDeploys=true")
			}
			return nil
		},
	}

	cs := &diff.ChangeSet{
		Changes: []diff.ResourceChange{
			{
				Type:        diff.ChangeCreate,
				ServiceName: "web",
				Fields: []diff.FieldDiff{
					{Path: "image", Desired: "node:20"},
					{Path: "variables.PORT", Desired: "3000"},
				},
			},
		},
	}

	configMap := map[string]config.ServiceConfig{
		"web": {
			Name:      "web",
			Image:     "node:20",
			Variables: map[string]string{"PORT": "3000"},
		},
	}

	result := Apply(mock, cs, "proj-1", "env-1", configMap, Opts{Output: io.Discard})

	if !setVarsCalled {
		t.Error("expected SetVariables to be called")
	}
	if capturedVars["PORT"] != "3000" {
		t.Errorf("expected PORT=3000, got %q", capturedVars["PORT"])
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected no errors, got %v", result.Errors)
	}
}

func TestApply_CreateServiceWithVolume(t *testing.T) {
	createVolumeCalled := false
	var capturedMountPath string

	mock := &api.MockClient{
		CreateServiceFunc: func(projectID, envID, name, image string, creds *api.RegistryCredentials) (types.Service, error) {
			return types.Service{ID: "svc-1", Name: name}, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return []types.Environment{{ID: "env-1", Name: "production"}}, nil
		},
		CreateVolumeFunc: func(projectID, envID, serviceID, mountPath string) (api.Volume, error) {
			createVolumeCalled = true
			capturedMountPath = mountPath
			return api.Volume{ID: "vol-1", Name: "data"}, nil
		},
	}

	cs := &diff.ChangeSet{
		Changes: []diff.ResourceChange{
			{
				Type:        diff.ChangeCreate,
				ServiceName: "db",
				Fields: []diff.FieldDiff{
					{Path: "image", Desired: "postgres:16"},
					{Path: "volume.mountPath", Desired: "/var/lib/postgresql/data"},
				},
			},
		},
	}

	configMap := map[string]config.ServiceConfig{
		"db": {
			Name:   "db",
			Image:  "postgres:16",
			Volume: config.VolumeConfig{MountPath: "/var/lib/postgresql/data"},
		},
	}

	result := Apply(mock, cs, "proj-1", "env-1", configMap, Opts{Output: io.Discard})

	if !createVolumeCalled {
		t.Error("expected CreateVolume to be called")
	}
	if capturedMountPath != "/var/lib/postgresql/data" {
		t.Errorf("expected mountPath '/var/lib/postgresql/data', got %q", capturedMountPath)
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected no errors, got %v", result.Errors)
	}
}

func TestApply_UpdateServiceImage(t *testing.T) {
	updateCalled := false
	var capturedImage string

	mock := &api.MockClient{
		ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{{ID: "svc-1", Name: "web"}}, nil
		},
		UpdateServiceInstanceFunc: func(serviceID, envID, image string, creds *api.RegistryCredentials) error {
			updateCalled = true
			capturedImage = image
			return nil
		},
	}

	cs := &diff.ChangeSet{
		Changes: []diff.ResourceChange{
			{
				Type:        diff.ChangeUpdate,
				ServiceName: "web",
				Fields: []diff.FieldDiff{
					{Path: "image", Current: "node:18-alpine", Desired: "node:20-alpine"},
				},
			},
		},
	}

	configMap := map[string]config.ServiceConfig{
		"web": {Name: "web", Image: "node:20-alpine"},
	}

	result := Apply(mock, cs, "proj-1", "env-1", configMap, Opts{Output: io.Discard})

	if !updateCalled {
		t.Error("expected UpdateServiceInstance to be called")
	}
	if capturedImage != "node:20-alpine" {
		t.Errorf("expected image 'node:20-alpine', got %q", capturedImage)
	}
	if len(result.Updated) != 1 || result.Updated[0] != "web" {
		t.Errorf("expected Updated=[web], got %v", result.Updated)
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected no errors, got %v", result.Errors)
	}
}

func TestApply_UpdateServiceDeployConfig(t *testing.T) {
	configCalled := false
	var capturedReplicas *int
	var capturedRestartPolicy *string

	mock := &api.MockClient{
		ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{{ID: "svc-1", Name: "web"}}, nil
		},
		UpdateServiceInstanceConfigFunc: func(serviceID, envID string, startCmd, restartPolicy *string, maxRetries, replicas *int, hcPath *string, hcTimeout *int) error {
			configCalled = true
			capturedReplicas = replicas
			capturedRestartPolicy = restartPolicy
			return nil
		},
	}

	cs := &diff.ChangeSet{
		Changes: []diff.ResourceChange{
			{
				Type:        diff.ChangeUpdate,
				ServiceName: "web",
				Fields: []diff.FieldDiff{
					{Path: "deploy.replicas", Current: "1", Desired: "3"},
					{Path: "deploy.restartPolicy", Current: "ON_FAILURE", Desired: "ALWAYS"},
				},
			},
		},
	}

	configMap := map[string]config.ServiceConfig{
		"web": {
			Name:  "web",
			Image: "node:20",
			Deploy: config.DeployConfig{
				Replicas:      3,
				RestartPolicy: "ALWAYS",
			},
		},
	}

	result := Apply(mock, cs, "proj-1", "env-1", configMap, Opts{Output: io.Discard})

	if !configCalled {
		t.Error("expected UpdateServiceInstanceConfig to be called")
	}
	if capturedReplicas == nil || *capturedReplicas != 3 {
		t.Errorf("expected replicas=3, got %v", capturedReplicas)
	}
	if capturedRestartPolicy == nil || *capturedRestartPolicy != "ALWAYS" {
		t.Errorf("expected restartPolicy=ALWAYS, got %v", capturedRestartPolicy)
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected no errors, got %v", result.Errors)
	}
}

func TestApply_UpdateServiceVariables(t *testing.T) {
	setVarsCalled := false
	deleteVarCalled := false
	var capturedSetVars map[string]string
	var capturedDeletedVar string

	mock := &api.MockClient{
		ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{{ID: "svc-1", Name: "web"}}, nil
		},
		SetVariablesFunc: func(projectID, envID, serviceID string, variables map[string]string, skipDeploys bool) error {
			setVarsCalled = true
			capturedSetVars = variables
			return nil
		},
		DeleteVariableFunc: func(projectID, envID, serviceID, name string) error {
			deleteVarCalled = true
			capturedDeletedVar = name
			return nil
		},
	}

	cs := &diff.ChangeSet{
		Changes: []diff.ResourceChange{
			{
				Type:        diff.ChangeUpdate,
				ServiceName: "web",
				Fields: []diff.FieldDiff{
					{Path: "variables.PORT", Current: "3000", Desired: "8080"},
					{Path: "variables.NEW_VAR", Current: "", Desired: "hello"},
					{Path: "variables.OLD_VAR", Current: "goodbye", Desired: ""},
				},
			},
		},
	}

	configMap := map[string]config.ServiceConfig{
		"web": {
			Name:  "web",
			Image: "node:20",
			Variables: map[string]string{
				"PORT":    "8080",
				"NEW_VAR": "hello",
			},
		},
	}

	result := Apply(mock, cs, "proj-1", "env-1", configMap, Opts{Output: io.Discard})

	if !setVarsCalled {
		t.Error("expected SetVariables to be called")
	}
	if capturedSetVars["PORT"] != "8080" {
		t.Errorf("expected PORT=8080, got %q", capturedSetVars["PORT"])
	}
	if capturedSetVars["NEW_VAR"] != "hello" {
		t.Errorf("expected NEW_VAR=hello, got %q", capturedSetVars["NEW_VAR"])
	}
	if !deleteVarCalled {
		t.Error("expected DeleteVariable to be called")
	}
	if capturedDeletedVar != "OLD_VAR" {
		t.Errorf("expected deleted var 'OLD_VAR', got %q", capturedDeletedVar)
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected no errors, got %v", result.Errors)
	}
}

func TestApply_DeleteService(t *testing.T) {
	deleteCalled := false
	var capturedServiceID string

	mock := &api.MockClient{
		ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{{ID: "svc-1", Name: "old-service"}}, nil
		},
		DeleteServiceFunc: func(id string) error {
			deleteCalled = true
			capturedServiceID = id
			return nil
		},
	}

	cs := &diff.ChangeSet{
		Changes: []diff.ResourceChange{
			{
				Type:        diff.ChangeDelete,
				ServiceName: "old-service",
				Fields: []diff.FieldDiff{
					{Path: "image", Current: "nginx:1.25"},
				},
			},
		},
	}

	configMap := map[string]config.ServiceConfig{}

	result := Apply(mock, cs, "proj-1", "env-1", configMap, Opts{Output: io.Discard})

	if !deleteCalled {
		t.Error("expected DeleteService to be called")
	}
	if capturedServiceID != "svc-1" {
		t.Errorf("expected service ID 'svc-1', got %q", capturedServiceID)
	}
	if len(result.Deleted) != 1 || result.Deleted[0] != "old-service" {
		t.Errorf("expected Deleted=[old-service], got %v", result.Deleted)
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected no errors, got %v", result.Errors)
	}
}

func TestApply_DryRun(t *testing.T) {
	// All mock funcs panic if called — dry run should not call any API.
	mock := &api.MockClient{
		CreateServiceFunc: func(projectID, envID, name, image string, creds *api.RegistryCredentials) (types.Service, error) {
			panic("CreateService should not be called in dry run")
		},
		UpdateServiceInstanceFunc: func(serviceID, envID, image string, creds *api.RegistryCredentials) error {
			panic("UpdateServiceInstance should not be called in dry run")
		},
		DeleteServiceFunc: func(id string) error {
			panic("DeleteService should not be called in dry run")
		},
		ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
			panic("ListServices should not be called in dry run")
		},
	}

	cs := &diff.ChangeSet{
		Changes: []diff.ResourceChange{
			{Type: diff.ChangeCreate, ServiceName: "new-svc", Fields: []diff.FieldDiff{{Path: "image", Desired: "node:20"}}},
			{Type: diff.ChangeUpdate, ServiceName: "web", Fields: []diff.FieldDiff{{Path: "image", Current: "node:18", Desired: "node:20"}}},
			{Type: diff.ChangeDelete, ServiceName: "old-svc", Fields: []diff.FieldDiff{{Path: "image", Current: "nginx:1.25"}}},
		},
	}

	configMap := map[string]config.ServiceConfig{
		"new-svc": {Name: "new-svc", Image: "node:20"},
		"web":     {Name: "web", Image: "node:20"},
	}

	var buf bytes.Buffer
	result := Apply(mock, cs, "proj-1", "env-1", configMap, Opts{DryRun: true, Output: &buf})

	output := buf.String()
	if !strings.Contains(output, "Would create service 'new-svc'") {
		t.Errorf("expected 'Would create' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "Would update service 'web'") {
		t.Errorf("expected 'Would update' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "Would delete service 'old-svc'") {
		t.Errorf("expected 'Would delete' in output, got:\n%s", output)
	}

	// No results should be populated in dry run.
	if len(result.Created) != 0 || len(result.Updated) != 0 || len(result.Deleted) != 0 {
		t.Errorf("expected empty result in dry run, got created=%v updated=%v deleted=%v",
			result.Created, result.Updated, result.Deleted)
	}
}

func TestApply_ErrorCollection(t *testing.T) {
	createCallCount := 0

	mock := &api.MockClient{
		CreateServiceFunc: func(projectID, envID, name, image string, creds *api.RegistryCredentials) (types.Service, error) {
			createCallCount++
			if name == "failing-svc" {
				return types.Service{}, errors.New("API error: create failed")
			}
			return types.Service{ID: "svc-2", Name: name}, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return []types.Environment{{ID: "env-1", Name: "production"}}, nil
		},
	}

	cs := &diff.ChangeSet{
		Changes: []diff.ResourceChange{
			{Type: diff.ChangeCreate, ServiceName: "failing-svc", Fields: []diff.FieldDiff{{Path: "image", Desired: "bad:image"}}},
			{Type: diff.ChangeCreate, ServiceName: "good-svc", Fields: []diff.FieldDiff{{Path: "image", Desired: "node:20"}}},
		},
	}

	configMap := map[string]config.ServiceConfig{
		"failing-svc": {Name: "failing-svc", Image: "bad:image"},
		"good-svc":    {Name: "good-svc", Image: "node:20"},
	}

	result := Apply(mock, cs, "proj-1", "env-1", configMap, Opts{Output: io.Discard})

	if createCallCount != 2 {
		t.Errorf("expected CreateService to be called 2 times, got %d", createCallCount)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(result.Errors), result.Errors)
	}
	if !strings.Contains(result.Errors[0].Error(), "failing-svc") {
		t.Errorf("expected error to mention 'failing-svc', got: %v", result.Errors[0])
	}
	if len(result.Created) != 1 || result.Created[0] != "good-svc" {
		t.Errorf("expected Created=[good-svc], got %v", result.Created)
	}
}

func TestApply_Mixed(t *testing.T) {
	mock := &api.MockClient{
		CreateServiceFunc: func(projectID, envID, name, image string, creds *api.RegistryCredentials) (types.Service, error) {
			return types.Service{ID: "svc-new", Name: name}, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return []types.Environment{{ID: "env-1", Name: "production"}}, nil
		},
		ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{
				{ID: "svc-web", Name: "web"},
				{ID: "svc-old", Name: "old-service"},
			}, nil
		},
		UpdateServiceInstanceFunc: func(serviceID, envID, image string, creds *api.RegistryCredentials) error {
			return nil
		},
		DeleteServiceFunc: func(id string) error {
			return nil
		},
	}

	cs := &diff.ChangeSet{
		Changes: []diff.ResourceChange{
			{Type: diff.ChangeCreate, ServiceName: "new-service", Fields: []diff.FieldDiff{{Path: "image", Desired: "redis:7"}}},
			{Type: diff.ChangeUpdate, ServiceName: "web", Fields: []diff.FieldDiff{{Path: "image", Current: "node:18", Desired: "node:20"}}},
			{Type: diff.ChangeDelete, ServiceName: "old-service", Fields: []diff.FieldDiff{{Path: "image", Current: "nginx:1.25"}}},
		},
	}

	configMap := map[string]config.ServiceConfig{
		"new-service": {Name: "new-service", Image: "redis:7"},
		"web":         {Name: "web", Image: "node:20"},
	}

	result := Apply(mock, cs, "proj-1", "env-1", configMap, Opts{Output: io.Discard})

	if len(result.Created) != 1 || result.Created[0] != "new-service" {
		t.Errorf("expected Created=[new-service], got %v", result.Created)
	}
	if len(result.Updated) != 1 || result.Updated[0] != "web" {
		t.Errorf("expected Updated=[web], got %v", result.Updated)
	}
	if len(result.Deleted) != 1 || result.Deleted[0] != "old-service" {
		t.Errorf("expected Deleted=[old-service], got %v", result.Deleted)
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected no errors, got %v", result.Errors)
	}
}

func TestApply_UpdateSendsRegistryCredsWithImage(t *testing.T) {
	var capturedImage string
	var capturedCreds *api.RegistryCredentials

	mock := &api.MockClient{
		ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{{ID: "svc-1", Name: "api"}}, nil
		},
		UpdateServiceInstanceFunc: func(serviceID, envID, image string, creds *api.RegistryCredentials) error {
			capturedImage = image
			capturedCreds = creds
			return nil
		},
	}

	cs := &diff.ChangeSet{
		Changes: []diff.ResourceChange{
			{
				Type:        diff.ChangeUpdate,
				ServiceName: "api",
				Fields:      []diff.FieldDiff{{Path: "image", Current: "ghcr.io/acme/api:v1", Desired: "ghcr.io/acme/api:v2"}},
			},
		},
	}
	configMap := map[string]config.ServiceConfig{
		"api": {
			Name:     "api",
			Image:    "ghcr.io/acme/api:v2",
			Registry: config.RegistryConfig{Username: "acme-bot", Password: "ghp_token"},
		},
	}

	result := Apply(mock, cs, "proj-1", "env-1", configMap, Opts{Output: io.Discard})

	if capturedImage != "ghcr.io/acme/api:v2" {
		t.Errorf("expected image update, got %q", capturedImage)
	}
	if capturedCreds == nil || capturedCreds.Username != "acme-bot" || capturedCreds.Password != "ghp_token" {
		t.Errorf("expected registry creds sent alongside the image, got %+v", capturedCreds)
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected no errors, got %v", result.Errors)
	}
}

// A var-only update to a private-registry service re-asserts the creds (Railway
// can't be diffed, so declared creds are refreshed on any update). Image is
// empty because only the credentials are being sent.
func TestApply_UpdateVarOnlyReassertsRegistryCreds(t *testing.T) {
	var capturedImage string
	var capturedCreds *api.RegistryCredentials

	mock := &api.MockClient{
		ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{{ID: "svc-1", Name: "api"}}, nil
		},
		UpdateServiceInstanceFunc: func(serviceID, envID, image string, creds *api.RegistryCredentials) error {
			capturedImage = image
			capturedCreds = creds
			return nil
		},
		SetVariablesFunc: func(projectID, envID, serviceID string, variables map[string]string, skipDeploys bool) error {
			return nil
		},
	}

	cs := &diff.ChangeSet{
		Changes: []diff.ResourceChange{
			{
				Type:        diff.ChangeUpdate,
				ServiceName: "api",
				Fields:      []diff.FieldDiff{{Path: "variables.FOO", Desired: "bar"}},
			},
		},
	}
	configMap := map[string]config.ServiceConfig{
		"api": {
			Name:     "api",
			Image:    "ghcr.io/acme/api:v1",
			Registry: config.RegistryConfig{Username: "acme-bot", Password: "ghp_token"},
		},
	}

	result := Apply(mock, cs, "proj-1", "env-1", configMap, Opts{Output: io.Discard})

	if capturedCreds == nil || capturedCreds.Username != "acme-bot" || capturedCreds.Password != "ghp_token" {
		t.Errorf("expected creds re-asserted on a var-only update, got %+v", capturedCreds)
	}
	if capturedImage != "" {
		t.Errorf("image should be empty when only creds are re-asserted, got %q", capturedImage)
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected no errors, got %v", result.Errors)
	}
}

// A var-only update to a service WITHOUT a registry block must not call
// UpdateServiceInstance (nothing to send, avoids a needless redeploy).
func TestApply_UpdateVarOnlyNoRegistrySkipsServiceInstance(t *testing.T) {
	var updateCalled bool

	mock := &api.MockClient{
		ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{{ID: "svc-1", Name: "pg"}}, nil
		},
		UpdateServiceInstanceFunc: func(serviceID, envID, image string, creds *api.RegistryCredentials) error {
			updateCalled = true
			return nil
		},
		SetVariablesFunc: func(projectID, envID, serviceID string, variables map[string]string, skipDeploys bool) error {
			return nil
		},
	}

	cs := &diff.ChangeSet{
		Changes: []diff.ResourceChange{
			{
				Type:        diff.ChangeUpdate,
				ServiceName: "pg",
				Fields:      []diff.FieldDiff{{Path: "variables.FOO", Desired: "bar"}},
			},
		},
	}
	configMap := map[string]config.ServiceConfig{
		"pg": {Name: "pg", Image: "postgres:16"},
	}

	result := Apply(mock, cs, "proj-1", "env-1", configMap, Opts{Output: io.Discard})

	if updateCalled {
		t.Error("UpdateServiceInstance must not be called for a var-only update with no registry block")
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected no errors, got %v", result.Errors)
	}
}

// A changed sensitive variable must be written to Railway with its real value,
// not the masked value the diff displays.
func TestApply_UpdateWritesRealSensitiveValueNotMask(t *testing.T) {
	var captured map[string]string
	mock := &api.MockClient{
		ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{{ID: "svc-1", Name: "api"}}, nil
		},
		SetVariablesFunc: func(projectID, envID, serviceID string, variables map[string]string, skipDeploys bool) error {
			captured = variables
			return nil
		},
	}

	// The diff field carries a MASKED desired value, as diff.Compute produces
	// for a sensitive key; the real value lives in the config.
	cs := &diff.ChangeSet{
		Changes: []diff.ResourceChange{
			{
				Type:        diff.ChangeUpdate,
				ServiceName: "api",
				Fields:      []diff.FieldDiff{{Path: "variables.API_KEY", Current: "ol************", Desired: "ne************"}},
			},
		},
	}
	configMap := map[string]config.ServiceConfig{
		"api": {Name: "api", Image: "ghcr.io/acme/api:v1", Variables: map[string]string{"API_KEY": "real-secret-value"}},
	}

	Apply(mock, cs, "proj-1", "env-1", configMap, Opts{Output: io.Discard})

	if captured["API_KEY"] != "real-secret-value" {
		t.Errorf("expected real value written to Railway, got %q (masked value leaked into the write)", captured["API_KEY"])
	}
}

func TestApply_UpdateTriggersDeployOnImageChange(t *testing.T) {
	deployCalled := false
	mock := &api.MockClient{
		ListServicesFunc: func(p, e string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{{ID: "svc-1", Name: "web"}}, nil
		},
		UpdateServiceInstanceFunc: func(s, e, i string, c *api.RegistryCredentials) error { return nil },
		DeployServiceInstanceFunc: func(s, e string) (string, error) { deployCalled = true; return "dep-1", nil },
	}
	cs := &diff.ChangeSet{Changes: []diff.ResourceChange{{
		Type: diff.ChangeUpdate, ServiceName: "web",
		Fields: []diff.FieldDiff{{Path: "image", Current: "node:18", Desired: "node:20"}},
	}}}
	cfg := map[string]config.ServiceConfig{"web": {Name: "web", Image: "node:20"}}
	Apply(mock, cs, "p", "e", cfg, Opts{Output: io.Discard})
	if !deployCalled {
		t.Error("expected a deployment to be triggered on an image change")
	}
}

func TestApply_UpdateTriggersDeployOnVariableChange(t *testing.T) {
	deployCalled := false
	mock := &api.MockClient{
		ListServicesFunc: func(p, e string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{{ID: "svc-1", Name: "web"}}, nil
		},
		SetVariablesFunc:          func(p, e, s string, v map[string]string, skip bool) error { return nil },
		DeployServiceInstanceFunc: func(s, e string) (string, error) { deployCalled = true; return "dep-1", nil },
	}
	cs := &diff.ChangeSet{Changes: []diff.ResourceChange{{
		Type: diff.ChangeUpdate, ServiceName: "web",
		Fields: []diff.FieldDiff{{Path: "variables.FOO", Desired: "bar"}},
	}}}
	cfg := map[string]config.ServiceConfig{"web": {Name: "web", Image: "node:20", Variables: map[string]string{"FOO": "bar"}}}
	Apply(mock, cs, "p", "e", cfg, Opts{Output: io.Discard})
	if !deployCalled {
		t.Error("expected a deployment to be triggered on a variable change (staged, skipDeploys=true)")
	}
}

func TestApply_UpdateNoDeployForNetworkingOnly(t *testing.T) {
	deployCalled := false
	port := 8080
	mock := &api.MockClient{
		ListServicesFunc: func(p, e string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{{ID: "svc-1", Name: "web"}}, nil
		},
		ListDomainsFunc: func(p, e, s string) (api.DomainList, error) {
			return api.DomainList{ServiceDomains: []api.ServiceDomain{{ID: "dom-1", TargetPort: &port}}}, nil
		},
		UpdateServiceDomainPortFunc: func(domainID string, targetPort int) error { return nil },
		DeployServiceInstanceFunc:   func(s, e string) (string, error) { deployCalled = true; return "dep-1", nil },
	}
	cs := &diff.ChangeSet{Changes: []diff.ResourceChange{{
		Type: diff.ChangeUpdate, ServiceName: "web",
		Fields: []diff.FieldDiff{{Path: "networking.domain.port", Current: "3000", Desired: "8080"}},
	}}}
	cfg := map[string]config.ServiceConfig{"web": {Name: "web", Image: "node:20", Networking: config.NetworkingConfig{Domain: config.DomainConfig{Port: 8080}}}}
	Apply(mock, cs, "p", "e", cfg, Opts{Output: io.Discard})
	if deployCalled {
		t.Error("networking-only change must not trigger a deploy (applies immediately)")
	}
}

func TestApply_UpdateNetworkingWithRegistryDoesNotStageOrDeploy(t *testing.T) {
	// Networking-only drift on a service with a registry block must not stage creds
	// (nothing would deploy them) nor trigger a deploy.
	credsStaged := false
	deployCalled := false
	port := 8080
	mock := &api.MockClient{
		ListServicesFunc: func(p, e string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{{ID: "svc-1", Name: "web"}}, nil
		},
		ListDomainsFunc: func(p, e, s string) (api.DomainList, error) {
			return api.DomainList{ServiceDomains: []api.ServiceDomain{{ID: "dom-1", TargetPort: &port}}}, nil
		},
		UpdateServiceDomainPortFunc: func(domainID string, targetPort int) error { return nil },
		UpdateServiceInstanceFunc:   func(s, e, i string, c *api.RegistryCredentials) error { credsStaged = true; return nil },
		DeployServiceInstanceFunc:   func(s, e string) (string, error) { deployCalled = true; return "dep-1", nil },
	}
	cs := &diff.ChangeSet{Changes: []diff.ResourceChange{{
		Type: diff.ChangeUpdate, ServiceName: "web",
		Fields: []diff.FieldDiff{
			{Path: "networking.domain.port", Current: "3000", Desired: "8080"},
			{Path: "registry.username", Desired: "user"},
		},
	}}}
	cfg := map[string]config.ServiceConfig{"web": {
		Name: "web", Image: "node:20",
		Registry:   config.RegistryConfig{Username: "user", Password: "pass"},
		Networking: config.NetworkingConfig{Domain: config.DomainConfig{Port: 8080}},
	}}
	Apply(mock, cs, "p", "e", cfg, Opts{Output: io.Discard})
	if credsStaged {
		t.Error("must not stage credentials on a networking-only change (they would strand as a pending change)")
	}
	if deployCalled {
		t.Error("networking-only change must not trigger a deploy even with a registry block")
	}
}

func TestApply_UpdateNoDeployForVolumeOnly(t *testing.T) {
	// The volume branch only warns and stages nothing, so a volume-only diff must
	// not redeploy (it would never converge and churn forever).
	deployCalled := false
	mock := &api.MockClient{
		ListServicesFunc: func(p, e string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{{ID: "svc-1", Name: "db"}}, nil
		},
		DeployServiceInstanceFunc: func(s, e string) (string, error) { deployCalled = true; return "dep-1", nil },
	}
	cs := &diff.ChangeSet{Changes: []diff.ResourceChange{{
		Type: diff.ChangeUpdate, ServiceName: "db",
		Fields: []diff.FieldDiff{{Path: "volume.mountPath", Current: "/old", Desired: "/data"}},
	}}}
	cfg := map[string]config.ServiceConfig{"db": {Name: "db", Image: "postgres:16"}}
	Apply(mock, cs, "p", "e", cfg, Opts{Output: io.Discard})
	if deployCalled {
		t.Error("volume-only change must not trigger a deploy (nothing is staged)")
	}
}
