//go:build e2e

package harness

import (
	crand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// UniqueName returns a collision-resistant resource name for e2e fixtures.
func UniqueName() string {
	b := make([]byte, 4)
	crand.Read(b)
	return fmt.Sprintf("e2e-%d-%s", time.Now().Unix(), hex.EncodeToString(b))
}

// WaitForProject polls `describe project` until the API confirms the project
// is queryable. Retries up to 10 times with 2-second delays.
func WaitForProject(e *Env, name string) error {
	for i := 0; i < 10; i++ {
		r := e.Run("describe", "project", name)
		if r.ExitCode == 0 {
			e.T.Logf("Project %s confirmed queryable after %d poll(s)", name, i+1)
			return nil
		}
		e.T.Logf("Waiting for project %s to propagate (attempt %d/10)...", name, i+1)
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("project %s not queryable after 10 attempts", name)
}

// WaitForEnvironment polls `get environments` until the environment appears in
// the project. Retries up to 10 times with 2-second delays.
func WaitForEnvironment(e *Env, envName string) error {
	for i := 0; i < 10; i++ {
		r := e.Run("get", "environments", "-p", e.ProjectName)
		if r.ExitCode == 0 && strings.Contains(r.Stdout, envName) {
			e.T.Logf("Environment %s confirmed queryable after %d poll(s)", envName, i+1)
			return nil
		}
		e.T.Logf("Waiting for environment %s to propagate (attempt %d/10)...", envName, i+1)
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("environment %s not queryable after 10 attempts", envName)
}

// SetupProject creates a fresh project (or reuses E2E_PROJECT) using the given
// token. The token must be able to create projects (account or workspace scope).
func SetupProject(t *testing.T, token string) *Env {
	t.Helper()
	RequireBinary(t)

	name := os.Getenv("E2E_PROJECT")
	if name == "" {
		name = UniqueName()
	}

	e := &Env{
		T:           t,
		Token:       token,
		ProjectName: name,
		EnvName:     "staging",
		ServiceName: "web",
		ServiceImg:  "nginx:1.25-alpine",
	}

	// Create project
	r := e.Run("create", "project", e.ProjectName)
	if r.ExitCode != 0 {
		t.Fatalf("failed to create project %s: %s", e.ProjectName, r.Stderr)
	}
	e.hasProject = true

	// Wait for Railway API propagation before proceeding
	if err := WaitForProject(e, e.ProjectName); err != nil {
		t.Fatalf("project propagation failed: %v", err)
	}

	t.Cleanup(func() {
		if os.Getenv("E2E_KEEP") == "1" && t.Failed() {
			t.Logf("E2E_KEEP=1: leaving project %s for debugging", e.ProjectName)
			return
		}
		e.Teardown()
	})

	t.Logf("Created project: %s", e.ProjectName)
	return e
}

// SetupEnvironment creates project + custom environment using the given token.
func SetupEnvironment(t *testing.T, token string) *Env {
	t.Helper()
	e := SetupProject(t, token)

	r := e.Run("create", "environment", e.EnvName, "-p", e.ProjectName)
	if r.ExitCode != 0 {
		t.Fatalf("failed to create environment %s: %s", e.EnvName, r.Stderr)
	}
	e.hasEnv = true

	// Wait for Railway API propagation before proceeding
	if err := WaitForEnvironment(e, e.EnvName); err != nil {
		t.Fatalf("environment propagation failed: %v", err)
	}

	t.Logf("Created environment: %s", e.EnvName)
	return e
}

// SetupService creates project + environment + service using the given token.
func SetupService(t *testing.T, token string) *Env {
	t.Helper()
	e := SetupEnvironment(t, token)

	r := e.Run("create", "service", e.ServiceName,
		"--image", e.ServiceImg,
		"-p", e.ProjectName, "-e", e.EnvName)
	if r.ExitCode != 0 {
		t.Fatalf("failed to create service %s: %s", e.ServiceName, r.Stderr)
	}
	e.hasService = true
	t.Logf("Created service: %s (waiting for deployment...)", e.ServiceName)
	time.Sleep(3 * time.Second) // Let initial deployment start
	return e
}

// Teardown cleans up all created resources in reverse order.
func (e *Env) Teardown() {
	e.T.Helper()
	e.T.Logf("Cleaning up project %s...", e.ProjectName)

	// Delete services in all environments
	for _, env := range []string{e.EnvName, "production"} {
		r := e.Run("get", "services", "-p", e.ProjectName, "-e", env, "-o", "json")
		var svcs []map[string]interface{}
		if json.Unmarshal([]byte(r.Stdout), &svcs) == nil {
			for _, svc := range svcs {
				if name, ok := svc["name"].(string); ok {
					e.T.Logf("  Deleting service %s in %s", name, env)
					e.Run("delete", "service", name, "-p", e.ProjectName, "-e", env, "--yes")
				}
			}
		}
	}

	// Delete volumes in all environments
	for _, env := range []string{e.EnvName, "production"} {
		r := e.Run("get", "volumes", "-p", e.ProjectName, "-e", env, "-o", "json")
		var vols []map[string]interface{}
		if json.Unmarshal([]byte(r.Stdout), &vols) == nil {
			for _, vol := range vols {
				if name, ok := vol["name"].(string); ok {
					e.T.Logf("  Deleting volume %s in %s", name, env)
					e.Run("delete", "volume", name, "-p", e.ProjectName, "-e", env, "--yes")
				}
			}
		}
	}

	time.Sleep(2 * time.Second)

	// Delete project
	e.T.Logf("  Deleting project %s", e.ProjectName)
	r := e.Run("delete", "project", e.ProjectName, "--yes")
	if r.ExitCode == 0 {
		e.T.Log("  Project deleted")
	} else {
		e.T.Logf("  Could not delete project: %s", r.Stderr)
	}
}
