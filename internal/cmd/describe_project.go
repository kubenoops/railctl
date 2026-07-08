package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/kubenoops/railctl/internal/output"
	"github.com/kubenoops/railctl/internal/resolver"
	"github.com/kubenoops/railctl/internal/types"
)

// describeProjectCmd represents the "describe project" command.
var describeProjectCmd = &cobra.Command{
	Use:     "project <name>",
	Aliases: []string{"proj"},
	Short:   "Show detailed information about a project",
	Long: `Display detailed information about a specific project.

Shows the project name, ID, update time, and lists all environments
and services within the project.

The name can be an exact match or a unique substring. If the name
matches multiple projects, an error is returned with the matches.

Examples:
  railctl describe project my-app
  railctl describe project my-app -o json
  railctl describe project my-app -o yaml`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDescribeProject,
}

func init() {
	describeCmd.AddCommand(describeProjectCmd)
}

func runDescribeProject(cmd *cobra.Command, args []string) error {
	// Get project name from args or -p flag
	projectName := ""
	if len(args) > 0 {
		projectName = args[0]
	} else {
		projectName = getProject()
	}

	if projectName == "" {
		return fmt.Errorf("project name is required: railctl describe project <name>")
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

	if err := cmdutil.RequireWorkspaceScope(client, "describe a project by name"); err != nil {
		return err
	}

	// List all projects and resolve by name
	projects, err := client.ListProjects()
	if err != nil {
		return err
	}

	project, err := resolver.ResolveProject(projects, projectName)
	if err != nil {
		return err
	}

	// Get detailed project info
	detail, err := client.GetProject(project.ID)
	if err != nil {
		return err
	}

	printer := output.NewPrinter(format)

	switch printer.Format() {
	case output.FormatJSON:
		return printer.PrintJSON(projectDetailToOutput(detail))
	case output.FormatYAML:
		return printer.PrintYAML(projectDetailToOutput(detail))
	default:
		return printProjectDetail(detail)
	}
}

// projectDetailOutput is the structured output for a single project.
type projectDetailOutput struct {
	Name         string            `json:"name" yaml:"name"`
	ID           string            `json:"id" yaml:"id"`
	UpdatedAt    string            `json:"updatedAt" yaml:"updatedAt"`
	Environments []detailEnvOutput `json:"environments" yaml:"environments"`
}

type detailEnvOutput struct {
	Name     string                `json:"name" yaml:"name"`
	ID       string                `json:"id" yaml:"id"`
	Services []detailServiceOutput `json:"services" yaml:"services"`
}

type detailServiceOutput struct {
	Name string `json:"name" yaml:"name"`
	ID   string `json:"id" yaml:"id"`
}

func projectDetailToOutput(p types.Project) projectDetailOutput {
	envs := make([]detailEnvOutput, len(p.Environments))
	for i, e := range p.Environments {
		svcs := make([]detailServiceOutput, len(e.Services))
		for j, s := range e.Services {
			svcs[j] = detailServiceOutput{Name: s.Name, ID: s.ID}
		}
		envs[i] = detailEnvOutput{Name: e.Name, ID: e.ID, Services: svcs}
	}
	return projectDetailOutput{
		Name:         p.Name,
		ID:           p.ID,
		UpdatedAt:    p.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		Environments: envs,
	}
}

func printProjectDetail(p types.Project) error {
	fmt.Printf("Name:         %s\n", p.Name)
	fmt.Printf("ID:           %s\n", p.ID)
	fmt.Printf("Updated:      %s\n", types.RelativeTime(p.UpdatedAt))
	fmt.Printf("Environments: %d\n", p.EnvironmentCount())
	if len(p.Environments) > 0 {
		for _, env := range p.Environments {
			fmt.Printf("  - %s (%d services)\n", env.Name, len(env.Services))
			for _, svc := range env.Services {
				fmt.Printf("      - %s\n", svc.Name)
			}
		}
	}
	return nil
}
