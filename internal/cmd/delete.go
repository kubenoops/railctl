package cmd

import (
	"github.com/spf13/cobra"
)

// deleteCmd represents the delete command group.
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

IMPORTANT: Deletion is permanent and cannot be undone.`,
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}
