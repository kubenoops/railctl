package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kubenoops/railctl/internal/contexts"
)

// configDeleteContextCmd removes a context. Deleting the active context is
// allowed and clears the active slot with a stderr notice — kubectl's
// dangling current-context behavior is deliberately not copied (FR-011).
var configDeleteContextCmd = &cobra.Command{
	Use:     "delete-context <name>",
	Aliases: []string{"delete"},
	Short:   "Delete a context",
	Long: `Delete a context. If the deleted context was active, the active slot is
cleared and subsequent commands behave as context-less until another
context is activated.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := loadOrNewConfig()
		if err != nil {
			return err
		}
		clearedActive, err := f.DeleteContext(args[0])
		if err != nil {
			return err
		}
		if err := contexts.Save(contexts.ConfigPath(), f); err != nil {
			return err
		}
		if clearedActive {
			fmt.Fprintf(cmd.ErrOrStderr(), "Note: deleted context %q was active; no context is active now.\n", args[0])
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Context %q deleted.\n", args[0])
		return nil
	},
}

func init() {
	configCmd.AddCommand(configDeleteContextCmd)
}
