package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/spf13/cobra"
)

var (
	deleteVariableYes bool
)

var deleteVariableCmd = &cobra.Command{
	Use:     "variable KEY",
	Aliases: []string{"var", "v"},
	Short:   "Delete an environment variable from a service",
	Long: `Delete an environment variable from a service in an environment.

This command removes a single variable by its key. You will be prompted for
confirmation unless the --yes flag is provided.

Required flags can be provided via environment variables:
  --project, -p     or RAILCTL_PROJECT
  --environment, -e or RAILCTL_ENVIRONMENT
  --service, -s     or RAILCTL_SERVICE`,
	Example: `  # Delete a variable (with confirmation prompt)
  railctl delete variable DATABASE_URL -p myproject -e production -s web

  # Delete a variable without confirmation
  railctl delete variable DATABASE_URL --yes -p myproject -e production -s web

  # Using environment variables for context
  export RAILCTL_PROJECT=myproject
  export RAILCTL_ENVIRONMENT=production
  export RAILCTL_SERVICE=web
  railctl delete variable OLD_API_KEY`,
	Args: cobra.ExactArgs(1),
	RunE: runDeleteVariable,
}

func init() {
	deleteCmd.AddCommand(deleteVariableCmd)

	deleteVariableCmd.Flags().BoolVarP(&deleteVariableYes, "yes", "y", false, "Skip confirmation prompt")
}

func runDeleteVariable(cmd *cobra.Command, args []string) error {
	variableKey := args[0]

	token, err := getToken()
	if err != nil {
		return err
	}

	client := newAPIClient(token)

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

	// Confirmation prompt
	if !deleteVariableYes {
		fmt.Printf("Are you sure you want to delete variable '%s' from service '%s' in environment '%s'? (y/N): ",
			variableKey, ctx.Service.Name, ctx.Environment.Name)

		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Deletion cancelled")
			return nil
		}
	}

	// Delete variable
	err = client.DeleteVariable(ctx.Project.ID, ctx.Environment.ID, ctx.Service.ID, variableKey)
	if err != nil {
		return fmt.Errorf("failed to delete variable: %w", err)
	}

	fmt.Printf("Variable '%s' deleted successfully from service '%s' in environment '%s'\n",
		variableKey, ctx.Service.Name, ctx.Environment.Name)

	return nil
}
