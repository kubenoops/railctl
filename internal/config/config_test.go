package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_NewFormat(t *testing.T) {
	dir := t.TempDir()
	content := `
project: my-project
environment: production
services:
  - name: web
    image: nginx:latest
    deploy:
      startCommand: "nginx -g 'daemon off;'"
      restartPolicy: ON_FAILURE
      maxRetries: 3
      replicas: 2
      healthcheckPath: /health
      healthcheckTimeout: 30
    networking:
      domain:
        port: 8080
      tcpProxy:
        port: 5432
    volume:
      mountPath: /data
    variables:
      ENV: production
      DEBUG: "false"
    registry:
      username: user
      password: pass
  - name: worker
    image: myapp:worker
    deploy:
      replicas: 3
`
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Project != "my-project" {
		t.Errorf("project = %q, want %q", cfg.Project, "my-project")
	}
	if cfg.Environment != "production" {
		t.Errorf("environment = %q, want %q", cfg.Environment, "production")
	}
	if len(cfg.Services) != 2 {
		t.Fatalf("len(services) = %d, want 2", len(cfg.Services))
	}

	web := cfg.Services[0]
	if web.Name != "web" {
		t.Errorf("services[0].name = %q, want %q", web.Name, "web")
	}
	if web.Image != "nginx:latest" {
		t.Errorf("services[0].image = %q, want %q", web.Image, "nginx:latest")
	}
	if web.Deploy.StartCommand != "nginx -g 'daemon off;'" {
		t.Errorf("services[0].deploy.startCommand = %q, want %q", web.Deploy.StartCommand, "nginx -g 'daemon off;'")
	}
	if web.Deploy.RestartPolicy != "ON_FAILURE" {
		t.Errorf("services[0].deploy.restartPolicy = %q, want %q", web.Deploy.RestartPolicy, "ON_FAILURE")
	}
	if web.Deploy.MaxRetries != 3 {
		t.Errorf("services[0].deploy.maxRetries = %d, want 3", web.Deploy.MaxRetries)
	}
	if web.Deploy.Replicas != 2 {
		t.Errorf("services[0].deploy.replicas = %d, want 2", web.Deploy.Replicas)
	}
	if web.Deploy.HealthcheckPath != "/health" {
		t.Errorf("services[0].deploy.healthcheckPath = %q, want %q", web.Deploy.HealthcheckPath, "/health")
	}
	if web.Deploy.HealthcheckTimeout != 30 {
		t.Errorf("services[0].deploy.healthcheckTimeout = %d, want 30", web.Deploy.HealthcheckTimeout)
	}
	if web.Networking.Domain.Port != 8080 {
		t.Errorf("services[0].networking.domain.port = %d, want 8080", web.Networking.Domain.Port)
	}
	if web.Networking.TCPProxy.Port != 5432 {
		t.Errorf("services[0].networking.tcpProxy.port = %d, want 5432", web.Networking.TCPProxy.Port)
	}
	if web.Volume.MountPath != "/data" {
		t.Errorf("services[0].volume.mountPath = %q, want %q", web.Volume.MountPath, "/data")
	}
	if web.Variables["ENV"] != "production" {
		t.Errorf("services[0].variables[ENV] = %q, want %q", web.Variables["ENV"], "production")
	}
	if web.Variables["DEBUG"] != "false" {
		t.Errorf("services[0].variables[DEBUG] = %q, want %q", web.Variables["DEBUG"], "false")
	}
	if web.Registry.Username != "user" {
		t.Errorf("services[0].registry.username = %q, want %q", web.Registry.Username, "user")
	}
	if web.Registry.Password != "pass" {
		t.Errorf("services[0].registry.password = %q, want %q", web.Registry.Password, "pass")
	}

	worker := cfg.Services[1]
	if worker.Name != "worker" {
		t.Errorf("services[1].name = %q, want %q", worker.Name, "worker")
	}
	if worker.Deploy.Replicas != 3 {
		t.Errorf("services[1].deploy.replicas = %d, want 3", worker.Deploy.Replicas)
	}
}

func TestLoad_LegacyFormat(t *testing.T) {
	dir := t.TempDir()
	content := `
service:
  name: my-api
  image: myapp:latest
deploy:
  startCommand: "go run main.go"
  restartPolicyType: ON_FAILURE
  restartPolicyMaxRetries: 10
  numReplicas: 2
  healthcheckPath: /healthz
  healthcheckTimeout: 15
domain:
  port: 5678
networking:
  tcpProxyPort: 5432
volume:
  mountPath: /data
registry:
  username: reg-user
  password: reg-pass
variables:
  KEY: "value"
  OTHER: "stuff"
`
	path := filepath.Join(dir, "legacy.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if len(cfg.Services) != 1 {
		t.Fatalf("len(services) = %d, want 1", len(cfg.Services))
	}

	svc := cfg.Services[0]
	if svc.Name != "my-api" {
		t.Errorf("name = %q, want %q", svc.Name, "my-api")
	}
	if svc.Image != "myapp:latest" {
		t.Errorf("image = %q, want %q", svc.Image, "myapp:latest")
	}
	if svc.Deploy.StartCommand != "go run main.go" {
		t.Errorf("deploy.startCommand = %q, want %q", svc.Deploy.StartCommand, "go run main.go")
	}
	if svc.Deploy.RestartPolicy != "ON_FAILURE" {
		t.Errorf("deploy.restartPolicy = %q, want %q", svc.Deploy.RestartPolicy, "ON_FAILURE")
	}
	if svc.Deploy.MaxRetries != 10 {
		t.Errorf("deploy.maxRetries = %d, want 10", svc.Deploy.MaxRetries)
	}
	if svc.Deploy.Replicas != 2 {
		t.Errorf("deploy.replicas = %d, want 2", svc.Deploy.Replicas)
	}
	if svc.Deploy.HealthcheckPath != "/healthz" {
		t.Errorf("deploy.healthcheckPath = %q, want %q", svc.Deploy.HealthcheckPath, "/healthz")
	}
	if svc.Deploy.HealthcheckTimeout != 15 {
		t.Errorf("deploy.healthcheckTimeout = %d, want 15", svc.Deploy.HealthcheckTimeout)
	}
	if svc.Networking.Domain.Port != 5678 {
		t.Errorf("networking.domain.port = %d, want 5678", svc.Networking.Domain.Port)
	}
	if svc.Networking.TCPProxy.Port != 5432 {
		t.Errorf("networking.tcpProxy.port = %d, want 5432", svc.Networking.TCPProxy.Port)
	}
	if svc.Volume.MountPath != "/data" {
		t.Errorf("volume.mountPath = %q, want %q", svc.Volume.MountPath, "/data")
	}
	if svc.Registry.Username != "reg-user" {
		t.Errorf("registry.username = %q, want %q", svc.Registry.Username, "reg-user")
	}
	if svc.Registry.Password != "reg-pass" {
		t.Errorf("registry.password = %q, want %q", svc.Registry.Password, "reg-pass")
	}
	if svc.Variables["KEY"] != "value" {
		t.Errorf("variables[KEY] = %q, want %q", svc.Variables["KEY"], "value")
	}
	if svc.Variables["OTHER"] != "stuff" {
		t.Errorf("variables[OTHER] = %q, want %q", svc.Variables["OTHER"], "stuff")
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte(":\n  :\n  - [invalid yaml\n"), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestValidate_Valid(t *testing.T) {
	cfg := &Config{
		Services: []ServiceConfig{
			{
				Name:  "api",
				Image: "myapp:latest",
				Deploy: DeployConfig{
					RestartPolicy: "on_failure",
					MaxRetries:    3,
					Replicas:      2,
				},
				Networking: NetworkingConfig{
					Domain:   DomainConfig{Port: 8080},
					TCPProxy: TCPProxyConfig{Port: 5432},
				},
			},
		},
	}

	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate() error: %v", err)
	}

	// Verify normalization.
	if cfg.Services[0].Deploy.RestartPolicy != "ON_FAILURE" {
		t.Errorf("restartPolicy = %q, want %q", cfg.Services[0].Deploy.RestartPolicy, "ON_FAILURE")
	}
}

func TestValidate_MissingName(t *testing.T) {
	cfg := &Config{
		Services: []ServiceConfig{
			{Image: "myapp:latest"},
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for missing name, got nil")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("error = %q, want it to contain 'name is required'", err.Error())
	}
}

func TestValidate_MissingImage(t *testing.T) {
	cfg := &Config{
		Services: []ServiceConfig{
			{Name: "api"},
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for missing image, got nil")
	}
	if !strings.Contains(err.Error(), "image is required") {
		t.Errorf("error = %q, want it to contain 'image is required'", err.Error())
	}
}

func TestValidate_DuplicateNames(t *testing.T) {
	cfg := &Config{
		Services: []ServiceConfig{
			{Name: "api", Image: "img1"},
			{Name: "api", Image: "img2"},
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for duplicate names, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate service name") {
		t.Errorf("error = %q, want it to contain 'duplicate service name'", err.Error())
	}
}

func TestValidate_InvalidRestartPolicy(t *testing.T) {
	cfg := &Config{
		Services: []ServiceConfig{
			{
				Name:  "api",
				Image: "img",
				Deploy: DeployConfig{
					RestartPolicy: "INVALID",
				},
			},
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid restart policy, got nil")
	}
	if !strings.Contains(err.Error(), "invalid restartPolicy") {
		t.Errorf("error = %q, want it to contain 'invalid restartPolicy'", err.Error())
	}
}

func TestValidate_MaxRetriesWithoutPolicy(t *testing.T) {
	cfg := &Config{
		Services: []ServiceConfig{
			{
				Name:  "api",
				Image: "img",
				Deploy: DeployConfig{
					MaxRetries: 5,
				},
			},
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for maxRetries without restartPolicy, got nil")
	}
	if !strings.Contains(err.Error(), "maxRetries requires restartPolicy") {
		t.Errorf("error = %q, want it to contain 'maxRetries requires restartPolicy'", err.Error())
	}
}

func TestValidate_InvalidReplicas(t *testing.T) {
	cfg := &Config{
		Services: []ServiceConfig{
			{
				Name:  "api",
				Image: "img",
				Deploy: DeployConfig{
					Replicas: -1,
				},
			},
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid replicas, got nil")
	}
	if !strings.Contains(err.Error(), "replicas must be >= 1") {
		t.Errorf("error = %q, want it to contain 'replicas must be >= 1'", err.Error())
	}
}

func TestValidate_InvalidPort(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
		want string
	}{
		{
			name: "domain port too high",
			cfg: &Config{
				Services: []ServiceConfig{
					{
						Name:  "api",
						Image: "img",
						Networking: NetworkingConfig{
							Domain: DomainConfig{Port: 70000},
						},
					},
				},
			},
			want: "domain port must be between 1 and 65535",
		},
		{
			name: "tcpProxy port negative",
			cfg: &Config{
				Services: []ServiceConfig{
					{
						Name:  "api",
						Image: "img",
						Networking: NetworkingConfig{
							TCPProxy: TCPProxyConfig{Port: -1},
						},
					},
				},
			},
			want: "tcpProxy port must be between 1 and 65535",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.cfg)
			if err == nil {
				t.Fatal("expected error for invalid port, got nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), tt.want)
			}
		})
	}
}

func TestLoadDir(t *testing.T) {
	dir := t.TempDir()

	file1 := `
project: my-project
environment: staging
services:
  - name: api
    image: api:latest
`
	file2 := `
project: my-project
environment: staging
services:
  - name: worker
    image: worker:latest
`
	if err := os.WriteFile(filepath.Join(dir, "01-api.yaml"), []byte(file1), 0644); err != nil {
		t.Fatalf("writing file1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "02-worker.yml"), []byte(file2), 0644); err != nil {
		t.Fatalf("writing file2: %v", err)
	}

	cfg, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir() error: %v", err)
	}

	if cfg.Project != "my-project" {
		t.Errorf("project = %q, want %q", cfg.Project, "my-project")
	}
	if cfg.Environment != "staging" {
		t.Errorf("environment = %q, want %q", cfg.Environment, "staging")
	}
	if len(cfg.Services) != 2 {
		t.Fatalf("len(services) = %d, want 2", len(cfg.Services))
	}
	if cfg.Services[0].Name != "api" {
		t.Errorf("services[0].name = %q, want %q", cfg.Services[0].Name, "api")
	}
	if cfg.Services[1].Name != "worker" {
		t.Errorf("services[1].name = %q, want %q", cfg.Services[1].Name, "worker")
	}
}

func TestLoadDir_ConflictingProject(t *testing.T) {
	dir := t.TempDir()

	file1 := `
project: project-a
services:
  - name: api
    image: api:latest
`
	file2 := `
project: project-b
services:
  - name: worker
    image: worker:latest
`
	if err := os.WriteFile(filepath.Join(dir, "01.yaml"), []byte(file1), 0644); err != nil {
		t.Fatalf("writing file1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "02.yaml"), []byte(file2), 0644); err != nil {
		t.Fatalf("writing file2: %v", err)
	}

	_, err := LoadDir(dir)
	if err == nil {
		t.Fatal("expected error for conflicting projects, got nil")
	}
	if !strings.Contains(err.Error(), "conflicting project") {
		t.Errorf("error = %q, want it to contain 'conflicting project'", err.Error())
	}
}
