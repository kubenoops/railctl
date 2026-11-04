package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kubenoops/railctl/internal/contexts"
)

var (
	setContextWorkspace   string
	setContextProject     string
	setContextEnvironment string
	setContextCredentials string
	setContextCurrent     bool
)

// configSetContextCmd creates or updates a context (upsert). Only the flags
// passed change an existing context (FR-005, FR-006); --current targets the
// active context in place (FR-007).
var configSetContextCmd = &cobra.Command{
	Use:   "set-context [name]",
	Short: "Create or update a context",
	Long: `Create or update a named context (upsert). Omitted flags keep their
existing values when updating. --current updates the active context instead
of a named one.

Examples:
  railctl config set-context shop-prod -w my-team -p shop -e production
  railctl config set-context shop-prod --credentials work
  railctl config set-context --current -e staging`,
	Args: func(cmd *cobra.Command, args []string) error {
		if setContextCurrent {
			if len(args) > 0 {
				return fmt.Errorf("cannot combine a context name with --current")
			}
			return nil
		}
		if len(args) != 1 {
			return fmt.Errorf("exactly one context name is required (or use --current)")
		}
		return nil
	},
	RunE: runConfigSetContext,
}

func init() {
	flags := configSetContextCmd.Flags()
	flags.StringVarP(&setContextWorkspace, "workspace", "w", "", "Workspace name")
	flags.StringVarP(&setContextProject, "project", "p", "", "Project name (required when creating)")
	flags.StringVarP(&setContextEnvironment, "environment", "e", "", "Environment name")
	flags.StringVar(&setContextCredentials, "credentials", "", "Credential name to attach")
	flags.BoolVar(&setContextCurrent, "current", false, "Update the active context")
	configCmd.AddCommand(configSetContextCmd)
}

func runConfigSetContext(cmd *cobra.Command, args []string) error {
	f, err := loadOrNewConfig()
	if err != nil {
		return err
	}

	name := ""
	if setContextCurrent {
		if f.CurrentContext == "" {
			return fmt.Errorf("no context is active")
		}
		name = f.CurrentContext
	} else {
		name = args[0]
	}

	update := contexts.ContextUpdate{
		Workspace:   changedString(cmd, "workspace", setContextWorkspace),
		Project:     changedString(cmd, "project", setContextProject),
		Environment: changedString(cmd, "environment", setContextEnvironment),
		Credentials: changedString(cmd, "credentials", setContextCredentials),
	}

	if err := f.UpsertContext(name, update); err != nil {
		return err
	}
	if err := contexts.Save(contexts.ConfigPath(), f); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Context %q set.\n", name)
	return nil
}

// changedString returns a pointer to the flag value only when the flag was
// explicitly provided, so upsert semantics can distinguish omitted from
// empty (FR-006).
func changedString(cmd *cobra.Command, flagName, value string) *string {
	if !cmd.Flags().Changed(flagName) {
		return nil
	}
	return &value
}
