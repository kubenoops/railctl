package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
	return path
}

func TestLoad_DeleteProtectionTrue(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "c.yaml", `
project: my-app
environment: production
deleteProtection: true
services:
  - name: web
    image: nginx:latest
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.DeleteProtection == nil {
		t.Fatal("expected DeleteProtection non-nil for explicit true")
	}
	if !*cfg.DeleteProtection {
		t.Errorf("expected DeleteProtection true, got %v", *cfg.DeleteProtection)
	}
}

func TestLoad_DeleteProtectionFalse(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "c.yaml", `
project: my-app
environment: production
deleteProtection: false
services:
  - name: web
    image: nginx:latest
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.DeleteProtection == nil {
		t.Fatal("expected DeleteProtection non-nil for explicit false (distinct from omitted)")
	}
	if *cfg.DeleteProtection {
		t.Errorf("expected DeleteProtection false, got %v", *cfg.DeleteProtection)
	}
}

func TestLoad_DeleteProtectionOmittedIsNil(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "c.yaml", `
project: my-app
environment: production
services:
  - name: web
    image: nginx:latest
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.DeleteProtection != nil {
		t.Errorf("expected DeleteProtection nil when omitted, got %v", *cfg.DeleteProtection)
	}
}

func TestLoadDir_DeleteProtectionMerge(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "a.yaml", `
project: my-app
environment: production
deleteProtection: true
services:
  - name: web
    image: nginx:latest
`)
	// Second file omits deleteProtection — must not override the explicit true.
	writeConfig(t, dir, "b.yaml", `
services:
  - name: worker
    image: busybox:latest
`)
	cfg, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir() error: %v", err)
	}
	if cfg.DeleteProtection == nil || !*cfg.DeleteProtection {
		t.Errorf("expected merged DeleteProtection true, got %v", cfg.DeleteProtection)
	}
}

func TestLoadDir_DeleteProtectionConflict(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "a.yaml", `
deleteProtection: true
services:
  - name: web
    image: nginx:latest
`)
	writeConfig(t, dir, "b.yaml", `
deleteProtection: false
services:
  - name: worker
    image: busybox:latest
`)
	_, err := LoadDir(dir)
	if err == nil {
		t.Fatal("expected conflict error for disagreeing deleteProtection")
	}
}
