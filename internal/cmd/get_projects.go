package cmd

import (
	"strconv"

	"github.com/spf13/cobra"

	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/kubenoops/railctl/internal/output"
	"github.com/kubenoops/railctl/internal/types"
)

// getProjectsCmd represents the "get projects" command.
var getProjectsCmd = &cobra.Command{
	Use:     "projects",
	Aliases: []string{"project", "proj"},
	Short:   "List all projects in the workspace",
	Long: `List all projects accessible with the current API token.

Output formats:
  table (default)  Shows name, services, updated time
  wide             Includes environments, environment names, service names
  json             Full project data as JSON array
  yaml             Full project data as YAML

Examples:
  railctl get projects
  railctl get projects -o wide
  railctl get projects -o json`,
	RunE: runGetProjects,
}

func init() {
	getCmd.AddCommand(getProjectsCmd)
}

func runGetProjects(cmd *cobra.Command, args []string) error {
	format, err := getOutputFormat()
	if err != nil {
		return err
	}

	tkn, err := getToken()
	if err != nil {
		return err
	}

	client := newAPIClient(tkn)

	if err := cmdutil.RequireWorkspaceScope(client, "list projects"); err != nil {
		return err
	}

	projects, err := client.ListProjects()
	if err != nil {
		return err
	}

	printer := output.NewPrinter(format)

	switch printer.Format() {
	case output.FormatJSON:
		return printer.PrintJSON(projectsToOutput(projects))
	case output.FormatYAML:
		return printer.PrintYAML(projectsToOutput(projects))
	case output.FormatWide:
		return printer.PrintTable(projectsToWideTable(projects))
	default:
		return printer.PrintTable(projectsToTable(projects))
	}
}

// projectOutput is the structured output format for projects.
type projectOutput struct {
	Name         string          `json:"name" yaml:"name"`
	ID           string          `json:"id" yaml:"id"`
	Environments []projEnvOutput `json:"environments" yaml:"environments"`
	Services     []projSvcOutput `json:"services" yaml:"services"`
	UpdatedAt    string          `json:"updatedAt" yaml:"updatedAt"`
}

type projEnvOutput struct {
	Name string `json:"name" yaml:"name"`
	ID   string `json:"id" yaml:"id"`
}

type projSvcOutput struct {
	Name string `json:"name" yaml:"name"`
	ID   string `json:"id" yaml:"id"`
}

func projectsToOutput(projects []types.Project) []projectOutput {
	out := make([]projectOutput, len(projects))
	for i, p := range projects {
		envs := make([]projEnvOutput, len(p.Environments))
		for j, e := range p.Environments {
			envs[j] = projEnvOutput{Name: e.Name, ID: e.ID}
		}
		svcs := make([]projSvcOutput, len(p.Services))
		for j, s := range p.Services {
			svcs[j] = projSvcOutput{Name: s.Name, ID: s.ID}
		}
		out[i] = projectOutput{
			Name:         p.Name,
			ID:           p.ID,
			Environments: envs,
			Services:     svcs,
			UpdatedAt:    p.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}
	return out
}

func projectsToTable(projects []types.Project) *output.Table {
	table := output.NewTable("Name", "Services", "Updated")
	for _, p := range projects {
		table.AddRow(
			p.Name,
			strconv.Itoa(p.ServiceCount()),
			types.RelativeTime(p.UpdatedAt),
		)
	}
	return table
}

func projectsToWideTable(projects []types.Project) *output.Table {
	table := output.NewTable("Name", "Environments", "Services", "Environment Names", "Service Names", "Updated")
	for _, p := range projects {
		table.AddRow(
			p.Name,
			strconv.Itoa(p.EnvironmentCount()),
			strconv.Itoa(p.ServiceCount()),
			p.EnvironmentNames(),
			p.ServiceNames(),
			types.RelativeTime(p.UpdatedAt),
		)
	}
	return table
}

// formatEnvList formats environments as "env1, env2" for table output.
func formatEnvList(envs []types.Environment) string {
	if len(envs) == 0 {
		return ""
	}
	result := ""
	for i, env := range envs {
		if i > 0 {
			result += ", "
		}
		result += env.Name
	}
	return result
}

// formatSvcList formats services as "svc1, svc2" for table output.
func formatSvcList(svcs []types.Service) string {
	if len(svcs) == 0 {
		return ""
	}
	result := ""
	for i, svc := range svcs {
		if i > 0 {
			result += ", "
		}
		result += svc.Name
	}
	return result
}
