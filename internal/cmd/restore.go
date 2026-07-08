package cmd

import (
	"github.com/spf13/cobra"
)

// restoreCmd represents the restore command group.
var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore resources",
	Long: `Restore resources in Railway.

Available resources:
  backup    Restore a volume from a backup`,
}

func init() {
	rootCmd.AddCommand(restoreCmd)
}
