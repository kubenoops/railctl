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

This sets an environment-level (shared, serviceless) variable. It works with ANY
token scoped to the environment — including a project token — so you can protect
with the same least-privilege token you use for everything else.

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

	// No token-scope gate: writing an environment-level shared variable works
	// with ANY token scoped to that environment — including a project token
	// (verified live; the earlier "project token cannot write shared variables"
	// belief was a raw-probe artifact of the wrong auth header). ResolveContext
	// handles both cases: a project token derives its own project/env scope, and
	// a broader token with no -p gets a clear "project required" error.
	ctx, err := cmdutil.ResolveContext(client, cmdutil.ResolveOpts{
		ProjectName:     getProject(),
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
				"Environment '%s' is now delete-protected — the environment, its project, and its services/volumes/backups cannot be deleted until unprotected (updates, creates, and config/rollback deletes still work).\n",
				ctx.Environment.Name)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(),
				"Environment '%s' is no longer delete-protected.\n",
				ctx.Environment.Name)
		}
		return nil
	}
}
