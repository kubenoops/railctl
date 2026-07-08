//go:build e2e

package workspace

import (
	"fmt"
	"testing"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// TestDeleteProtection exercises the DELETE_PROTECTION guard: an environment
// carrying a truthy DELETE_PROTECTION shared (environment-level, serviceless)
// variable must refuse `delete environment` and block `delete project`, with
// no bypass — --yes only skips the prompt, not the protection.
//
// NOTE: shared variables cannot be set through the CLI — `set variable` and
// `delete variable` both require -s/--service and always target a service's
// variable scope, so there is no serviceless CLI write path. The test
// therefore sets/unsets the shared variable directly via the API client
// (the harness already talks to the API for token preflight).
//
//	go test -tags e2e -v -run TestDeleteProtection ./tests/e2e/workspace/...
func TestDeleteProtection(t *testing.T) {
	env := harness.SetupEnvironment(t, token)

	// Resolve project and environment IDs for the direct API calls.
	client := api.NewClient(token)
	projectID, environmentID, err := resolveIDs(client, env.ProjectName, env.EnvName)
	if err != nil {
		t.Fatalf("failed to resolve project/environment IDs: %v", err)
	}

	t.Run("protect", func(t *testing.T) {
		if err := client.SetSharedVariables(projectID, environmentID,
			map[string]string{"DELETE_PROTECTION": "true"}); err != nil {
			t.Fatalf("failed to set shared DELETE_PROTECTION=true: %v", err)
		}
	})

	t.Run("delete_environment_refused", func(t *testing.T) {
		r := env.RunFail(t, env.WithP("delete", "environment", env.EnvName, "--yes")...)
		harness.AssertContains(t, r.Stderr, "delete-protected")
		harness.AssertContains(t, r.Stderr, "DELETE_PROTECTION")
	})

	t.Run("delete_project_refused_names_env", func(t *testing.T) {
		r := env.RunFail(t, "delete", "project", env.ProjectName, "--yes")
		harness.AssertContains(t, r.Stderr, "delete-protected")
		harness.AssertContains(t, r.Stderr, env.EnvName)
	})

	// Unprotect by setting the variable to a falsy value. (The CLI cannot
	// delete a shared variable either — `delete variable` requires -s — so
	// falsifying via the API stands in for unsetting.)
	t.Run("unprotect", func(t *testing.T) {
		if err := client.SetSharedVariables(projectID, environmentID,
			map[string]string{"DELETE_PROTECTION": "false"}); err != nil {
			t.Fatalf("failed to set shared DELETE_PROTECTION=false: %v", err)
		}
	})

	t.Run("delete_environment_succeeds", func(t *testing.T) {
		env.RunOK(t, env.WithP("delete", "environment", env.EnvName, "--yes")...)
	})

	t.Run("verify_deleted", func(t *testing.T) {
		r := env.RunOK(t, env.WithP("get", "environments")...)
		harness.AssertNotContains(t, r.Stdout, env.EnvName)
	})
}

// resolveIDs maps project and environment names to their Railway IDs via the
// API, since the shared-variable calls need IDs rather than names.
func resolveIDs(client *api.Client, projectName, envName string) (projectID, environmentID string, err error) {
	projects, err := client.ListProjects()
	if err != nil {
		return "", "", fmt.Errorf("list projects: %w", err)
	}
	for _, p := range projects {
		if p.Name == projectName {
			projectID = p.ID
			break
		}
	}
	if projectID == "" {
		return "", "", fmt.Errorf("project %q not found", projectName)
	}

	envs, err := client.ListEnvironments(projectID)
	if err != nil {
		return "", "", fmt.Errorf("list environments: %w", err)
	}
	for _, e := range envs {
		if e.Name == envName {
			return projectID, e.ID, nil
		}
	}
	return "", "", fmt.Errorf("environment %q not found in project %q", envName, projectName)
}
