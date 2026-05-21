package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var updateDeploymentCmd = &cobra.Command{
	Use:   "deployment <deployment-id>",
	Short: "Update a deployment (e.g., set as active)",
	Long: `Update a deployment by reactivating it.

This command allows you to reactivate a previous deployment by its ID,
effectively rolling back to that deployment. This is useful for:
  - Rolling back to a known good deployment
  - Reactivating a specific deployment version
  - Reverting recent changes

The deployment will be redeployed with its original configuration.`,
	Example: `  # Reactivate a specific deployment
  railctl update deployment 712eee5e-5ad8-46a5-8a8b-4b20efc5bfa5 --set-active

  # List deployments to find the ID
  railctl get deployments -s my-service`,
	Args: cobra.ExactArgs(1),
	RunE: runUpdateDeployment,
}

var setActive bool

func init() {
	updateCmd.AddCommand(updateDeploymentCmd)
	updateDeploymentCmd.Flags().BoolVar(&setActive, "set-active", false, "Reactivate this deployment (redeploy it)")
}

func runUpdateDeployment(cmd *cobra.Command, args []string) error {
	deploymentID := args[0]

	if !setActive {
		return fmt.Errorf("--set-active flag is required. Use --set-active to reactivate this deployment")
	}

	token, err := getToken()
	if err != nil {
		return err
	}

	client := newAPIClient(token)

	// Redeploy the deployment
	err = client.RedeployDeployment(deploymentID)
	if err != nil {
		return fmt.Errorf("failed to reactivate deployment: %w", err)
	}

	fmt.Printf("Deployment '%s' has been reactivated successfully.\n", deploymentID)
	fmt.Println("The deployment is now being redeployed with its original configuration.")
	return nil
}
