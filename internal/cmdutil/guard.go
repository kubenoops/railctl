package cmdutil

import (
	"fmt"

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
