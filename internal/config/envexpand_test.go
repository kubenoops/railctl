package config

import (
	"strings"
	"testing"
)

func TestExpandEnvRefs_Simple(t *testing.T) {
	t.Setenv("FOO", "bar")

	result, err := ExpandEnvRefs("$env(FOO)")
	if err != nil {
		t.Fatalf("ExpandEnvRefs() error: %v", err)
	}
	if result != "bar" {
		t.Errorf("result = %q, want %q", result, "bar")
	}
}

func TestExpandEnvRefs_Multiple(t *testing.T) {
	t.Setenv("HOST", "localhost")
	t.Setenv("PORT", "5432")

	result, err := ExpandEnvRefs("postgres://$env(HOST):$env(PORT)/db")
	if err != nil {
		t.Fatalf("ExpandEnvRefs() error: %v", err)
	}
	if result != "postgres://localhost:5432/db" {
		t.Errorf("result = %q, want %q", result, "postgres://localhost:5432/db")
	}
}

func TestExpandEnvRefs_Missing(t *testing.T) {
	// Ensure the variable is not set.
	t.Setenv("MISSING_VAR_TEST", "")

	// Unset it after Setenv to guarantee LookupEnv returns false.
	// t.Setenv doesn't support unsetting, so we test with a truly novel name.
	_, err := ExpandEnvRefs("$env(DEFINITELY_NOT_SET_12345)")
	if err == nil {
		t.Fatal("expected error for missing env var, got nil")
	}
	if !strings.Contains(err.Error(), "DEFINITELY_NOT_SET_12345") {
		t.Errorf("error = %q, want it to contain var name", err.Error())
	}
}

func TestExpandEnvRefs_RailwayRef(t *testing.T) {
	input := "${{service.DATABASE_URL}}"
	result, err := ExpandEnvRefs(input)
	if err != nil {
		t.Fatalf("ExpandEnvRefs() error: %v", err)
	}
	if result != input {
		t.Errorf("result = %q, want %q (Railway refs should be preserved)", result, input)
	}
}

func TestExpandEnvRefs_Mixed(t *testing.T) {
	t.Setenv("MY_HOST", "db.example.com")

	input := "host=$env(MY_HOST) ref=${{service.SECRET}}"
	result, err := ExpandEnvRefs(input)
	if err != nil {
		t.Fatalf("ExpandEnvRefs() error: %v", err)
	}

	expected := "host=db.example.com ref=${{service.SECRET}}"
	if result != expected {
		t.Errorf("result = %q, want %q", result, expected)
	}
}

func TestExpandEnvRefs_Empty(t *testing.T) {
	result, err := ExpandEnvRefs("")
	if err != nil {
		t.Fatalf("ExpandEnvRefs() error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty string", result)
	}
}

func TestExpandServiceConfigEnvRefs(t *testing.T) {
	t.Setenv("IMG_TAG", "v1.2.3")
	t.Setenv("REG_USER", "admin")
	t.Setenv("REG_PASS", "secret")
	t.Setenv("DB_URL", "postgres://localhost/db")

	svc := &ServiceConfig{
		Image: "myapp:$env(IMG_TAG)",
		Deploy: DeployConfig{
			StartCommand: "go run main.go",
		},
		Variables: map[string]string{
			"DATABASE_URL": "$env(DB_URL)",
			"STATIC":       "no-expansion-needed",
		},
		Registry: RegistryConfig{
			Username: "$env(REG_USER)",
			Password: "$env(REG_PASS)",
		},
	}

	errs := ExpandServiceConfigEnvRefs(svc)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	if svc.Image != "myapp:v1.2.3" {
		t.Errorf("image = %q, want %q", svc.Image, "myapp:v1.2.3")
	}
	if svc.Variables["DATABASE_URL"] != "postgres://localhost/db" {
		t.Errorf("variables[DATABASE_URL] = %q, want %q", svc.Variables["DATABASE_URL"], "postgres://localhost/db")
	}
	if svc.Variables["STATIC"] != "no-expansion-needed" {
		t.Errorf("variables[STATIC] = %q, want %q", svc.Variables["STATIC"], "no-expansion-needed")
	}
	if svc.Registry.Username != "admin" {
		t.Errorf("registry.username = %q, want %q", svc.Registry.Username, "admin")
	}
	if svc.Registry.Password != "secret" {
		t.Errorf("registry.password = %q, want %q", svc.Registry.Password, "secret")
	}
}

func TestExpandConfigEnvRefs(t *testing.T) {
	t.Setenv("GOOD_VAR", "works")

	cfg := &Config{
		Services: []ServiceConfig{
			{
				Name:  "svc1",
				Image: "$env(GOOD_VAR)",
			},
			{
				Name:  "svc2",
				Image: "$env(DOES_NOT_EXIST_XYZ)",
			},
		},
	}

	err := ExpandConfigEnvRefs(cfg)
	if err == nil {
		t.Fatal("expected error for missing env var, got nil")
	}
	if !strings.Contains(err.Error(), "DOES_NOT_EXIST_XYZ") {
		t.Errorf("error = %q, want it to mention the missing var", err.Error())
	}

	// The first service should still have been expanded.
	if cfg.Services[0].Image != "works" {
		t.Errorf("services[0].image = %q, want %q", cfg.Services[0].Image, "works")
	}
}
