package cmd

import (
	"github.com/spf13/cobra"
)

// unprotectCmd is the parent command for clearing delete protection.
var unprotectCmd = &cobra.Command{
	Use:   "unprotect",
	Short: "Clear delete protection from resources",
	Long: `Clear delete protection so resources can be deleted again.

Available resources:
  environment    Remove delete protection from an environment`,
}

// unprotectEnvironmentCmd sets DELETE_PROTECTION=false on an environment.
var unprotectEnvironmentCmd = &cobra.Command{
	Use:   "environment NAME",
	Short: "Remove delete protection from an environment",
	Long: `Remove delete protection from an environment by setting its DELETE_PROTECTION
shared variable to a falsy value. Once unprotected, the environment (and its
project) can be deleted again.

This writes an environment-level (shared, serviceless) variable, so it requires
an account or workspace token — a project token cannot write shared variables.

The operation is idempotent: unprotecting an already-unprotected environment is
a no-op that preserves every other shared variable.`,
	Aliases: []string{"env"},
	Args:    cobra.ExactArgs(1),
	Example: `  railctl unprotect environment production -p my-app
  railctl unprotect env production -p my-app -o json`,
	RunE: runUnprotectEnvironment,
}

func init() {
	unprotectCmd.AddCommand(unprotectEnvironmentCmd)
	rootCmd.AddCommand(unprotectCmd)
}

func runUnprotectEnvironment(cmd *cobra.Command, args []string) error {
	return toggleDeleteProtection(cmd, args[0], false)
}
