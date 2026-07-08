package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/kubenoops/railctl/internal/cmdutil"
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

	// Verify deployment belongs to this service by listing deployments
	deployments, err := client.ListDeployments(ctx.Project.ID, ctx.Environment.ID, serviceID, 50)
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
