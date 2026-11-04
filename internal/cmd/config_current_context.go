package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kubenoops/railctl/internal/contexts"
)

// configCurrentContextCmd prints the active context name and nothing else —
// prompt-friendly. No active context is an error with a non-zero exit,
// kubectl-parity (FR-009).
var configCurrentContextCmd = &cobra.Command{
	Use:   "current-context",
	Short: "Print the active context name",
	Long: `Print the name of the active context and nothing else. Fails with a
non-zero exit when no context is active (kubectl parity).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := contexts.Load(contexts.ConfigPath())
		if err != nil {
			return err
		}
		if f == nil || f.CurrentContext == "" {
			return fmt.Errorf("no active context is set")
		}
		fmt.Fprintln(cmd.OutOrStdout(), f.CurrentContext)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configCurrentContextCmd)
}
