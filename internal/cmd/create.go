package cmd

import (
	"github.com/spf13/cobra"
)

// createCmd represents the create command group.
var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create resources",
	Long: `Create resources in Railway.

Available resources:
  project        Create a new project
  environment    Create a new environment in a project
  service        Create a new service from a Docker image
  deployment     Trigger a new deployment (redeploy) for a service
  domain         Create a custom domain on a service`,
}

func init() {
	rootCmd.AddCommand(createCmd)
}
