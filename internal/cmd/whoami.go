package cmd

import (
	"fmt"
	"strings"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/kubenoops/railctl/internal/output"
	"github.com/spf13/cobra"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the current token's type and scope",
	Long: `Classifies the RAILWAY_TOKEN in use (account, workspace, or project scope)
and prints its containment chain:

  - account token:   all workspaces the account can access
  - workspace token: the single workspace the token is bound to
  - project token:   its workspace, project, and environment

The token value itself is never printed.`,
	Args: cobra.NoArgs,
	Example: `  railctl whoami
  railctl whoami -o json`,
	RunE: runWhoami,
}

func init() {
	rootCmd.AddCommand(whoamiCmd)
}

// whoamiScope is one link of the token's containment chain (workspace,
// project, or environment) for structured output.
type whoamiScope struct {
	ID   string `json:"id" yaml:"id"`
	Name string `json:"name" yaml:"name"`
}

// whoamiResult is the structured shape rendered by -o json/yaml. Exactly the
// fields applicable to the detected token type are populated.
type whoamiResult struct {
	Type        string        `json:"type" yaml:"type"`
	Workspace   *whoamiScope  `json:"workspace,omitempty" yaml:"workspace,omitempty"`
	Workspaces  []whoamiScope `json:"workspaces,omitempty" yaml:"workspaces,omitempty"`
	Project     *whoamiScope  `json:"project,omitempty" yaml:"project,omitempty"`
	Environment *whoamiScope  `json:"environment,omitempty" yaml:"environment,omitempty"`
}

func runWhoami(cmd *cobra.Command, args []string) error {
	format, err := getOutputFormat()
	if err != nil {
		return err
	}

	tkn, err := getToken()
	if err != nil {
		return err
	}

	client := newAPIClient(tkn)

	result, err := buildWhoamiResult(client)
	if err != nil {
		return err
	}

	return cmdutil.PrintResult(format, result, whoamiTable(result, false), whoamiTable(result, true), "no token scope information available")
}

// buildWhoamiResult classifies the token and resolves its containment chain.
func buildWhoamiResult(client api.APIClient) (whoamiResult, error) {
	isProject, err := client.IsProjectToken()
	if err != nil {
		return whoamiResult{}, err
	}

	if isProject {
		return buildProjectTokenResult(client)
	}

	isWorkspace, err := client.IsWorkspaceToken()
	if err != nil {
		return whoamiResult{}, err
	}

	workspaces, err := client.TokenWorkspaces()
	if err != nil {
		return whoamiResult{}, err
	}

	if isWorkspace {
		result := whoamiResult{Type: "workspace"}
		if len(workspaces) > 0 {
			result.Workspace = &whoamiScope{ID: workspaces[0].ID, Name: workspaces[0].Name}
		}
		return result, nil
	}

	result := whoamiResult{Type: "account", Workspaces: make([]whoamiScope, len(workspaces))}
	for i, ws := range workspaces {
		result.Workspaces[i] = whoamiScope{ID: ws.ID, Name: ws.Name}
	}
	return result, nil
}

// buildProjectTokenResult resolves the full workspace → project → environment
// chain a project-scoped token is pinned to.
func buildProjectTokenResult(client api.APIClient) (whoamiResult, error) {
	projectID, environmentID, err := client.GetProjectContext()
	if err != nil {
		return whoamiResult{}, err
	}

	result := whoamiResult{Type: "project"}

	workspaces, err := client.TokenWorkspaces()
	if err != nil {
		return whoamiResult{}, err
	}
	if len(workspaces) > 0 {
		result.Workspace = &whoamiScope{ID: workspaces[0].ID, Name: workspaces[0].Name}
	}

	proj, err := client.GetProject(projectID)
	if err != nil {
		return whoamiResult{}, fmt.Errorf("failed to resolve the token's project: %w", err)
	}
	result.Project = &whoamiScope{ID: projectID, Name: proj.Name}

	// Resolve the environment name; fall back to the raw ID if it cannot be
	// matched (the ID is still unambiguous scope information).
	envName := environmentID
	envs, err := client.ListEnvironments(projectID)
	if err != nil {
		return whoamiResult{}, fmt.Errorf("failed to resolve the token's environment: %w", err)
	}
	for _, env := range envs {
		if env.ID == environmentID {
			envName = env.Name
			break
		}
	}
	result.Environment = &whoamiScope{ID: environmentID, Name: envName}

	return result, nil
}

// whoamiTable renders the result as a FIELD/VALUE table. In wide mode each
// scope value carries its ID.
func whoamiTable(result whoamiResult, wide bool) *output.Table {
	table := output.NewTable("FIELD", "VALUE")
	table.AddRow("Type", result.Type)

	if result.Type == "account" {
		names := make([]string, len(result.Workspaces))
		for i, ws := range result.Workspaces {
			names[i] = formatWhoamiScope(ws, wide)
		}
		value := "-"
		if len(names) > 0 {
			value = strings.Join(names, ", ")
		}
		table.AddRow("Workspaces", value)
		return table
	}

	table.AddRow("Workspace", formatWhoamiScopePtr(result.Workspace, wide))
	if result.Type == "project" {
		table.AddRow("Project", formatWhoamiScopePtr(result.Project, wide))
		table.AddRow("Environment", formatWhoamiScopePtr(result.Environment, wide))
	}
	return table
}

func formatWhoamiScope(scope whoamiScope, wide bool) string {
	if wide {
		return fmt.Sprintf("%s (%s)", scope.Name, scope.ID)
	}
	return scope.Name
}

func formatWhoamiScopePtr(scope *whoamiScope, wide bool) string {
	if scope == nil {
		return "-"
	}
	return formatWhoamiScope(*scope, wide)
}
