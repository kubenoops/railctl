package cmd

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/spf13/cobra"
)

var tokenDeleteYes bool

var tokenDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete (revoke) a project token",
	Long: `Delete a project token by ID from a project.

This immediately revokes the token. This operation is irreversible.`,
	Args: cobra.ExactArgs(1),
	Example: `  railctl token delete <id> --project my-app
  railctl token delete <id> --project my-app --yes`,
	RunE: runTokenDelete,
}

func init() {
	tokenDeleteCmd.Flags().BoolVarP(&tokenDeleteYes, "yes", "y", false, "Skip confirmation prompt")
	tokenCmd.AddCommand(tokenDeleteCmd)
}

func runTokenDelete(cmd *cobra.Command, args []string) error {
	tokenID := args[0]

	tkn, err := getToken()
	if err != nil {
		return err
	}
	client := newAPIClient(tkn)

	ctx, err := cmdutil.ResolveContext(client, cmdutil.ResolveOpts{
		ProjectName:     getProject(),
		EnvironmentName: getEnvironment(),
		NeedEnvironment: false,
	})
	if err != nil {
		return err
	}

	// Resolve the token within the project for a friendly prompt + not-found error.
	tokens, err := client.ListProjectTokens(ctx.Project.ID)
	if err != nil {
		return err
	}
	var found *api.ProjectToken
	for i := range tokens {
		if tokens[i].ID == tokenID {
			found = &tokens[i]
			break
		}
	}
	if found == nil {
		return fmt.Errorf("project token '%s' not found in project '%s'", tokenID, ctx.Project.Name)
	}

	if !tokenDeleteYes {
		fmt.Fprintf(cmd.OutOrStdout(), "Delete project token '%s' (%s) from project '%s'? [y/N]: ", found.Name, tokenID, ctx.Project.Name)
		reader := bufio.NewReader(cmd.InOrStdin())
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}
		response = strings.ToLower(strings.TrimSpace(response))
		if response != "y" && response != "yes" {
			fmt.Fprintln(cmd.OutOrStdout(), "Deletion cancelled.")
			return nil
		}
	}

	if err := client.DeleteProjectToken(tokenID); err != nil {
		return fmt.Errorf("failed to delete project token: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Project token '%s' deleted.\n", found.Name)
	return nil
}
