package cmd

import (
	"fmt"
	"time"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/kubenoops/railctl/internal/output"
	"github.com/spf13/cobra"
)

var tokenListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List project tokens",
	Long: `List the project tokens for a project. Token values are masked; use
'railctl token create' to mint a new one. Pass --environment to filter by
environment.`,
	Args: cobra.NoArgs,
	Example: `  railctl token list --project my-app
  railctl token list --project my-app --environment production
  railctl token list -p my-app -o json`,
	RunE: runTokenList,
}

func init() {
	tokenCmd.AddCommand(tokenListCmd)
}

func runTokenList(cmd *cobra.Command, args []string) error {
	format, err := getOutputFormat()
	if err != nil {
		return err
	}

	tkn, err := getToken()
	if err != nil {
		return err
	}
	client := newAPIClient(tkn)

	// Railway denies token enumeration to project-scoped tokens (verified
	// live). Fail fast rather than surfacing a bare "Not Authorized".
	if err := cmdutil.RequireWorkspaceScope(client, "list project tokens"); err != nil {
		return err
	}

	needEnv := getEnvironment() != ""
	ctx, err := cmdutil.ResolveContext(client, cmdutil.ResolveOpts{
		ProjectName:     getProject(),
		EnvironmentName: getEnvironment(),
		NeedEnvironment: needEnv,
	})
	if err != nil {
		return err
	}

	tokens, err := client.ListProjectTokens(ctx.Project.ID)
	if err != nil {
		return fmt.Errorf("failed to list project tokens: %w", err)
	}

	if needEnv {
		var filtered []api.ProjectToken
		for _, tk := range tokens {
			if tk.EnvironmentID == ctx.Environment.ID {
				filtered = append(filtered, tk)
			}
		}
		tokens = filtered
	}

	// Best-effort environment id → name map for friendly output.
	envNames := map[string]string{}
	if envs, err := client.ListEnvironments(ctx.Project.ID); err == nil {
		for _, e := range envs {
			envNames[e.ID] = e.Name
		}
	}

	return cmdutil.PrintResult(
		format,
		tokensToOutput(tokens, envNames),
		tokensToTable(tokens, envNames),
		tokensToWideTable(tokens, envNames),
		fmt.Sprintf("No project tokens found for project '%s'.", ctx.Project.Name),
	)
}

// tokenOutput is the structured (-o json/yaml) form of a listed token.
type tokenOutput struct {
	Name        string `json:"name" yaml:"name"`
	Environment string `json:"environment" yaml:"environment"`
	ID          string `json:"id" yaml:"id"`
	CreatedAt   string `json:"createdAt" yaml:"createdAt"`
}

func envName(m map[string]string, id string) string {
	if n, ok := m[id]; ok && n != "" {
		return n
	}
	return id
}

func tokensToOutput(tokens []api.ProjectToken, envs map[string]string) []tokenOutput {
	result := make([]tokenOutput, len(tokens))
	for i, tk := range tokens {
		result[i] = tokenOutput{
			Name:        tk.Name,
			Environment: envName(envs, tk.EnvironmentID),
			ID:          tk.ID,
			CreatedAt:   tk.CreatedAt,
		}
	}
	return result
}

func tokensToTable(tokens []api.ProjectToken, envs map[string]string) *output.Table {
	table := output.NewTable("NAME", "ENVIRONMENT", "ID", "CREATED")
	for _, tk := range tokens {
		table.AddRow(tk.Name, envName(envs, tk.EnvironmentID), tk.ID, formatTokenTime(tk.CreatedAt))
	}
	return table
}

func tokensToWideTable(tokens []api.ProjectToken, envs map[string]string) *output.Table {
	table := output.NewTable("NAME", "ENVIRONMENT", "ID", "CREATED", "TOKEN")
	for _, tk := range tokens {
		table.AddRow(tk.Name, envName(envs, tk.EnvironmentID), tk.ID, formatTokenTime(tk.CreatedAt), tk.DisplayToken)
	}
	return table
}

// formatTokenTime renders an RFC3339 timestamp as "2006-01-02 15:04".
func formatTokenTime(ts string) string {
	if ts == "" {
		return "-"
	}
	if t, err := time.Parse(time.RFC3339, ts); err == nil {
		return t.Format("2006-01-02 15:04")
	}
	return ts
}
