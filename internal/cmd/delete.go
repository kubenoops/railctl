package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// deleteCmd represents the delete command group. It doubles as the
// declarative-deletion entry point when -f/--file is given (the teardown
// counterpart of `apply -f` / `diff -f`); subcommand dispatch always wins
// when a subcommand name is present, so `delete service …` etc. are
// unaffected.
var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete resources",
	Long: `Delete resources in Railway.

Available resources:
  project        Delete a project (requires --yes or confirmation)
  environment    Delete an environment (requires --yes or confirmation)
  service        Delete a service (requires --yes or confirmation)
  deployment     Remove a deployment (requires --yes or confirmation, rollback if latest)
  variable       Delete an environment variable (requires --yes or confirmation)

Declarative mode (-f/--file):
  Delete the services declared in a YAML config file or directory — the
  teardown counterpart of 'apply -f'. Services are deleted in reverse
  manifest order, then the volumes the manifest declares. Only what the
  manifest declares is deleted; the environment and project are never
  touched.

IMPORTANT: Deletion is permanent and cannot be undone.`,
	Example: `  # Subcommand mode
  railctl delete service api -p my-app -e production --yes

  # Declarative mode: tear down everything a manifest declares
  railctl delete -f stack.yaml
  railctl delete -f stack.yaml --yes`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if deleteFile != "" {
			if len(args) > 0 {
				return fmt.Errorf("unexpected arguments with -f/--file: %v", args)
			}
			return runDeleteFile(cmd, args)
		}
		// No -f and no subcommand: keep the pre-existing behavior (help).
		// A stray argument is a typo'd subcommand — error like cobra would.
		if len(args) > 0 {
			return fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath())
		}
		return cmd.Help()
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}
