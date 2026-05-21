package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// createProjectCmd creates a new project.
var createProjectCmd = &cobra.Command{
	Use:     "project NAME",
	Short:   "Create a new project",
	Long:    `Create a new project in the current workspace.`,
	Aliases: []string{"proj"},
	Args:    cobra.ExactArgs(1),
	Example: `  railctl create project my-app
  railctl create project "My New Project"`,
	RunE: runCreateProject,
}

func init() {
	createCmd.AddCommand(createProjectCmd)
}

func runCreateProject(cmd *cobra.Command, args []string) error {
	token, err := getToken()
	if err != nil {
		return err
	}

	name := args[0]

	client := newAPIClient(token)
	project, err := client.CreateProject(name)
	if err != nil {
		return fmt.Errorf("failed to create project: %w", err)
	}

	fmt.Printf("Created project %q (ID: %s)\n", project.Name, project.ID)

	if len(project.Environments) > 0 {
		fmt.Println("Default environments:")
		for _, env := range project.Environments {
			fmt.Printf("  - %s\n", env.Name)
		}
	}

	return nil
}
