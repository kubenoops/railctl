package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/kubenoops/railctl/internal/resolver"
	"github.com/spf13/cobra"
)

var (
	deleteEnvironmentYes bool
)

// deleteEnvironmentCmd deletes an environment.
var deleteEnvironmentCmd = &cobra.Command{
	Use:     "environment NAME",
	Short:   "Delete an environment from a project",
	Long:    `Delete an environment by name. Requires confirmation unless --yes is specified.`,
	Aliases: []string{"env"},
	Args:    cobra.ExactArgs(1),
	Example: `  railctl delete environment staging -p my-app
  railctl delete env staging -p my-app --yes`,
	RunE: runDeleteEnvironment,
}

func init() {
	deleteEnvironmentCmd.Flags().BoolVar(&deleteEnvironmentYes, "yes", false, "Skip confirmation prompt")
	deleteCmd.AddCommand(deleteEnvironmentCmd)
}

func runDeleteEnvironment(cmd *cobra.Command, args []string) error {
	token, err := getToken()
	if err != nil {
		return err
	}

	projectName := getProject()
	if projectName == "" {
		return fmt.Errorf("project required: use -p flag or set RAILCTL_PROJECT")
	}

	name := args[0]
	client := newAPIClient(token)

	if err := cmdutil.RequireWorkspaceScope(client, "delete an environment"); err != nil {
		return err
	}

	// Resolve project name to ID
	projects, err := client.ListProjects()
	if err != nil {
		return fmt.Errorf("failed to list projects: %w", err)
	}

	project, err := resolver.ResolveProject(projects, projectName)
	if err != nil {
		return err
	}

	// Get environments and resolve the name
	environments, err := client.ListEnvironments(project.ID)
	if err != nil {
		return fmt.Errorf("failed to list environments: %w", err)
	}

	env, err := resolver.ResolveEnvironment(environments, name)
	if err != nil {
		return err
	}

	// Prevent deleting the last environment
	if len(environments) == 1 {
		return fmt.Errorf("cannot delete the last environment in project %q", project.Name)
	}

	// Confirm deletion
	if !deleteEnvironmentYes {
		fmt.Printf("Are you sure you want to delete environment %q from project %q? This action cannot be undone.\n",
			env.Name, project.Name)
		fmt.Print("Type the environment name to confirm: ")

		reader := bufio.NewReader(os.Stdin)
		confirmation, _ := reader.ReadString('\n')
		confirmation = strings.TrimSpace(confirmation)

		if confirmation != env.Name {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// Delete the environment
	if err := client.DeleteEnvironment(env.ID); err != nil {
		return fmt.Errorf("failed to delete environment: %w", err)
	}

	fmt.Printf("Deleted environment %q from project %q\n", env.Name, project.Name)
	return nil
}
