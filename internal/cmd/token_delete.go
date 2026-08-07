package cmd

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/kubenoops/railctl/internal/resolver"
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

	// Deleting resolves the token by listing first, which Railway denies to
	// project-scoped tokens (verified live) — so delete is unusable with one.
	if err := cmdutil.RequireWorkspaceScope(client, "delete a project token"); err != nil {
		return err
	}

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
		return fmt.Errorf("failed to list project tokens: %w", err)
	}
	var found *api.ProjectToken
	for i := range tokens {
		if tokens[i].ID == tokenID {
			found = &tokens[i]
			break
		}
	}
	if found == nil {
		available := make([]string, len(tokens))
		for i := range tokens {
			available[i] = tokens[i].Name
		}
		return resolver.ErrNotFound{Resource: "project token", Name: tokenID, Available: available}
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
