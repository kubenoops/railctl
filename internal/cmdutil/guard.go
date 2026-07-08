package cmdutil

import (
	"fmt"
	"strings"

	"github.com/kubenoops/railctl/internal/api"
)

// RequireWorkspaceScope fails fast when the client's token is project-scoped.
// op is a short imperative description used in the error, e.g. "list projects".
//
// Workspace-level commands (listing projects, creating or deleting
// projects/environments) cannot work with a project token, which is pinned to
// a single project and environment. Calling this guard immediately after
// client construction — before any other API call — turns the opaque API
// "Not Authorized" failure into an actionable error.
func RequireWorkspaceScope(client api.APIClient, op string) error {
	isProjectToken, err := client.IsProjectToken()
	if err != nil {
		return fmt.Errorf("failed to check token type: %w", err)
	}
	if isProjectToken {
		return fmt.Errorf("cannot %s with a project token — it is scoped to a single project and environment; use an account or workspace token", op)
	}
	return nil
}

// GuardServiceCreationScope fails fast when creating a service would leak
// instances into environments the token cannot clean up: Railway creates
// service instances in ALL environments (see docs/railway-service-creation-behavior.md),
// and a project token cannot delete instances outside its scoped environment.
//
// Non-project tokens pass unconditionally — they can run the post-create
// cleanup themselves. A project token passes only when the project has no
// environment other than the token's own (single-environment project: nothing
// to leak into).
func GuardServiceCreationScope(client api.APIClient, projectID, projectName, envID, envName string) error {
	isProjectToken, err := client.IsProjectToken()
	if err != nil {
		return fmt.Errorf("failed to check token type: %w", err)
	}
	if !isProjectToken {
		return nil
	}

	// Project tokens ARE allowed to list their project's environments.
	environments, err := client.ListEnvironments(projectID)
	if err != nil {
		return fmt.Errorf("failed to list environments of project '%s': %w", projectName, err)
	}

	var otherEnvs []string
	for _, env := range environments {
		if env.ID != envID {
			otherEnvs = append(otherEnvs, env.Name)
		}
	}
	if len(otherEnvs) == 0 {
		return nil
	}

	return fmt.Errorf(
		"cannot create a service with a project token in a multi-environment project — Railway creates service instances in ALL environments and this token (scoped to '%s') cannot remove the instances it would leak into: %s; use a workspace or account token, or run in a single-environment project",
		envName, strings.Join(otherEnvs, ", "))
}
