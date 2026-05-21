package cmd

import (
	"github.com/spf13/cobra"
)

// describeCmd represents the "describe" command group.
var describeCmd = &cobra.Command{
	Use:   "describe <resource> <name>",
	Short: "Show detailed information about a resource",
	Long: `Display detailed information about a specific Railway resource.

Available resources:
  project      Show project details including environments and services
  environment  Show environment details (requires -p)
  service      Show service details including deployments (requires -p, -e)

Examples:
  railctl describe project my-app
  railctl describe project my-app -o json
  railctl describe environment production -p my-app
  railctl describe service api -p my-app -e production`,
}

func init() {
	rootCmd.AddCommand(describeCmd)
}
