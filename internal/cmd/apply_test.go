package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/types"
)

// setupApplyTestGlobals saves and restores global state for apply/diff tests.
type applyTestCleanup struct {
	origAPIClient func(string) api.APIClient
	origProject   string
	origEnv       string
	origToken     string
}

func saveApplyGlobals() applyTestCleanup {
	return applyTestCleanup{
		origAPIClient: newAPIClient,
		origProject:   project,
		origEnv:       environment,
		origToken:     token,
	}
}

func (c applyTestCleanup) restore() {
	newAPIClient = c.origAPIClient
	project = c.origProject
	environment = c.origEnv
	token = c.origToken
}

// writeTestConfig writes a YAML config to a temp file and returns its path.
func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}
	return path
}

// writeTestConfigDir writes multiple YAML configs to a temp directory.
func writeTestConfigDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("writing test config %s: %v", name, err)
		}
	}
	return dir
}

func newMockForApply(liveServices []types.ServiceDetail) *api.MockClient {
	port := 8080
	return &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{{ID: "proj-1", Name: "test-project"}}, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return []types.Environment{{ID: "env-1", Name: "test-env"}}, nil
		},
		ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
			return liveServices, nil
		},
		GetVariablesFunc: func(projectID, environmentID, serviceID string) (map[string]string, error) {
			return map[string]string{"PORT": "3000"}, nil
		},
		// The diff reads raw (unrendered) variables via GetRawVariables.
		GetRawVariablesFunc: func(projectID, environmentID, serviceID string) (map[string]string, error) {
			return map[string]string{"PORT": "3000"}, nil
		},
		ListVolumesFunc: func(projectID, environmentID string) ([]api.VolumeInstance, error) {
			return nil, nil
		},
		ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
			return api.DomainList{
				ServiceDomains: []api.ServiceDomain{
					{ID: "dom-1", Domain: "svc.up.railway.app", TargetPort: &port},
				},
			}, nil
		},
		ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
			return nil, nil
		},
	}
}

const testConfig = `
services:
  - name: api
    image: nginx:latest
    variables:
      PORT: "3000"
`

const testConfigDifferent = `
services:
  - name: api
    image: node:20
    variables:
      PORT: "8080"
`

func TestApply_DryRun(t *testing.T) {
	cleanup := saveApplyGlobals()
	defer cleanup.restore()

	configFile := writeTestConfig(t, testConfigDifferent)

	liveServices := []types.ServiceDetail{
		{ID: "svc-1", Name: "api", Source: "nginx:latest"},
	}

	mock := newMockForApply(liveServices)
	token = "test-token"
	project = "test-project"
	environment = "test-env"
	newAPIClient = func(tkn string) api.APIClient { return mock }

	// Save and restore applyDryRun flag.
	origDryRun := applyDryRun
	origFile := applyFile
	origPrune := applyPrune
	origNoColor := applyNoColor
	defer func() {
		applyDryRun = origDryRun
		applyFile = origFile
		applyPrune = origPrune
		applyNoColor = origNoColor
	}()

	applyFile = configFile
	applyDryRun = true
	applyPrune = false
	applyNoColor = true

	// Capture stdout.
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runApply(applyCmd, []string{})

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Fatalf("expected no error on dry-run, got: %v", err)
	}

	// Should show diff output but not actually apply.
	if !strings.Contains(output, "image") {
		t.Errorf("expected dry-run output to contain diff info about image, got:\n%s", output)
	}
}

func TestApply_NoChanges(t *testing.T) {
	cleanup := saveApplyGlobals()
	defer cleanup.restore()

	configFile := writeTestConfig(t, testConfig)

	liveServices := []types.ServiceDetail{
		{
			ID:     "svc-1",
			Name:   "api",
			Source: "nginx:latest",
		},
	}

	mock := newMockForApply(liveServices)
	// Override: no domains in live state (config has no domain either).
	mock.ListDomainsFunc = func(projectID, environmentID, serviceID string) (api.DomainList, error) {
		return api.DomainList{}, nil
	}
	token = "test-token"
	project = "test-project"
	environment = "test-env"
	newAPIClient = func(tkn string) api.APIClient { return mock }

	origDryRun := applyDryRun
	origFile := applyFile
	origPrune := applyPrune
	origNoColor := applyNoColor
	defer func() {
		applyDryRun = origDryRun
		applyFile = origFile
		applyPrune = origPrune
		applyNoColor = origNoColor
	}()

	applyFile = configFile
	applyDryRun = false
	applyPrune = false
	applyNoColor = true

	// Capture stdout.
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runApply(applyCmd, []string{})

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Fatalf("expected no error when no changes, got: %v", err)
	}

	if !strings.Contains(output, "No changes") {
		t.Errorf("expected 'No changes' message, got:\n%s", output)
	}
}

func TestDiff_ShowsChanges(t *testing.T) {
	cleanup := saveApplyGlobals()
	defer cleanup.restore()

	configFile := writeTestConfig(t, testConfigDifferent)

	liveServices := []types.ServiceDetail{
		{ID: "svc-1", Name: "api", Source: "nginx:latest"},
	}

	mock := newMockForApply(liveServices)
	token = "test-token"
	project = "test-project"
	environment = "test-env"
	newAPIClient = func(tkn string) api.APIClient { return mock }

	origFile := diffFile
	origPrune := diffPrune
	origNoColor := diffNoColor
	defer func() {
		diffFile = origFile
		diffPrune = origPrune
		diffNoColor = origNoColor
	}()

	diffFile = configFile
	diffPrune = false
	diffNoColor = true

	// Capture stdout.
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runDiff(diffCmd, []string{})

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// diff always succeeds — a diff with changes is a report, not a failure.
	if err != nil {
		t.Fatalf("expected diff to succeed, got: %v", err)
	}

	// Output should contain the diff.
	if !strings.Contains(output, "image") {
		t.Errorf("expected diff output to contain image changes, got:\n%s", output)
	}
	if !strings.Contains(output, "node:20") {
		t.Errorf("expected diff output to contain desired image 'node:20', got:\n%s", output)
	}
}

func TestApply_LoadFile(t *testing.T) {
	cleanup := saveApplyGlobals()
	defer cleanup.restore()

	configFile := writeTestConfig(t, testConfig)

	port := 8080
	liveServices := []types.ServiceDetail{
		{
			ID:     "svc-1",
			Name:   "api",
			Source: "nginx:latest",
			ServiceDomains: []types.ServiceDomain{
				{ID: "dom-1", Domain: "svc.up.railway.app", TargetPort: &port},
			},
		},
	}

	mock := newMockForApply(liveServices)
	token = "test-token"
	project = "test-project"
	environment = "test-env"
	newAPIClient = func(tkn string) api.APIClient { return mock }

	origDryRun := applyDryRun
	origFile := applyFile
	origPrune := applyPrune
	origNoColor := applyNoColor
	defer func() {
		applyDryRun = origDryRun
		applyFile = origFile
		applyPrune = origPrune
		applyNoColor = origNoColor
	}()

	applyFile = configFile
	applyDryRun = true
	applyPrune = false
	applyNoColor = true

	err := runApply(applyCmd, []string{})
	if err != nil {
		t.Fatalf("expected no error loading file config, got: %v", err)
	}
}

func TestApply_LoadDir(t *testing.T) {
	cleanup := saveApplyGlobals()
	defer cleanup.restore()

	dir := writeTestConfigDir(t, map[string]string{
		"api.yaml": `
services:
  - name: api
    image: nginx:latest
`,
		"worker.yaml": `
services:
  - name: worker
    image: node:20
`,
	})

	port := 8080
	liveServices := []types.ServiceDetail{
		{
			ID:     "svc-1",
			Name:   "api",
			Source: "nginx:latest",
			ServiceDomains: []types.ServiceDomain{
				{ID: "dom-1", Domain: "api.up.railway.app", TargetPort: &port},
			},
		},
		{
			ID:     "svc-2",
			Name:   "worker",
			Source: "node:20",
			ServiceDomains: []types.ServiceDomain{
				{ID: "dom-2", Domain: "worker.up.railway.app", TargetPort: &port},
			},
		},
	}

	mock := newMockForApply(liveServices)
	token = "test-token"
	project = "test-project"
	environment = "test-env"
	newAPIClient = func(tkn string) api.APIClient { return mock }

	origDryRun := applyDryRun
	origFile := applyFile
	origPrune := applyPrune
	origNoColor := applyNoColor
	defer func() {
		applyDryRun = origDryRun
		applyFile = origFile
		applyPrune = origPrune
		applyNoColor = origNoColor
	}()

	applyFile = dir
	applyDryRun = true
	applyPrune = false
	applyNoColor = true

	err := runApply(applyCmd, []string{})
	if err != nil {
		t.Fatalf("expected no error loading directory config, got: %v", err)
	}
}
