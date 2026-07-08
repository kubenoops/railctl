package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/kubenoops/railctl/internal/output"
	"github.com/kubenoops/railctl/internal/types"
)

// describeEnvironmentCmd represents the "describe environment" command.
var describeEnvironmentCmd = &cobra.Command{
	Use:     "environment <name>",
	Aliases: []string{"env"},
	Short:   "Show detailed information about an environment",
	Long: `Display detailed information about a specific environment.

Shows the environment name, ID, services count, and update time.

The name can be an exact match or a unique substring.

Examples:
  railctl describe environment production -p my-app
  railctl describe env prod -p my-app -o json`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDescribeEnvironment,
}

func init() {
	describeCmd.AddCommand(describeEnvironmentCmd)
}

func runDescribeEnvironment(cmd *cobra.Command, args []string) error {
	// Get environment name from args or -e flag
	envName := ""
	if len(args) > 0 {
		envName = args[0]
	} else {
		envName = getEnvironment()
	}

	format, err := getOutputFormat()
	if err != nil {
		return err
	}

	tkn, err := getToken()
	if err != nil {
		return err
	}

	client := newAPIClient(tkn)

	// Resolve project and environment. With a project token both are derived
	// from the token itself; a conflicting <name> argument is warned about and
	// ignored (the token is bound to a single environment).
	ctx, err := cmdutil.ResolveContext(client, cmdutil.ResolveOpts{
		ProjectName:     getProject(),
		EnvironmentName: envName,
		NeedEnvironment: true,
	})
	if err != nil {
		return err
	}
	project := ctx.Project
	env := ctx.Environment

	printer := output.NewPrinter(format)

	switch printer.Format() {
	case output.FormatJSON:
		return printer.PrintJSON(envDetailToOutput(env, project.Name))
	case output.FormatYAML:
		return printer.PrintYAML(envDetailToOutput(env, project.Name))
	default:
		return printEnvironmentDetail(env, project.Name)
	}
}

// envDetailOutput is the structured output for a single environment.
type envDetailOutput struct {
	Name      string             `json:"name" yaml:"name"`
	ID        string             `json:"id" yaml:"id"`
	Project   string             `json:"project" yaml:"project"`
	Services  []envServiceOutput `json:"services" yaml:"services"`
	UpdatedAt string             `json:"updatedAt,omitempty" yaml:"updatedAt,omitempty"`
}

type envServiceOutput struct {
	Name string `json:"name" yaml:"name"`
	ID   string `json:"id" yaml:"id"`
}

func envDetailToOutput(env types.Environment, projectName string) envDetailOutput {
	services := make([]envServiceOutput, len(env.Services))
	for i, svc := range env.Services {
		services[i] = envServiceOutput{Name: svc.Name, ID: svc.ID}
	}

	out := envDetailOutput{
		Name:     env.Name,
		ID:       env.ID,
		Project:  projectName,
		Services: services,
	}
	if !env.UpdatedAt.IsZero() {
		out.UpdatedAt = env.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	return out
}

func printEnvironmentDetail(env types.Environment, projectName string) error {
	fmt.Printf("Name:         %s\n", env.Name)
	fmt.Printf("ID:           %s\n", env.ID)
	fmt.Printf("Project:      %s\n", projectName)

	if len(env.Services) == 0 {
		fmt.Printf("Services:     (none)\n")
	} else {
		fmt.Printf("Services:     %d\n", len(env.Services))
		for _, svc := range env.Services {
			fmt.Printf("  - %s\n", svc.Name)
		}
	}

	if !env.UpdatedAt.IsZero() {
		fmt.Printf("Updated:      %s\n", types.RelativeTime(env.UpdatedAt))
	}
	return nil
}
