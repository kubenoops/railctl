package config

import (
	"os"
	"path/filepath"
	"testing"
)

// REQ-CFG-001: deploy.region loads into DeployConfig.Region.
func TestLoad_DeployRegion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `services:
  - name: api
    image: nginx:latest
    deploy:
      region: us-west1
      replicas: 2
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if got := cfg.Services[0].Deploy.Region; got != "us-west1" {
		t.Errorf("Deploy.Region = %q, want us-west1", got)
	}
}

// REQ-CFG-002: an unknown region name still passes offline validation (no API call).
func TestValidate_RegionNotChecked(t *testing.T) {
	cfg := &Config{Services: []ServiceConfig{{
		Name:   "api",
		Image:  "nginx:latest",
		Deploy: DeployConfig{Region: "totally-made-up-region"},
	}}}
	if err := Validate(cfg); err != nil {
		t.Errorf("offline Validate should not reject region names, got %v", err)
	}
}

// REQ-CFG-003: deploy.region is NOT $env-expanded (kept literal).
func TestExpand_RegionLiteral(t *testing.T) {
	t.Setenv("MY_REGION", "us-west1")
	svc := &ServiceConfig{
		Name:   "api",
		Image:  "nginx:latest",
		Deploy: DeployConfig{Region: "$env(MY_REGION)"},
	}
	if errs := ExpandServiceConfigEnvRefs(svc); len(errs) != 0 {
		t.Fatalf("unexpected expansion errors: %v", errs)
	}
	if svc.Deploy.Region != "$env(MY_REGION)" {
		t.Errorf("region should be literal, got %q", svc.Deploy.Region)
	}
}

// REQ-CFG-004: legacy config loads with Region unset.
func TestLoad_LegacyRegionUnset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "legacy.yaml")
	content := `service:
  name: postgres
  image: postgres:16
deploy:
  numReplicas: 2
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Services[0].Deploy.Region != "" {
		t.Errorf("legacy Region should be empty, got %q", cfg.Services[0].Deploy.Region)
	}
}
