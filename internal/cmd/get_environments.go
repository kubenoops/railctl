package cmd

import (
	"strconv"

	"github.com/spf13/cobra"

	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/kubenoops/railctl/internal/output"
	"github.com/kubenoops/railctl/internal/types"
)

// getEnvironmentsCmd lists environments in a project.
var getEnvironmentsCmd = &cobra.Command{
	Use:     "environments",
	Short:   "List environments in a project",
	Long:    `List all environments in a project.`,
	Aliases: []string{"environment", "env", "envs"},
	Example: `  railctl get environments -p my-app
  railctl get envs -p my-app -o json`,
	RunE: runGetEnvironments,
}

func init() {
	getCmd.AddCommand(getEnvironmentsCmd)
}

func runGetEnvironments(cmd *cobra.Command, args []string) error {
	format, err := getOutputFormat()
	if err != nil {
		return err
	}

	token, err := getToken()
	if err != nil {
		return err
	}

	client := newAPIClient(token)

	ctx, err := cmdutil.ResolveContext(client, cmdutil.ResolveOpts{
		ProjectName: getProject(),
	})
	if err != nil {
		return err
	}

	environments, err := client.ListEnvironments(ctx.Project.ID)
	if err != nil {
		return err
	}

	return cmdutil.PrintResult(
		format,
		environmentsToOutput(environments),
		environmentsToTable(environments),
		nil,
		"No environments found.",
	)
}

// envOutput represents an environment for JSON/YAML output.
type envOutput struct {
	Name         string `json:"name" yaml:"name"`
	ID           string `json:"id" yaml:"id"`
	ServiceCount int    `json:"serviceCount" yaml:"serviceCount"`
	UpdatedAt    string `json:"updatedAt,omitempty" yaml:"updatedAt,omitempty"`
}

func environmentsToOutput(environments []types.Environment) []envOutput {
	result := make([]envOutput, len(environments))
	for i, env := range environments {
		result[i] = envOutput{
			Name:         env.Name,
			ID:           env.ID,
			ServiceCount: env.ServiceCount,
		}
		if !env.UpdatedAt.IsZero() {
			result[i].UpdatedAt = env.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
		}
	}
	return result
}

func environmentsToTable(environments []types.Environment) *output.Table {
	table := output.NewTable("NAME", "SERVICES", "UPDATED")
	for _, env := range environments {
		updated := "-"
		if !env.UpdatedAt.IsZero() {
			updated = types.RelativeTime(env.UpdatedAt)
		}
		table.AddRow(env.Name, strconv.Itoa(env.ServiceCount), updated)
	}
	return table
}
