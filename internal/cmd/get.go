package cmd

import (
	"github.com/spf13/cobra"
)

// getCmd represents the "get" command group.
var getCmd = &cobra.Command{
	Use:   "get <resource>",
	Short: "List resources",
	Long: `List Railway resources in table, wide, JSON, or YAML format.

Available resources:
  projects      List all projects in the workspace
  environments  List environments in a project (requires -p)
  services      List services in a project/environment (requires -p, -e)
  variables     List variables for a service (requires -p, -e, -s)
  deployments   List deployments for a service (requires -p, -e, -s)
  domains       List domains for a service (requires -p, -e, -s)

Examples:
  railctl get projects
  railctl get projects -o wide
  railctl get environments -p my-app
  railctl get services -p my-app -e production -o json`,
}

func init() {
	rootCmd.AddCommand(getCmd)
}
