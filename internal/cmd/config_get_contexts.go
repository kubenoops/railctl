package cmd

import (
	"github.com/spf13/cobra"

	"github.com/kubenoops/railctl/internal/contexts"
	"github.com/kubenoops/railctl/internal/output"
)

// configGetContextsCmd lists contexts, marking the active one. Token TYPES
// are shown (persisted at set-credentials time); token VALUES never
// (FR-008, FR-020).
var configGetContextsCmd = &cobra.Command{
	Use:     "get-contexts",
	Aliases: []string{"contexts"},
	Short:   "List contexts",
	Long: `List saved contexts, marking the active one with *.

Shows name, workspace, project, environment, credential reference, and the
credential's token type. Token values are never displayed.

Output formats:
  table (default)  Marked table
  json             Full context data as JSON array
  yaml             Full context data as YAML`,
	RunE: runConfigGetContexts,
}

func init() {
	configCmd.AddCommand(configGetContextsCmd)
}

// contextOutput is the structured output format for contexts; tokenType is
// resolved from the referenced credential, empty when none is referenced.
type contextOutput struct {
	Current     bool   `json:"current" yaml:"current"`
	Name        string `json:"name" yaml:"name"`
	Workspace   string `json:"workspace,omitempty" yaml:"workspace,omitempty"`
	Project     string `json:"project" yaml:"project"`
	Environment string `json:"environment,omitempty" yaml:"environment,omitempty"`
	Credentials string `json:"credentials,omitempty" yaml:"credentials,omitempty"`
	TokenType   string `json:"tokenType,omitempty" yaml:"tokenType,omitempty"`
}

func runConfigGetContexts(cmd *cobra.Command, args []string) error {
	format, err := getOutputFormat()
	if err != nil {
		return err
	}

	f, err := contexts.Load(contexts.ConfigPath())
	if err != nil {
		return err
	}

	printer := output.NewPrinter(format)
	switch printer.Format() {
	case output.FormatJSON:
		return printer.PrintJSON(contextsToOutput(f))
	case output.FormatYAML:
		return printer.PrintYAML(contextsToOutput(f))
	default:
		table := output.NewTable("Current", "Name", "Workspace", "Project", "Environment", "Credentials", "Token Type")
		for _, ctx := range f.Contexts {
			tokenType := ""
			if ctx.Credentials != "" {
				if cred := f.Credential(ctx.Credentials); cred != nil {
					tokenType = cred.TokenType
				}
			}
			marker := ""
			if ctx.Name == f.CurrentContext {
				marker = "*"
			}
			table.AddRow(marker, ctx.Name, ctx.Workspace, ctx.Project, ctx.Environment, ctx.Credentials, tokenType)
		}
		return printer.PrintTable(table)
	}
}

func contextsToOutput(f *contexts.File) []contextOutput {
	out := make([]contextOutput, len(f.Contexts))
	for i, ctx := range f.Contexts {
		tokenType := ""
		if ctx.Credentials != "" {
			if cred := f.Credential(ctx.Credentials); cred != nil {
				tokenType = cred.TokenType
			}
		}
		out[i] = contextOutput{
			Current:     ctx.Name == f.CurrentContext,
			Name:        ctx.Name,
			Workspace:   ctx.Workspace,
			Project:     ctx.Project,
			Environment: ctx.Environment,
			Credentials: ctx.Credentials,
			TokenType:   tokenType,
		}
	}
	return out
}
