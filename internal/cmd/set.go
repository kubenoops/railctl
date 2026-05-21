package cmd

import (
	"github.com/spf13/cobra"
)

// setCmd represents the "set" command group.
var setCmd = &cobra.Command{
	Use:   "set <resource>",
	Short: "Set or update resources",
	Long: `Set or update Railway resources.

Available resources:
  variable      Set environment variables for a service (requires -p, -e, -s)

Examples:
  # Set a variable
  railctl set variable DATABASE_URL=postgres://... -p my-app -e production -s web

  # Set variable without triggering deployment
  railctl set variable FEATURE=enabled --skip-deployment -p my-app -e production -s web`,
}

func init() {
	rootCmd.AddCommand(setCmd)
}
