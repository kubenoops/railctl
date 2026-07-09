package cmd

import (
	"fmt"

	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/kubenoops/railctl/internal/output"
	"github.com/spf13/cobra"
)

// protectCmd is the parent command for delete-protection toggles.
var protectCmd = &cobra.Command{
	Use:   "protect",
	Short: "Mark resources as delete-protected",
	Long: `Mark resources as delete-protected so they cannot be deleted until unprotected.

Available resources:
  environment    Delete-protect an environment (and, by extension, its project)`,
}

// protectEnvironmentCmd sets DELETE_PROTECTION=true on an environment.
var protectEnvironmentCmd = &cobra.Command{
	Use:   "environment NAME",
	Short: "Delete-protect an environment",
	Long: `Delete-protect an environment by setting its DELETE_PROTECTION shared variable
to a truthy value. While protected, the environment cannot be deleted, and its
project cannot be deleted either (deleting a project is refused when any of its
environments is protected).

This sets an environment-level (shared, serviceless) variable, so it requires an
account or workspace token — a project token cannot write shared variables.

The operation is idempotent: protecting an already-protected environment simply
re-asserts the flag and preserves every other shared variable.`,
	Aliases: []string{"env"},
	Args:    cobra.ExactArgs(1),
	Example: `  railctl protect environment production -p my-app
  railctl protect env production -p my-app -o json`,
	RunE: runProtectEnvironment,
}

func init() {
	protectCmd.AddCommand(protectEnvironmentCmd)
	rootCmd.AddCommand(protectCmd)
}

// protectionOutput is the structured (-o json/yaml) form of a protection toggle.
type protectionOutput struct {
	Project          string `json:"project" yaml:"project"`
	Environment      string `json:"environment" yaml:"environment"`
	DeleteProtection bool   `json:"deleteProtection" yaml:"deleteProtection"`
}

func runProtectEnvironment(cmd *cobra.Command, args []string) error {
	return toggleDeleteProtection(cmd, args[0], true)
}

// toggleDeleteProtection resolves the project/environment and sets or clears
// DELETE_PROTECTION on that environment. It is shared by `protect environment`
// and `unprotect environment`.
func toggleDeleteProtection(cmd *cobra.Command, envName string, protect bool) error {
	format, err := getOutputFormat()
	if err != nil {
		return err
	}

	tkn, err := getToken()
	if err != nil {
		return err
	}
	client := newAPIClient(tkn)

	op := "protect an environment"
	if !protect {
		op = "unprotect an environment"
	}
	if err := cmdutil.RequireWorkspaceScope(client, op); err != nil {
		return err
	}

	projectName := getProject()
	if projectName == "" {
		return fmt.Errorf("project required: use -p flag or set RAILCTL_PROJECT")
	}

	ctx, err := cmdutil.ResolveContext(client, cmdutil.ResolveOpts{
		ProjectName:     projectName,
		EnvironmentName: envName,
		NeedEnvironment: true,
	})
	if err != nil {
		return err
	}

	if err := cmdutil.SetDeleteProtection(client, ctx.Project.ID, ctx.Environment.ID, protect); err != nil {
		return err
	}

	switch format {
	case output.FormatJSON, output.FormatYAML:
		return cmdutil.PrintResult(format, protectionOutput{
			Project:          ctx.Project.Name,
			Environment:      ctx.Environment.Name,
			DeleteProtection: protect,
		}, nil, nil, "")
	default:
		if protect {
			fmt.Fprintf(cmd.OutOrStdout(),
				"Environment '%s' is now delete-protected — it and its project cannot be deleted until unprotected.\n",
				ctx.Environment.Name)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(),
				"Environment '%s' is no longer delete-protected.\n",
				ctx.Environment.Name)
		}
		return nil
	}
}
