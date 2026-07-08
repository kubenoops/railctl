//go:build e2e

package workspace

import (
	"testing"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// TestProjects exercises project CRUD operations.
// Setup: none (just needs RAILWAY_WORKSPACE_TOKEN)
//
//	go test -tags e2e -v -run TestProjects ./tests/e2e/workspace/...
func TestProjects(t *testing.T) {
	env := harness.SetupProject(t, token)

	t.Run("get_table", func(t *testing.T) {
		r := env.RunOK(t, "get", "projects")
		harness.AssertContains(t, r.Stdout, env.ProjectName)
	})

	t.Run("get_json", func(t *testing.T) {
		r := env.RunOK(t, "get", "projects", "-o", "json")
		harness.AssertValidJSON(t, r.Stdout)
		harness.AssertContains(t, r.Stdout, env.ProjectName)
	})

	t.Run("get_yaml", func(t *testing.T) {
		r := env.RunOK(t, "get", "projects", "-o", "yaml")
		harness.AssertValidYAML(t, r.Stdout)
		harness.AssertContains(t, r.Stdout, env.ProjectName)
	})

	t.Run("get_wide", func(t *testing.T) {
		r := env.RunOK(t, "get", "projects", "-o", "wide")
		harness.AssertContains(t, r.Stdout, env.ProjectName)
	})

	t.Run("describe_table", func(t *testing.T) {
		r := env.RunOK(t, "describe", "project", env.ProjectName)
		harness.AssertContains(t, r.Stdout, env.ProjectName)
	})

	t.Run("describe_json", func(t *testing.T) {
		r := env.RunOK(t, "describe", "project", env.ProjectName, "-o", "json")
		harness.AssertValidJSON(t, r.Stdout)
	})

	t.Run("describe_yaml", func(t *testing.T) {
		r := env.RunOK(t, "describe", "project", env.ProjectName, "-o", "yaml")
		harness.AssertValidYAML(t, r.Stdout)
	})

	t.Run("describe_nonexistent", func(t *testing.T) {
		r := env.RunFail(t, "describe", "project", "nonexistent-project-xyz-999")
		// Not-found errors must list the names that DO exist (error-taxonomy
		// class 3) — the fixture project is enumerable with this token.
		harness.AssertContains(t, r.Stderr+r.Stdout, "not found")
		harness.AssertContains(t, r.Stderr+r.Stdout, "available:")
	})

	t.Run("get_bad_token", func(t *testing.T) {
		env.RunFail(t, "get", "projects", "--token", "invalid-token-12345")
	})

	t.Run("substring_resolution", func(t *testing.T) {
		// Use the full project name minus the last 4 characters.
		// This is unique enough to match only our project, unlike a short
		// prefix (e.g. "e2e-177946") which can collide with concurrent test runs.
		prefix := env.ProjectName
		if len(prefix) > 4 {
			prefix = prefix[:len(prefix)-4]
		}
		r := env.RunOK(t, "describe", "project", prefix)
		harness.AssertContains(t, r.Stdout, env.ProjectName)
	})
}
