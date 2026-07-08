package cmd

import (
	"fmt"

	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/spf13/cobra"
)

var createDeploymentCmd = &cobra.Command{
	Use:     "deployment",
	Aliases: []string{"deploy", "dep"},
	Short:   "Trigger a new deployment (redeploy) for a service",
	Long: `Trigger a new deployment for a service.

This command initiates a fresh deployment using the service's current configuration
and image. It is equivalent to clicking "Redeploy" in the Railway UI.

Use cases:
  - Restart a service with the same configuration
  - Force a fresh container pull for the current image
  - Re-run build and deploy steps for repo-based services

Note: To deploy with a NEW image, use 'railctl update service --image <image>' instead.`,
	Example: `  # Trigger a new deployment (redeploy)
  railctl create deployment -s api

  # With explicit project/environment
  railctl create deployment -p my-project -e production -s api`,
	RunE: runCreateDeployment,
}

var (
	createDeplAwait   bool
	createDeplTimeout int
)

func init() {
	createDeploymentCmd.Flags().BoolVar(&createDeplAwait, "await-completion", false, "Wait for the deployment to reach a terminal status before returning")
	createDeploymentCmd.Flags().IntVar(&createDeplTimeout, "timeout", 600, "Timeout in seconds for --await-completion (default: 600)")
	createCmd.AddCommand(createDeploymentCmd)
}

func runCreateDeployment(cmd *cobra.Command, args []string) error {
	token, err := getToken()
	if err != nil {
		return err
	}

	client := newAPIClient(token)

	// Resolve project, environment, and service. With a project token the
	// project and environment are derived from the token itself.
	ctx, err := cmdutil.ResolveContext(client, cmdutil.ResolveOpts{
		ProjectName:     getProject(),
		EnvironmentName: getEnvironment(),
		ServiceName:     getService(),
		NeedEnvironment: true,
		NeedService:     true,
	})
	if err != nil {
		return err
	}
	serviceID := ctx.Service.ID
	serviceName := ctx.Service.Name

	// Trigger deployment
	deploymentID, err := client.DeployServiceInstance(serviceID, ctx.Environment.ID)
	if err != nil {
		return fmt.Errorf("failed to trigger deployment: %w", err)
	}

	fmt.Printf("Deployment triggered for service '%s'\n", serviceName)
	fmt.Printf("Deployment ID: %s\n", deploymentID)

	if createDeplAwait {
		return awaitDeployment(client, ctx.Project.ID, ctx.Environment.ID, serviceID, deploymentID, serviceName, createDeplTimeout)
	}
	return nil
}
