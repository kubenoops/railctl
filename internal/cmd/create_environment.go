package cmd

import (
	"fmt"

	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/kubenoops/railctl/internal/resolver"
	"github.com/spf13/cobra"
)

// createEnvironmentCmd creates a new environment.
var createEnvironmentCmd = &cobra.Command{
	Use:     "environment NAME",
	Short:   "Create a new environment in a project",
	Long:    `Create a new environment in the specified project.`,
	Aliases: []string{"env"},
	Args:    cobra.ExactArgs(1),
	Example: `  railctl create environment staging -p my-app
  railctl create env development -p my-app`,
	RunE: runCreateEnvironment,
}

func init() {
	createCmd.AddCommand(createEnvironmentCmd)
}

func runCreateEnvironment(cmd *cobra.Command, args []string) error {
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

	if err := cmdutil.RequireWorkspaceScope(client, "create an environment"); err != nil {
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

	// Create the environment
	env, err := client.CreateEnvironment(project.ID, name)
	if err != nil {
		return fmt.Errorf("failed to create environment: %w", err)
	}

	fmt.Printf("Created environment %q in project %q (ID: %s)\n", env.Name, project.Name, env.ID)
	return nil
}
