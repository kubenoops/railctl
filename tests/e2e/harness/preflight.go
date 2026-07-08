//go:build e2e

package harness

import (
	"fmt"
	"os"

	"github.com/kubenoops/railctl/internal/api"
)

// TokenType classifies a Railway API token by its scope.
type TokenType int

const (
	// TokenAccount is a personal account token (spans all workspaces).
	TokenAccount TokenType = iota
	// TokenWorkspace is a workspace-scoped token.
	TokenWorkspace
	// TokenProject is a project-scoped token (implicit project + environment).
	TokenProject
)

// String returns the human-readable token scope name.
func (t TokenType) String() string {
	switch t {
	case TokenAccount:
		return "account"
	case TokenWorkspace:
		return "workspace"
	case TokenProject:
		return "project"
	default:
		return "unknown"
	}
}

// ClassifyToken probes the Railway API with the same detection railctl itself
// uses (internal/api lazy token-type detection) and reports the token's scope.
func ClassifyToken(token string) (TokenType, error) {
	c := api.NewClient(token)

	isProject, err := c.IsProjectToken()
	if err != nil {
		return TokenAccount, fmt.Errorf("token type detection failed: %w", err)
	}
	if isProject {
		return TokenProject, nil
	}

	isWorkspace, err := c.IsWorkspaceToken()
	if err != nil {
		return TokenAccount, fmt.Errorf("token type detection failed: %w", err)
	}
	if isWorkspace {
		return TokenWorkspace, nil
	}

	return TokenAccount, nil
}

// RequireToken reads envVar, classifies its value against the live Railway
// API, and exits the process with an actionable message if the variable is
// missing, the token fails detection, or its scope mismatches want.
// Intended for use in a test group's TestMain, before m.Run().
func RequireToken(envVar string, want TokenType) string {
	token := os.Getenv(envVar)
	if token == "" {
		fmt.Fprintf(os.Stderr,
			"e2e preflight: %s is not set — this test group needs a %s token.\n%s\n",
			envVar, want, obtainHint(want))
		os.Exit(1)
	}

	got, err := ClassifyToken(token)
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"e2e preflight: %s is set, but the token failed type detection: %v\n"+
				"Check that the token is valid and not revoked.\n",
			envVar, err)
		os.Exit(1)
	}

	if got != want {
		fmt.Fprintf(os.Stderr,
			"e2e preflight: %s is a %s token — this group needs a %s token.\n%s\n",
			envVar, got, want, obtainHint(want))
		os.Exit(1)
	}

	return token
}

// obtainHint tells the operator how to obtain a token of the wanted scope.
func obtainHint(want TokenType) string {
	switch want {
	case TokenProject:
		return "Hint: mint one with `railctl token create <name> -p <project> -e <environment>` (requires a workspace token), or create a project token in the Railway dashboard (Project Settings → Tokens)."
	case TokenWorkspace:
		return "Hint: create a workspace-scoped token in the Railway dashboard (Account Settings → Tokens, with a workspace selected)."
	default:
		return "Hint: create an account token in the Railway dashboard (Account Settings → Tokens, with no workspace selected)."
	}
}
