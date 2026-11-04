package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kubenoops/railctl/internal/contexts"
)

// configUseContextCmd sets the active context as a purely local write —
// no API calls, no validation against Railway (FR-010).
var configUseContextCmd = &cobra.Command{
	Use:     "use-context <name>",
	Short:   "Set the active context",
	Aliases: []string{"use"},
	Long: `Set the active context. Every railctl command afterwards uses this
context's workspace, project, and environment as defaults (explicit flags
still win). This is a local write: no API calls are made and the target is
not validated — errors surface at resolution time if the target is stale.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := loadOrNewConfig()
		if err != nil {
			return err
		}
		if err := f.SetActive(args[0]); err != nil {
			return err
		}
		if err := contexts.Save(contexts.ConfigPath(), f); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Switched to context %q.\n", args[0])
		return nil
	},
}

func init() {
	configCmd.AddCommand(configUseContextCmd)
}
