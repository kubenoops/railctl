package cmd

import (
	"github.com/spf13/cobra"
)

// updateCmd represents the update command group.
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update resources",
	Long: `Update resources in Railway.

Available resources:
  service    Update a service (e.g., change image)`,
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
