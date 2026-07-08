package cmd

import (
	"fmt"

	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/kubenoops/railctl/internal/output"
	"github.com/spf13/cobra"
)

var tokenCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a project token scoped to a project + environment",
	Long: `Create a project token for a project and environment.

The raw token is printed to stdout and shown only once — store it immediately.
Works with any token type; with a project token the new token is minted within
that token's own project and environment (-p/-e are ignored in that case).`,
	Args: cobra.ExactArgs(1),
	Example: `  railctl token create ci --project my-app --environment production
  TOKEN=$(railctl token create ci -p my-app -e production)`,
	RunE: runTokenCreate,
}

func init() {
	tokenCmd.AddCommand(tokenCreateCmd)
}

// tokenCreateOutput is the structured (-o json/yaml) form of a minted token.
type tokenCreateOutput struct {
	Name          string `json:"name" yaml:"name"`
	ProjectID     string `json:"projectId" yaml:"projectId"`
	EnvironmentID string `json:"environmentId" yaml:"environmentId"`
	Token         string `json:"token" yaml:"token"`
}

func runTokenCreate(cmd *cobra.Command, args []string) error {
	name := args[0]

	format, err := getOutputFormat()
	if err != nil {
		return err
	}

	tkn, err := getToken()
	if err != nil {
		return err
	}
	client := newAPIClient(tkn)

	ctx, err := cmdutil.ResolveContext(client, cmdutil.ResolveOpts{
		ProjectName:     getProject(),
		EnvironmentName: getEnvironment(),
		NeedEnvironment: true,
	})
	if err != nil {
		return err
	}

	value, err := client.CreateProjectToken(ctx.Project.ID, ctx.Environment.ID, name)
	if err != nil {
		return fmt.Errorf("failed to create project token: %w", err)
	}

	switch format {
	case output.FormatJSON, output.FormatYAML:
		return cmdutil.PrintResult(format, tokenCreateOutput{
			Name:          name,
			ProjectID:     ctx.Project.ID,
			EnvironmentID: ctx.Environment.ID,
			Token:         value,
		}, nil, nil, "")
	default:
		fmt.Fprintf(cmd.ErrOrStderr(),
			"Created project token '%s' (project %s / %s). Store it now — it will not be shown again.\n",
			name, ctx.Project.Name, ctx.Environment.Name)
		fmt.Fprintln(cmd.OutOrStdout(), value)
		return nil
	}
}
