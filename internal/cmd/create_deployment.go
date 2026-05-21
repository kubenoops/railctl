package cmd

import (
	"fmt"

	"github.com/kubenoops/railctl/internal/resolver"
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

	projectFlag := getProject()
	if projectFlag == "" {
		return fmt.Errorf("-p/--project is required. Use -p flag or set RAILCTL_PROJECT")
	}

	envFlag := getEnvironment()
	if envFlag == "" {
		return fmt.Errorf("-e/--environment is required. Use -e flag or set RAILCTL_ENVIRONMENT")
	}

	serviceFlag := getService()
	if serviceFlag == "" {
		return fmt.Errorf("-s/--service is required. Use -s flag or set RAILCTL_SERVICE")
	}

	client := newAPIClient(token)

	// Resolve project
	projects, err := client.ListProjects()
	if err != nil {
		return err
	}
	project, err := resolver.ResolveProject(projects, projectFlag)
	if err != nil {
		return fmt.Errorf("project '%s' not found", projectFlag)
	}

	// Resolve environment
	environments, err := client.ListEnvironments(project.ID)
	if err != nil {
		return err
	}
	env, err := resolver.ResolveEnvironment(environments, envFlag)
	if err != nil {
		return fmt.Errorf("environment '%s' not found in project", envFlag)
	}

	// Resolve service
	services, err := client.ListServices(project.ID, env.ID)
	if err != nil {
		return err
	}
	svcResources := make([]resolver.Resource, len(services))
	var serviceName string
	for i, s := range services {
		svcResources[i] = resolver.Resource{ID: s.ID, Name: s.Name}
	}
	serviceID, err := resolver.Resolve(serviceFlag, svcResources)
	if err != nil {
		return fmt.Errorf("service '%s' not found in environment", serviceFlag)
	}
	for _, s := range services {
		if s.ID == serviceID {
			serviceName = s.Name
			break
		}
	}

	// Trigger deployment
	deploymentID, err := client.DeployServiceInstance(serviceID, env.ID)
	if err != nil {
		return fmt.Errorf("failed to trigger deployment: %w", err)
	}

	fmt.Printf("Deployment triggered for service '%s'\n", serviceName)
	fmt.Printf("Deployment ID: %s\n", deploymentID)

	if createDeplAwait {
		return awaitDeployment(client, project.ID, env.ID, serviceID, deploymentID, serviceName, createDeplTimeout)
	}
	return nil
}
