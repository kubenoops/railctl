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
	deleteProjectYes bool
)

// deleteProjectCmd deletes a project.
var deleteProjectCmd = &cobra.Command{
	Use:     "project NAME",
	Short:   "Delete a project",
	Long:    `Delete a project by name. Requires confirmation unless --yes is specified.`,
	Aliases: []string{"proj"},
	Args:    cobra.ExactArgs(1),
	Example: `  railctl delete project my-app
  railctl delete project my-app --yes`,
	RunE: runDeleteProject,
}

func init() {
	deleteProjectCmd.Flags().BoolVar(&deleteProjectYes, "yes", false, "Skip confirmation prompt")
	deleteCmd.AddCommand(deleteProjectCmd)
}

func runDeleteProject(cmd *cobra.Command, args []string) error {
	token, err := getToken()
	if err != nil {
		return err
	}

	name := args[0]
	client := newAPIClient(token)

	if err := cmdutil.RequireWorkspaceScope(client, "delete a project"); err != nil {
		return err
	}

	// Resolve project name to ID
	projects, err := client.ListProjects()
	if err != nil {
		return fmt.Errorf("failed to list projects: %w", err)
	}

	project, err := resolver.ResolveProject(projects, name)
	if err != nil {
		return err
	}

	// Check if project has services and environments (deletion guard)
	if len(project.Services) > 0 {
		return fmt.Errorf("cannot delete project %q: has %d service(s). Delete services first or use Railway dashboard",
			project.Name, len(project.Services))
	}

	// Confirm deletion
	if !deleteProjectYes {
		fmt.Printf("Are you sure you want to delete project %q? This action cannot be undone.\n", project.Name)
		fmt.Print("Type the project name to confirm: ")

		reader := bufio.NewReader(os.Stdin)
		confirmation, _ := reader.ReadString('\n')
		confirmation = strings.TrimSpace(confirmation)

		if confirmation != project.Name {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// Delete the project
	if err := client.DeleteProject(project.ID); err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}

	fmt.Printf("Deleted project %q\n", project.Name)
	return nil
}
