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
	deleteServiceYes bool
)

var deleteServiceCmd = &cobra.Command{
	Use:     "service <name>",
	Aliases: []string{"svc"},
	Short:   "Delete a service",
	Args:    cobra.ExactArgs(1),
	Example: `  railctl delete service api -p my-project -e production
  railctl delete service api -p my-project -e production --yes`,
	RunE: runDeleteService,
}

func init() {
	deleteServiceCmd.Flags().BoolVar(&deleteServiceYes, "yes", false, "Skip confirmation prompt")
	deleteCmd.AddCommand(deleteServiceCmd)
}

func runDeleteService(cmd *cobra.Command, args []string) error {
	serviceName := args[0]

	token, err := getToken()
	if err != nil {
		return err
	}

	client := newAPIClient(token)

	ctx, err := cmdutil.ResolveContext(client, cmdutil.ResolveOpts{
		ProjectName:     getProject(),
		EnvironmentName: getEnvironment(),
		ServiceName:     serviceName,
		NeedEnvironment: true,
		NeedService:     true,
	})
	if err != nil {
		return err
	}

	// A service is structure: a delete-protected environment shields it.
	if err := cmdutil.RequireDeletable(client, ctx.Project.ID, ctx.Environment, "service", ctx.Service.Name); err != nil {
		return err
	}

	// Confirmation
	if !deleteServiceYes {
		fmt.Printf("Are you sure you want to delete service '%s' in %s/%s? (y/N): ", ctx.Service.Name, ctx.Project.Name, ctx.Environment.Name)
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// Delete the service
	err = client.DeleteService(ctx.Service.ID)
	if err != nil {
		return fmt.Errorf("failed to delete service: %w", err)
	}

	fmt.Printf("Service '%s' deleted.\n", ctx.Service.Name)
	return nil
}
