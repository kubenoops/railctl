package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/kubenoops/railctl/internal/resolver"
	"github.com/spf13/cobra"
)

var deleteDeploymentYes bool

var deleteDeploymentCmd = &cobra.Command{
	Use:     "deployment <id>",
	Aliases: []string{"deploy", "dep"},
	Short:   "Remove a deployment (rollback if latest)",
	Long: `Remove a deployment by ID.

If the removed deployment is the currently active (latest) deployment, Railway will
automatically promote the previous successful deployment - effectively performing
a rollback to the prior version.

This is the recommended way to rollback a service:
  1. Use 'railctl get deployments -s <service>' to find deployment IDs
  2. Remove the problematic deployment with 'railctl delete deployment <id>'
  3. Railway promotes the previous successful deployment

Note: Removing old/inactive deployments has no effect on the running service.`,
	Example: `  # Rollback by removing the latest deployment
  railctl delete deployment abc123-def456 -s api

  # Skip confirmation prompt
  railctl delete deployment abc123-def456 -s api --yes

  # First, find deployment IDs
  railctl get deployments -s api`,
	Args: cobra.ExactArgs(1),
	RunE: runDeleteDeployment,
}

func init() {
	deleteDeploymentCmd.Flags().BoolVarP(&deleteDeploymentYes, "yes", "y", false, "Skip confirmation prompt")
	deleteCmd.AddCommand(deleteDeploymentCmd)
}

func runDeleteDeployment(cmd *cobra.Command, args []string) error {
	deploymentID := args[0]

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

	// Resolve service to get its name for display
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

	// Verify deployment belongs to this service by listing deployments
	deployments, err := client.ListDeployments(project.ID, env.ID, serviceID, 50)
	if err != nil {
		return fmt.Errorf("failed to list deployments: %w", err)
	}

	found := false
	var deploymentStatus string
	isLatest := false
	for i, d := range deployments {
		if d.ID == deploymentID {
			found = true
			deploymentStatus = d.Status
			isLatest = i == 0
			break
		}
	}

	if !found {
		return fmt.Errorf("deployment '%s' not found for service '%s'", deploymentID, serviceName)
	}

	// Confirmation prompt
	if !deleteDeploymentYes {
		fmt.Printf("Remove deployment %s from service '%s'?\n", deploymentID, serviceName)
		fmt.Printf("  Status: %s\n", deploymentStatus)
		if isLatest {
			fmt.Printf("  ⚠️  This is the latest deployment. Railway will promote the previous successful one.\n")
		}
		fmt.Printf("Continue? (y/N): ")

		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	// Remove the deployment
	err = client.RemoveDeployment(deploymentID)
	if err != nil {
		return fmt.Errorf("failed to remove deployment: %w", err)
	}

	fmt.Printf("Deployment %s removed from service '%s'\n", deploymentID, serviceName)
	if isLatest {
		fmt.Println("Railway will promote the previous successful deployment.")
	}
	return nil
}
