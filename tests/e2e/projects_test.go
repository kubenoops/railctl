//go:build e2e

package e2e

import "testing"

// TestProjects exercises project CRUD operations.
// Setup: none (just needs RAILWAY_TOKEN)
//
//	go test -tags e2e -v -run TestProjects ./tests/e2e/...
func TestProjects(t *testing.T) {
	env := SetupProject(t)

	t.Run("get_table", func(t *testing.T) {
		r := env.RunOK(t, "get", "projects")
		AssertContains(t, r.Stdout, env.ProjectName)
	})

	t.Run("get_json", func(t *testing.T) {
		r := env.RunOK(t, "get", "projects", "-o", "json")
		AssertValidJSON(t, r.Stdout)
		AssertContains(t, r.Stdout, env.ProjectName)
	})

	t.Run("get_yaml", func(t *testing.T) {
		r := env.RunOK(t, "get", "projects", "-o", "yaml")
		AssertValidYAML(t, r.Stdout)
		AssertContains(t, r.Stdout, env.ProjectName)
	})

	t.Run("get_wide", func(t *testing.T) {
		r := env.RunOK(t, "get", "projects", "-o", "wide")
		AssertContains(t, r.Stdout, env.ProjectName)
	})

	t.Run("describe_table", func(t *testing.T) {
		r := env.RunOK(t, "describe", "project", env.ProjectName)
		AssertContains(t, r.Stdout, env.ProjectName)
	})

	t.Run("describe_json", func(t *testing.T) {
		r := env.RunOK(t, "describe", "project", env.ProjectName, "-o", "json")
		AssertValidJSON(t, r.Stdout)
	})

	t.Run("describe_yaml", func(t *testing.T) {
		r := env.RunOK(t, "describe", "project", env.ProjectName, "-o", "yaml")
		AssertValidYAML(t, r.Stdout)
	})

	t.Run("describe_nonexistent", func(t *testing.T) {
		env.RunFail(t, "describe", "project", "nonexistent-project-xyz-999")
	})

	t.Run("get_bad_token", func(t *testing.T) {
		env.RunFail(t, "get", "projects", "--token", "invalid-token-12345")
	})

	t.Run("substring_resolution", func(t *testing.T) {
		prefix := env.ProjectName
		if len(prefix) > 10 {
			prefix = prefix[:10]
		}
		r := env.RunOK(t, "describe", "project", prefix)
		AssertContains(t, r.Stdout, env.ProjectName)
	})
}
