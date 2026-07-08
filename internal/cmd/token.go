package cmd

import (
	"github.com/spf13/cobra"
)

// tokenCmd is the parent for project token management.
var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Manage project/environment access tokens",
	Long: `Create, list, and delete Railway project tokens.

A project token is scoped to a single project and environment — a much smaller
blast radius than an account or workspace token. Minting a token requires an
account or workspace token; a project-scoped token cannot create tokens.`,
	Example: `  railctl token create ci --project my-app --environment production
  railctl token list --project my-app
  railctl token delete <id> --project my-app`,
}

func init() {
	rootCmd.AddCommand(tokenCmd)
}
