package apply

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"slices"
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

// TestApply_CreateServiceTriggersDeployment pins the rule that a created
// service must end up WITH exactly one deployment, born after all config. The
// service is created source-less (serviceCreate's implicit rollout is
// unreliable and races staged config — a multi-service apply routinely left
// services with zero deployments, a systemic failure --await then hid), and
// the source attach + rollout travel as ONE request: two triggers meant two
// Railway deployments, the loser REMOVED (observed live 2026-09-03/04).
func TestApply_CreateServiceTriggersDeployment(t *testing.T) {
	var deployedService, deployedEnv, deployedImage string
	rolloutCalls, bareDeploys := 0, 0
	mock := &api.MockClient{
		CreateServiceFunc: func(projectID, envID, name, image string, creds *api.RegistryCredentials) (types.Service, error) {
			if image != "" {
				t.Errorf("create must be source-less, got image %q", image)
			}
			return types.Service{ID: "svc-1", Name: name}, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return []types.Environment{{ID: "env-1", Name: "production"}}, nil
		},
		AttachSourceAndDeployFunc: func(serviceID, environmentID, image string, _ *api.RegistryCredentials) (string, error) {
			rolloutCalls++
			deployedService, deployedEnv, deployedImage = serviceID, environmentID, image
			return "deploy-1", nil
		},
		DeployServiceInstanceFunc: func(serviceID, environmentID string) (string, error) {
			bareDeploys++
			return "deploy-x", nil
		},
	}

	cs := &diff.ChangeSet{
		Changes: []diff.ResourceChange{
			{
				Type:        diff.ChangeCreate,
				ServiceName: "web",
				Fields:      []diff.FieldDiff{{Path: "image", Desired: "node:20-alpine"}},
			},
		},
	}
	configMap := map[string]config.ServiceConfig{
		"web": {Name: "web", Image: "node:20-alpine"},
	}

	result := Apply(mock, cs, "proj-1", "env-1", configMap, Opts{Output: io.Discard})

	if len(result.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	if rolloutCalls != 1 {
		t.Fatalf("expected exactly 1 attach+rollout for the created service, got %d", rolloutCalls)
	}
	if bareDeploys != 0 {
		t.Errorf("a service with an image must not take the bare deploy path, got %d calls", bareDeploys)
	}
	if deployedService != "svc-1" || deployedEnv != "env-1" || deployedImage != "node:20-alpine" {
		t.Errorf("rollout targeted %s/%s image %q, want svc-1/env-1 node:20-alpine", deployedService, deployedEnv, deployedImage)
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
		CreateVolumeFunc: func(projectID, envID, serviceID, mountPath string, _ string) (api.Volume, error) {
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
		DeleteServiceFunc: func(environmentID, id string) error {
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
		DeleteServiceFunc: func(environmentID, id string) error {
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
		DeleteServiceFunc: func(environmentID, id string) error {
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
		UpdateServiceDomainPortFunc: func(domainID, domain, environmentID, serviceID string, targetPort int) error { return nil },
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

func TestApply_UpdateDomainPortUsesCustomDomainMutation(t *testing.T) {
	// When only a custom domain exists, the port must go through UpdateCustomDomainPort,
	// not the service-domain mutation.
	var customPort int
	serviceDomainCalled := false
	mock := &api.MockClient{
		ListServicesFunc: func(p, e string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{{ID: "svc-1", Name: "web"}}, nil
		},
		ListDomainsFunc: func(p, e, s string) (api.DomainList, error) {
			return api.DomainList{CustomDomains: []api.CustomDomain{{ID: "cdom-1", Domain: "app.example.com"}}}, nil
		},
		UpdateCustomDomainPortFunc:  func(id, envID string, port int) error { customPort = port; return nil },
		UpdateServiceDomainPortFunc: func(id, domain, envID, svcID string, port int) error { serviceDomainCalled = true; return nil },
	}
	cs := &diff.ChangeSet{Changes: []diff.ResourceChange{{
		Type: diff.ChangeUpdate, ServiceName: "web",
		Fields: []diff.FieldDiff{{Path: "networking.domain.port", Current: "3000", Desired: "8080"}},
	}}}
	cfg := map[string]config.ServiceConfig{"web": {Name: "web", Image: "node:20", Networking: config.NetworkingConfig{Domain: config.DomainConfig{Port: 8080}}}}
	result := Apply(mock, cs, "p", "e", cfg, Opts{Output: io.Discard})
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected apply errors: %v", result.Errors)
	}
	if serviceDomainCalled {
		t.Error("must not call UpdateServiceDomainPort for a custom domain")
	}
	if customPort != 8080 {
		t.Errorf("expected UpdateCustomDomainPort with port 8080, got %d", customPort)
	}
}

func TestApply_UpdateCreatesCustomDomainAndPrintsDNS(t *testing.T) {
	// A declared custom domain absent from live is created, and its DNS record
	// is printed for the user to configure.
	var createdName string
	var createdPort int
	mock := &api.MockClient{
		ListServicesFunc: func(p, e string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{{ID: "svc-1", Name: "web"}}, nil
		},
		ListDomainsFunc: func(p, e, s string) (api.DomainList, error) { return api.DomainList{}, nil },
		CreateCustomDomainFunc: func(proj, env, svc, domain string, port int) (api.CustomDomain, error) {
			createdName, createdPort = domain, port
			return api.CustomDomain{ID: "cd-1", Domain: domain, Status: &api.CustomDomainStatus{
				Verified:            false,
				VerificationDNSHost: "_railway-verify.app",
				VerificationToken:   "railway-verify=token123",
				// Railway puts only the routing CNAME here; the verification TXT is separate.
				DNSRecords: []api.DNSRecord{
					{RecordType: "DNS_RECORD_TYPE_CNAME", Purpose: "DNS_RECORD_PURPOSE_TRAFFIC_ROUTE", Hostlabel: "app", RequiredValue: "abc.up.railway.app"},
				},
			}}, nil
		},
	}
	cs := &diff.ChangeSet{Changes: []diff.ResourceChange{{
		Type: diff.ChangeUpdate, ServiceName: "web",
		Fields: []diff.FieldDiff{{Path: "customDomain.app.example.com", Desired: "app.example.com"}},
	}}}
	cfg := map[string]config.ServiceConfig{"web": {Name: "web", Image: "node:20", Networking: config.NetworkingConfig{
		Domain:        config.DomainConfig{Port: 8080},
		CustomDomains: []config.CustomDomainConfig{{Name: "app.example.com"}},
	}}}
	var out bytes.Buffer
	result := Apply(mock, cs, "p", "e", cfg, Opts{Output: &out})
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected apply errors: %v", result.Errors)
	}
	if createdName != "app.example.com" {
		t.Errorf("expected custom domain app.example.com created, got %q", createdName)
	}
	if createdPort != 8080 {
		t.Errorf("expected port to default to the service domain port 8080, got %d", createdPort)
	}
	for _, want := range []string{"CNAME", "abc.up.railway.app", "(traffic_route)", "TXT", "_railway-verify.app", "railway-verify=token123", "(verification)"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "DNS_RECORD_TYPE_") || strings.Contains(out.String(), "DNS_RECORD_PURPOSE_") {
		t.Errorf("raw enum prefixes must be stripped, got:\n%s", out.String())
	}
}

func TestApply_UpdateSkipsExistingCustomDomain(t *testing.T) {
	// A custom domain already present must not be recreated.
	createCalled := false
	mock := &api.MockClient{
		ListServicesFunc: func(p, e string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{{ID: "svc-1", Name: "web"}}, nil
		},
		ListDomainsFunc: func(p, e, s string) (api.DomainList, error) {
			return api.DomainList{CustomDomains: []api.CustomDomain{{ID: "cd-1", Domain: "app.example.com"}}}, nil
		},
		CreateCustomDomainFunc: func(proj, env, svc, domain string, port int) (api.CustomDomain, error) {
			createCalled = true
			return api.CustomDomain{}, nil
		},
	}
	cs := &diff.ChangeSet{Changes: []diff.ResourceChange{{
		Type: diff.ChangeUpdate, ServiceName: "web",
		Fields: []diff.FieldDiff{{Path: "image", Current: "node:18", Desired: "node:20"}},
	}}}
	cfg := map[string]config.ServiceConfig{"web": {Name: "web", Image: "node:20", Networking: config.NetworkingConfig{
		CustomDomains: []config.CustomDomainConfig{{Name: "app.example.com"}},
	}}}
	Apply(mock, cs, "p", "e", cfg, Opts{Output: io.Discard})
	if createCalled {
		t.Error("must not recreate an existing custom domain")
	}
}

func TestApply_UpdateExistingCustomDomainPortWhenDrifted(t *testing.T) {
	// A declared custom domain that already exists but whose live port differs
	// must be updated in place, not recreated.
	livePort := 3000
	var updatedPort int
	createCalled := false
	mock := &api.MockClient{
		ListServicesFunc: func(p, e string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{{ID: "svc-1", Name: "web"}}, nil
		},
		ListDomainsFunc: func(p, e, s string) (api.DomainList, error) {
			return api.DomainList{CustomDomains: []api.CustomDomain{{ID: "cd-1", Domain: "app.example.com", TargetPort: &livePort}}}, nil
		},
		CreateCustomDomainFunc: func(proj, env, svc, domain string, port int) (api.CustomDomain, error) {
			createCalled = true
			return api.CustomDomain{}, nil
		},
		UpdateCustomDomainPortFunc: func(id, envID string, port int) error { updatedPort = port; return nil },
	}
	cs := &diff.ChangeSet{Changes: []diff.ResourceChange{{
		Type: diff.ChangeUpdate, ServiceName: "web",
		Fields: []diff.FieldDiff{{Path: "customDomain.app.example.com.port", Current: "3000", Desired: "9000"}},
	}}}
	cfg := map[string]config.ServiceConfig{"web": {Name: "web", Image: "node:20", Networking: config.NetworkingConfig{
		CustomDomains: []config.CustomDomainConfig{{Name: "app.example.com", Port: 9000}},
	}}}
	result := Apply(mock, cs, "p", "e", cfg, Opts{Output: io.Discard})
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected apply errors: %v", result.Errors)
	}
	if createCalled {
		t.Error("must not recreate an existing custom domain")
	}
	if updatedPort != 9000 {
		t.Errorf("expected UpdateCustomDomainPort with port 9000, got %d", updatedPort)
	}
}

func TestApply_UpdateDomainPortPrefersServiceDomainWhenBothExist(t *testing.T) {
	// networking.domain.port governs the Railway service domain, so when both a
	// service domain and an undeclared custom domain exist, the service domain wins.
	var servicePort int
	customDomainCalled := false
	mock := &api.MockClient{
		ListServicesFunc: func(p, e string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{{ID: "svc-1", Name: "web"}}, nil
		},
		ListDomainsFunc: func(p, e, s string) (api.DomainList, error) {
			return api.DomainList{
				ServiceDomains: []api.ServiceDomain{{ID: "dom-1", Domain: "web.up.railway.app"}},
				CustomDomains:  []api.CustomDomain{{ID: "cdom-1", Domain: "app.example.com"}},
			}, nil
		},
		UpdateCustomDomainPortFunc:  func(id, envID string, port int) error { customDomainCalled = true; return nil },
		UpdateServiceDomainPortFunc: func(id, domain, envID, svcID string, port int) error { servicePort = port; return nil },
	}
	cs := &diff.ChangeSet{Changes: []diff.ResourceChange{{
		Type: diff.ChangeUpdate, ServiceName: "web",
		Fields: []diff.FieldDiff{{Path: "networking.domain.port", Current: "3000", Desired: "8080"}},
	}}}
	cfg := map[string]config.ServiceConfig{"web": {Name: "web", Image: "node:20", Networking: config.NetworkingConfig{Domain: config.DomainConfig{Port: 8080}}}}
	result := Apply(mock, cs, "p", "e", cfg, Opts{Output: io.Discard})
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected apply errors: %v", result.Errors)
	}
	if customDomainCalled {
		t.Error("must not call UpdateCustomDomainPort for an undeclared custom domain when a service domain exists")
	}
	if servicePort != 8080 {
		t.Errorf("expected UpdateServiceDomainPort with port 8080, got %d", servicePort)
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
		UpdateServiceDomainPortFunc: func(domainID, domain, environmentID, serviceID string, targetPort int) error { return nil },
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

func TestApply_UpdateRefusesMountPathMove(t *testing.T) {
	// Moving a live volume fails the update before anything is written, even
	// with other changes in the same diff.
	var writes []string
	mock := &api.MockClient{
		ListServicesFunc: func(p, e string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{{ID: "svc-1", Name: "db"}}, nil
		},
		UpdateServiceInstanceFunc: func(s, e, image string, creds *api.RegistryCredentials) error {
			writes = append(writes, "image")
			return nil
		},
		CreateVolumeFunc: func(p, e, s, m, r string) (api.Volume, error) {
			writes = append(writes, "volume")
			return api.Volume{ID: "vol-2"}, nil
		},
		DeployServiceInstanceFunc: func(s, e string) (string, error) {
			writes = append(writes, "deploy")
			return "dep-1", nil
		},
	}
	cs := &diff.ChangeSet{Changes: []diff.ResourceChange{{
		Type: diff.ChangeUpdate, ServiceName: "db",
		Fields: []diff.FieldDiff{
			{Path: "image", Current: "postgres:15", Desired: "postgres:16"},
			{Path: "volume.mountPath", Current: "/old", Desired: "/data"},
		},
	}}}
	cfg := map[string]config.ServiceConfig{"db": {Name: "db", Image: "postgres:16", Volume: config.VolumeConfig{MountPath: "/data"}}}
	result := Apply(mock, cs, "p", "e", cfg, Opts{Output: io.Discard})
	if len(result.Errors) != 1 || !strings.Contains(result.Errors[0].Error(), "mounted at /old") {
		t.Fatalf("expected a mount path error, got %v", result.Errors)
	}
	if len(writes) != 0 {
		t.Errorf("a refused update must write nothing, got %v", writes)
	}
}

// updatedServiceMock serves one live service "web" in region europe-west4 and
// the given volumes; tests add the write funcs they assert on.
func updatedServiceMock(volumes []api.VolumeInstance) *api.MockClient {
	return &api.MockClient{
		ListServicesFunc: func(p, e string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{{ID: "svc-1", Name: "web"}}, nil
		},
		ListVolumesFunc: func(p, e string) ([]api.VolumeInstance, error) { return volumes, nil },
	}
}

// deploymentsSeq answers ListDeployments with each list in turn, repeating the
// last one, and records every call in events.
func deploymentsSeq(events *[]string, lists ...[]api.Deployment) func(string, string, string, int) ([]api.Deployment, error) {
	i := 0
	return func(string, string, string, int) ([]api.Deployment, error) {
		*events = append(*events, "list")
		l := lists[min(i, len(lists)-1)]
		i++
		return l, nil
	}
}

var (
	oldDeployment   = []api.Deployment{{ID: "dep-old", Status: "SUCCESS"}}
	railwayRedeploy = []api.Deployment{{ID: "dep-railway", Status: "SUCCESS"}}
)

func TestApply_UpdateCreatesMissingVolume(t *testing.T) {
	// Only the volume changed: apply follows the redeploy Railway starts for
	// the attach instead of restarting the service a second time.
	var events, created []string
	mock := updatedServiceMock(nil)
	mock.ListDeploymentsFunc = deploymentsSeq(&events, oldDeployment, railwayRedeploy)
	mock.CreateVolumeFunc = func(p, e, s, mountPath, region string) (api.Volume, error) {
		created = append(created, s, mountPath, region)
		return api.Volume{ID: "vol-1"}, nil
	}
	mock.DeployServiceInstanceFunc = func(s, e string) (string, error) {
		events = append(events, "deploy")
		return "dep-1", nil
	}

	cs := &diff.ChangeSet{Changes: []diff.ResourceChange{{
		Type: diff.ChangeUpdate, ServiceName: "web",
		Fields: []diff.FieldDiff{{Path: "volume.mountPath", Desired: "/data"}},
	}}}
	cfg := map[string]config.ServiceConfig{"web": {Name: "web", Volume: config.VolumeConfig{MountPath: "/data"}}}
	var out bytes.Buffer
	result := Apply(mock, cs, "p", "e", cfg, Opts{Output: &out})
	if len(result.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	// No region declared: Railway places the volume beside the service.
	if strings.Join(created, "|") != "svc-1|/data|" {
		t.Errorf("CreateVolume(service, mountPath, region) = %v", created)
	}
	if slices.Contains(events, "deploy") || result.DeploymentIDs["web"] != "dep-railway" {
		t.Errorf("events = %v, DeploymentIDs = %v; want Railway's redeploy followed, no rollout of ours", events, result.DeploymentIDs)
	}
	if !strings.Contains(out.String(), "volume created at /data") {
		t.Errorf("output should report the new volume:\n%s", out.String())
	}
}

func TestApply_UpdateVolumeRedeployCancelled(t *testing.T) {
	// Railway can cancel its own volume redeploy without a replacement; apply
	// then rolls out itself so the service isn't left with nothing running.
	var events []string
	mock := updatedServiceMock(nil)
	mock.ListDeploymentsFunc = deploymentsSeq(&events, oldDeployment, []api.Deployment{{ID: "dep-railway", Status: "REMOVED"}})
	mock.CreateVolumeFunc = func(p, e, s, m, r string) (api.Volume, error) { return api.Volume{ID: "vol-1"}, nil }
	mock.DeployServiceInstanceFunc = func(s, e string) (string, error) { return "dep-1", nil }

	cs := &diff.ChangeSet{Changes: []diff.ResourceChange{{
		Type: diff.ChangeUpdate, ServiceName: "web",
		Fields: []diff.FieldDiff{{Path: "volume.mountPath", Desired: "/data"}},
	}}}
	cfg := map[string]config.ServiceConfig{"web": {Name: "web", Volume: config.VolumeConfig{MountPath: "/data"}}}
	result := Apply(mock, cs, "p", "e", cfg, Opts{Output: io.Discard})
	if len(result.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	if result.DeploymentIDs["web"] != "dep-1" {
		t.Errorf("DeploymentIDs = %v, want apply's own rollout dep-1", result.DeploymentIDs)
	}
}

func TestApply_UpdateCreatesVolumeInDeclaredRegion(t *testing.T) {
	var gotRegion string
	mock := updatedServiceMock(nil)
	mock.CreateVolumeFunc = func(p, e, s, m, region string) (api.Volume, error) {
		gotRegion = region
		return api.Volume{ID: "vol-1"}, nil
	}
	cs := &diff.ChangeSet{Changes: []diff.ResourceChange{{
		Type: diff.ChangeUpdate, ServiceName: "web",
		Fields: []diff.FieldDiff{{Path: "volume.mountPath", Desired: "/data"}},
	}}}
	cfg := map[string]config.ServiceConfig{"web": {
		Name:   "web",
		Deploy: config.DeployConfig{Region: "europe-west4"},
		Volume: config.VolumeConfig{MountPath: "/data"},
	}}
	result := Apply(mock, cs, "p", "e", cfg, Opts{Output: io.Discard})
	if len(result.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	want, _ := api.ResolveRegionID("europe-west4")
	if gotRegion != want {
		t.Errorf("volume region = %q, want %q", gotRegion, want)
	}
}

func TestApply_UpdateCreatesVolumeWithSizeAndSchedules(t *testing.T) {
	svcID := "svc-1"
	var events []string
	var resized []any
	var scheduledOn string
	mock := updatedServiceMock([]api.VolumeInstance{
		{ID: "vi-1", Volume: api.Volume{ID: "vol-1"}, ServiceID: &svcID, MountPath: "/data", SizeMB: 50000},
	})
	mock.ListDeploymentsFunc = deploymentsSeq(&events,
		oldDeployment,
		[]api.Deployment{{ID: "dep-attach", Status: "SUCCESS"}},
		[]api.Deployment{{ID: "dep-resize", Status: "SUCCESS"}},
	)
	mock.CreateVolumeFunc = func(p, e, s, m, r string) (api.Volume, error) {
		events = append(events, "create")
		return api.Volume{ID: "vol-1"}, nil
	}
	mock.ResizeVolumeFunc = func(e, volumeID string, sizeMB int) (string, error) {
		events = append(events, "resize")
		resized = append(resized, volumeID, sizeMB)
		return "patch-1", nil
	}
	mock.SetVolumeBackupSchedulesFunc = func(instanceID string, kinds []string) error {
		scheduledOn = instanceID
		return nil
	}
	cs := &diff.ChangeSet{Changes: []diff.ResourceChange{{
		Type: diff.ChangeUpdate, ServiceName: "web",
		Fields: []diff.FieldDiff{
			{Path: "volume.mountPath", Desired: "/data"},
			{Path: "volume.backupSchedules", Desired: "DAILY"},
			{Path: "volume.sizeMB", Desired: "250000"},
		},
	}}}
	cfg := map[string]config.ServiceConfig{"web": {
		Name:   "web",
		Volume: config.VolumeConfig{MountPath: "/data", BackupSchedules: []string{"DAILY"}, SizeMB: 250000},
	}}
	result := Apply(mock, cs, "p", "e", cfg, Opts{Output: io.Discard})
	if len(result.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	if len(resized) != 2 || resized[0] != "vol-1" || resized[1] != 250000 {
		t.Errorf("ResizeVolume(volume, size) = %v, want [vol-1 250000]", resized)
	}
	if scheduledOn != "vi-1" {
		t.Errorf("schedules set on %q, want vi-1 (the new volume's instance)", scheduledOn)
	}
	// The resize waits for the attach redeploy (a resize during it cancels it),
	// and the resize's own redeploy is then followed.
	if got := strings.Join(events, "|"); got != "list|create|list|resize|list" {
		t.Errorf("events = %s, want list|create|list|resize|list", got)
	}
	if result.DeploymentIDs["web"] != "dep-resize" {
		t.Errorf("DeploymentIDs = %v, want dep-resize", result.DeploymentIDs)
	}
}

func TestApply_UpdateVolumeAndImageRollsOutAfterRailway(t *testing.T) {
	// With other staged changes apply still rolls out itself, but only after
	// Railway's volume redeploy settled, so the two don't race.
	var events []string
	mock := updatedServiceMock(nil)
	mock.ListDeploymentsFunc = deploymentsSeq(&events, oldDeployment, railwayRedeploy)
	mock.CreateVolumeFunc = func(p, e, s, m, r string) (api.Volume, error) {
		events = append(events, "create")
		return api.Volume{ID: "vol-1"}, nil
	}
	mock.UpdateServiceInstanceFunc = func(s, e, image string, creds *api.RegistryCredentials) error {
		events = append(events, "image")
		return nil
	}
	mock.DeployServiceInstanceFunc = func(s, e string) (string, error) {
		events = append(events, "deploy")
		return "dep-1", nil
	}
	cs := &diff.ChangeSet{Changes: []diff.ResourceChange{{
		Type: diff.ChangeUpdate, ServiceName: "web",
		Fields: []diff.FieldDiff{
			{Path: "image", Current: "nginx:1.27", Desired: "nginx:1.28"},
			{Path: "volume.mountPath", Desired: "/data"},
		},
	}}}
	cfg := map[string]config.ServiceConfig{"web": {Name: "web", Image: "nginx:1.28", Volume: config.VolumeConfig{MountPath: "/data"}}}
	result := Apply(mock, cs, "p", "e", cfg, Opts{Output: io.Discard})
	if len(result.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	if got := strings.Join(events, "|"); got != "list|create|image|list|deploy" {
		t.Errorf("events = %s, want list|create|image|list|deploy (volume first, rollout after Railway's)", got)
	}
	if result.DeploymentIDs["web"] != "dep-1" {
		t.Errorf("DeploymentIDs = %v, want dep-1", result.DeploymentIDs)
	}
}

func TestApply_UpdateGrowsVolume(t *testing.T) {
	svcID := "svc-1"
	var events []string
	mock := updatedServiceMock([]api.VolumeInstance{
		{ID: "vi-1", Volume: api.Volume{ID: "vol-1"}, ServiceID: &svcID, MountPath: "/data", SizeMB: 50000},
	})
	mock.ListDeploymentsFunc = deploymentsSeq(&events, oldDeployment, railwayRedeploy)
	mock.ResizeVolumeFunc = func(e, volumeID string, sizeMB int) (string, error) {
		events = append(events, fmt.Sprintf("resize:%d", sizeMB))
		return "patch-1", nil
	}
	mock.DeployServiceInstanceFunc = func(s, e string) (string, error) {
		events = append(events, "deploy")
		return "dep-1", nil
	}
	cs := &diff.ChangeSet{Changes: []diff.ResourceChange{{
		Type: diff.ChangeUpdate, ServiceName: "web",
		Fields: []diff.FieldDiff{{Path: "volume.sizeMB", Current: "50000", Desired: "250000"}},
	}}}
	cfg := map[string]config.ServiceConfig{"web": {Name: "web", Volume: config.VolumeConfig{MountPath: "/data", SizeMB: 250000}}}
	result := Apply(mock, cs, "p", "e", cfg, Opts{Output: io.Discard})
	if len(result.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	if got := strings.Join(events, "|"); got != "list|resize:250000|list" {
		t.Errorf("events = %s, want list|resize:250000|list (Railway's resize redeploy followed)", got)
	}
}

func TestApply_UpdateResizeFailureWritesNothingElse(t *testing.T) {
	// The volume step runs first, so a refused resize leaves the image and
	// variables unwritten instead of staged with no rollout.
	svcID := "svc-1"
	var writes []string
	mock := updatedServiceMock([]api.VolumeInstance{
		{ID: "vi-1", Volume: api.Volume{ID: "vol-1"}, ServiceID: &svcID, MountPath: "/data", SizeMB: 50000},
	})
	mock.ResizeVolumeFunc = func(e, v string, s int) (string, error) { return "patch-1", nil }
	mock.GetEnvironmentPatchStatusFunc = func(id string) (api.EnvironmentPatchStatus, error) {
		return api.EnvironmentPatchStatus{Status: "FAILED", LastAppliedError: "plan limit reached"}, nil
	}
	mock.UpdateServiceInstanceFunc = func(s, e, image string, creds *api.RegistryCredentials) error {
		writes = append(writes, "image")
		return nil
	}
	mock.SetVariablesFunc = func(p, e, s string, vars map[string]string, skip bool) error {
		writes = append(writes, "variables")
		return nil
	}
	mock.DeployServiceInstanceFunc = func(s, e string) (string, error) {
		writes = append(writes, "deploy")
		return "dep-1", nil
	}
	cs := &diff.ChangeSet{Changes: []diff.ResourceChange{{
		Type: diff.ChangeUpdate, ServiceName: "web",
		Fields: []diff.FieldDiff{
			{Path: "image", Current: "nginx:1.27", Desired: "nginx:1.28"},
			{Path: "variables.MODE", Desired: "fast"},
			{Path: "volume.sizeMB", Current: "50000", Desired: "2000000"},
		},
	}}}
	cfg := map[string]config.ServiceConfig{"web": {
		Name: "web", Image: "nginx:1.28", Variables: map[string]string{"MODE": "fast"},
		Volume: config.VolumeConfig{MountPath: "/data", SizeMB: 2000000},
	}}
	result := Apply(mock, cs, "p", "e", cfg, Opts{Output: io.Discard})
	if len(result.Errors) != 1 || !strings.Contains(result.Errors[0].Error(), "plan limit reached") {
		t.Fatalf("expected Railway's refusal, got %v", result.Errors)
	}
	if len(writes) != 0 {
		t.Errorf("a refused resize must leave the rest unwritten, got %v", writes)
	}
}

func TestApply_UpdateResizeUnknownPatchStatus(t *testing.T) {
	// A status other than STAGED/APPLYING/COMMITTED/FAILED fails at once
	// instead of polling to the deadline.
	svcID := "svc-1"
	checks := 0
	mock := updatedServiceMock([]api.VolumeInstance{
		{ID: "vi-1", Volume: api.Volume{ID: "vol-1"}, ServiceID: &svcID, MountPath: "/data", SizeMB: 50000},
	})
	mock.ResizeVolumeFunc = func(e, v string, s int) (string, error) { return "patch-1", nil }
	mock.GetEnvironmentPatchStatusFunc = func(id string) (api.EnvironmentPatchStatus, error) {
		checks++
		return api.EnvironmentPatchStatus{Status: "SKIPPED"}, nil
	}
	cs := &diff.ChangeSet{Changes: []diff.ResourceChange{{
		Type: diff.ChangeUpdate, ServiceName: "web",
		Fields: []diff.FieldDiff{{Path: "volume.sizeMB", Current: "50000", Desired: "250000"}},
	}}}
	cfg := map[string]config.ServiceConfig{"web": {Name: "web", Volume: config.VolumeConfig{MountPath: "/data", SizeMB: 250000}}}
	result := Apply(mock, cs, "p", "e", cfg, Opts{Output: io.Discard})
	if len(result.Errors) != 1 || !strings.Contains(result.Errors[0].Error(), "SKIPPED") {
		t.Fatalf("expected the unknown status in the error, got %v", result.Errors)
	}
	if checks != 1 {
		t.Errorf("status checked %d times, want 1", checks)
	}
}

func TestApply_UpdateResizeRefusedByRailway(t *testing.T) {
	// The commit succeeds; only the patch reports the refusal.
	svcID := "svc-1"
	statuses := []string{"APPLYING", "FAILED"}
	mock := updatedServiceMock([]api.VolumeInstance{
		{ID: "vi-1", Volume: api.Volume{ID: "vol-1"}, ServiceID: &svcID, MountPath: "/data", SizeMB: 50000},
	})
	mock.ResizeVolumeFunc = func(e, v string, s int) (string, error) { return "patch-1", nil }
	mock.GetEnvironmentPatchStatusFunc = func(id string) (api.EnvironmentPatchStatus, error) {
		st := statuses[0]
		if len(statuses) > 1 {
			statuses = statuses[1:]
		}
		return api.EnvironmentPatchStatus{Status: st, LastAppliedError: "plan limit reached"}, nil
	}
	cs := &diff.ChangeSet{Changes: []diff.ResourceChange{{
		Type: diff.ChangeUpdate, ServiceName: "web",
		Fields: []diff.FieldDiff{{Path: "volume.sizeMB", Current: "50000", Desired: "2000000"}},
	}}}
	cfg := map[string]config.ServiceConfig{"web": {Name: "web", Volume: config.VolumeConfig{MountPath: "/data", SizeMB: 2000000}}}
	result := Apply(mock, cs, "p", "e", cfg, Opts{Output: io.Discard})
	if len(result.Errors) != 1 || !strings.Contains(result.Errors[0].Error(), "plan limit reached") {
		t.Fatalf("expected Railway's refusal to surface, got %v", result.Errors)
	}
}

func TestApply_CreateServiceWithVolumeSize(t *testing.T) {
	svcID := "svc-1"
	tests := []struct {
		name      string
		sizeMB    int
		wantCalls string
	}{
		// Grown before the first rollout, so it deploys onto the full size.
		{name: "above the default", sizeMB: 250000, wantCalls: "resize:250000|deploy"},
		// sizeMB is a minimum: a plan default already above it is kept.
		{name: "below the default", sizeMB: 5000, wantCalls: "deploy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls []string
			mock := &api.MockClient{
				CreateServiceFunc: func(p, e, name, image string, creds *api.RegistryCredentials) (types.Service, error) {
					return types.Service{ID: svcID, Name: name}, nil
				},
				CreateVolumeFunc: func(p, e, s, m, r string) (api.Volume, error) { return api.Volume{ID: "vol-1"}, nil },
				ListVolumesFunc: func(p, e string) ([]api.VolumeInstance, error) {
					return []api.VolumeInstance{{ID: "vi-1", Volume: api.Volume{ID: "vol-1"}, ServiceID: &svcID, MountPath: "/data", SizeMB: 50000}}, nil
				},
				ResizeVolumeFunc: func(e, v string, sizeMB int) (string, error) {
					calls = append(calls, fmt.Sprintf("resize:%d", sizeMB))
					return "patch-1", nil
				},
				AttachSourceAndDeployFunc: func(s, e, image string, creds *api.RegistryCredentials) (string, error) {
					calls = append(calls, "deploy")
					return "dep-1", nil
				},
			}
			cs := &diff.ChangeSet{Changes: []diff.ResourceChange{{Type: diff.ChangeCreate, ServiceName: "web"}}}
			cfg := map[string]config.ServiceConfig{"web": {
				Name: "web", Image: "nginx", Volume: config.VolumeConfig{MountPath: "/data", SizeMB: tt.sizeMB},
			}}
			result := Apply(mock, cs, "p", "e", cfg, Opts{Output: io.Discard})
			if len(result.Errors) != 0 {
				t.Fatalf("unexpected errors: %v", result.Errors)
			}
			if got := strings.Join(calls, "|"); got != tt.wantCalls {
				t.Errorf("calls = %s, want %s", got, tt.wantCalls)
			}
		})
	}
}

func TestApply_CreateServiceWithVolumeBackupSchedules(t *testing.T) {
	var capturedInstanceID string
	var capturedKinds []string

	svcID := "svc-1"
	mock := &api.MockClient{
		CreateServiceFunc: func(projectID, envID, name, image string, creds *api.RegistryCredentials) (types.Service, error) {
			return types.Service{ID: "svc-1", Name: name}, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return []types.Environment{{ID: "env-1", Name: "production"}}, nil
		},
		CreateVolumeFunc: func(projectID, envID, serviceID, mountPath string, _ string) (api.Volume, error) {
			return api.Volume{ID: "vol-1", Name: "data"}, nil
		},
		ListVolumesFunc: func(projectID, environmentID string) ([]api.VolumeInstance, error) {
			return []api.VolumeInstance{
				{ID: "vi-1", Volume: api.Volume{ID: "vol-1", Name: "data"}, ServiceID: &svcID, MountPath: "/data"},
			}, nil
		},
		SetVolumeBackupSchedulesFunc: func(volumeInstanceID string, kinds []string) error {
			capturedInstanceID = volumeInstanceID
			capturedKinds = kinds
			return nil
		},
	}

	cs := &diff.ChangeSet{
		Changes: []diff.ResourceChange{
			{
				Type:        diff.ChangeCreate,
				ServiceName: "db",
				Fields: []diff.FieldDiff{
					{Path: "image", Desired: "postgres:16"},
					{Path: "volume.mountPath", Desired: "/data"},
					{Path: "volume.backupSchedules", Desired: "DAILY,WEEKLY"},
				},
			},
		},
	}

	configMap := map[string]config.ServiceConfig{
		"db": {
			Name:   "db",
			Image:  "postgres:16",
			Volume: config.VolumeConfig{MountPath: "/data", BackupSchedules: []string{"DAILY", "WEEKLY"}},
		},
	}

	result := Apply(mock, cs, "proj-1", "env-1", configMap, Opts{Output: io.Discard})
	if len(result.Errors) != 0 {
		t.Fatalf("expected no errors, got %v", result.Errors)
	}
	if capturedInstanceID != "vi-1" {
		t.Errorf("expected instance vi-1, got %q", capturedInstanceID)
	}
	if len(capturedKinds) != 2 || capturedKinds[0] != "DAILY" || capturedKinds[1] != "WEEKLY" {
		t.Errorf("expected kinds [DAILY WEEKLY], got %v", capturedKinds)
	}
}

func TestApply_UpdateVolumeBackupSchedules(t *testing.T) {
	var capturedInstanceID string
	var capturedKinds []string

	svcID := "svc-1"
	mock := &api.MockClient{
		ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{{ID: "svc-1", Name: "db"}}, nil
		},
		ListVolumesFunc: func(projectID, environmentID string) ([]api.VolumeInstance, error) {
			return []api.VolumeInstance{
				{ID: "vi-1", Volume: api.Volume{ID: "vol-1", Name: "data"}, ServiceID: &svcID, MountPath: "/data"},
			}, nil
		},
		SetVolumeBackupSchedulesFunc: func(volumeInstanceID string, kinds []string) error {
			capturedInstanceID = volumeInstanceID
			capturedKinds = kinds
			return nil
		},
	}

	cs := &diff.ChangeSet{
		Changes: []diff.ResourceChange{
			{
				Type:        diff.ChangeUpdate,
				ServiceName: "db",
				Fields: []diff.FieldDiff{
					{Path: "volume.backupSchedules", Current: "DAILY", Desired: "DAILY,WEEKLY"},
				},
			},
		},
	}

	configMap := map[string]config.ServiceConfig{
		"db": {
			Name:   "db",
			Image:  "postgres:16",
			Volume: config.VolumeConfig{MountPath: "/data", BackupSchedules: []string{"DAILY", "WEEKLY"}},
		},
	}

	result := Apply(mock, cs, "proj-1", "env-1", configMap, Opts{Output: io.Discard})
	if len(result.Errors) != 0 {
		t.Fatalf("expected no errors, got %v", result.Errors)
	}
	if capturedInstanceID != "vi-1" {
		t.Errorf("expected instance vi-1, got %q", capturedInstanceID)
	}
	if len(capturedKinds) != 2 {
		t.Errorf("expected 2 kinds, got %v", capturedKinds)
	}
	if len(result.Updated) != 1 || result.Updated[0] != "db" {
		t.Errorf("expected Updated=[db], got %v", result.Updated)
	}
}

func TestApply_UpdateBackupSchedules_VolumeMissing(t *testing.T) {
	// A service declaring backupSchedules but with no live volume yet should
	// warn and continue (not fail the update).
	setCalled := false
	mock := &api.MockClient{
		ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{{ID: "svc-1", Name: "db"}}, nil
		},
		ListVolumesFunc: func(projectID, environmentID string) ([]api.VolumeInstance, error) {
			return []api.VolumeInstance{}, nil // no volume attached yet
		},
		SetVolumeBackupSchedulesFunc: func(volumeInstanceID string, kinds []string) error {
			setCalled = true
			return nil
		},
	}

	cs := &diff.ChangeSet{
		Changes: []diff.ResourceChange{
			{
				Type:        diff.ChangeUpdate,
				ServiceName: "db",
				Fields:      []diff.FieldDiff{{Path: "volume.backupSchedules", Desired: "DAILY"}},
			},
		},
	}
	configMap := map[string]config.ServiceConfig{
		"db": {Name: "db", Image: "postgres:16", Volume: config.VolumeConfig{MountPath: "/data", BackupSchedules: []string{"DAILY"}}},
	}

	result := Apply(mock, cs, "proj-1", "env-1", configMap, Opts{Output: io.Discard})
	if len(result.Errors) != 0 {
		t.Fatalf("expected no errors (should warn), got %v", result.Errors)
	}
	if setCalled {
		t.Error("SetVolumeBackupSchedules should not be called when no volume exists")
	}
	if len(result.Updated) != 1 {
		t.Errorf("expected the service to still count as updated, got %v", result.Updated)
	}
}

func TestApply_UpdateBackupSchedules_ListError(t *testing.T) {
	// A transient ListVolumes failure must propagate, not be swallowed as
	// "no volume yet".
	mock := &api.MockClient{
		ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{{ID: "svc-1", Name: "db"}}, nil
		},
		ListVolumesFunc: func(projectID, environmentID string) ([]api.VolumeInstance, error) {
			return nil, errors.New("boom: API unavailable")
		},
	}

	cs := &diff.ChangeSet{
		Changes: []diff.ResourceChange{
			{
				Type:        diff.ChangeUpdate,
				ServiceName: "db",
				Fields:      []diff.FieldDiff{{Path: "volume.backupSchedules", Desired: "DAILY"}},
			},
		},
	}
	configMap := map[string]config.ServiceConfig{
		"db": {Name: "db", Image: "postgres:16", Volume: config.VolumeConfig{MountPath: "/data", BackupSchedules: []string{"DAILY"}}},
	}

	result := Apply(mock, cs, "proj-1", "env-1", configMap, Opts{Output: io.Discard})
	if len(result.Errors) == 0 {
		t.Fatal("expected an error to propagate from ListVolumes failure")
	}
}

func TestApply_ClearVolumeBackupSchedules(t *testing.T) {
	var capturedKinds []string

	svcID := "svc-1"
	mock := &api.MockClient{
		ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{{ID: "svc-1", Name: "db"}}, nil
		},
		ListVolumesFunc: func(projectID, environmentID string) ([]api.VolumeInstance, error) {
			return []api.VolumeInstance{
				{ID: "vi-1", Volume: api.Volume{ID: "vol-1", Name: "data"}, ServiceID: &svcID, MountPath: "/data"},
			}, nil
		},
		SetVolumeBackupSchedulesFunc: func(volumeInstanceID string, kinds []string) error {
			capturedKinds = kinds
			return nil
		},
	}

	cs := &diff.ChangeSet{
		Changes: []diff.ResourceChange{
			{
				Type:        diff.ChangeUpdate,
				ServiceName: "db",
				Fields: []diff.FieldDiff{
					{Path: "volume.backupSchedules", Current: "DAILY,WEEKLY", Desired: ""},
				},
			},
		},
	}

	configMap := map[string]config.ServiceConfig{
		"db": {
			Name:   "db",
			Image:  "postgres:16",
			Volume: config.VolumeConfig{MountPath: "/data"}, // no schedules declared → clear
		},
	}

	var out bytes.Buffer
	result := Apply(mock, cs, "proj-1", "env-1", configMap, Opts{Output: &out})
	if len(result.Errors) != 0 {
		t.Fatalf("expected no errors, got %v", result.Errors)
	}
	if len(capturedKinds) != 0 {
		t.Errorf("expected schedules to be cleared (empty kinds), got %v", capturedKinds)
	}
	if !strings.Contains(out.String(), "cleared (were: DAILY, WEEKLY)") {
		t.Errorf("expected clear warning naming previous schedules, got:\n%s", out.String())
	}
}
