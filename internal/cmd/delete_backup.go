package cmd

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/spf13/cobra"
)

var (
	deleteBackupVolume string
	deleteBackupYes    bool
)

var deleteBackupCmd = &cobra.Command{
	Use:     "backup <backup-id>",
	Aliases: []string{"bak"},
	Short:   "Delete a volume backup",
	Long: `Delete a backup by ID from a volume.

This operation is irreversible.`,
	Args: cobra.ExactArgs(1),
	Example: `  railctl delete backup backup-id-123 --volume my-data -p my-project -e production
  railctl delete backup backup-id-123 --volume my-data --yes -p my-project -e production`,
	RunE: runDeleteBackup,
}

func init() {
	deleteBackupCmd.Flags().StringVar(&deleteBackupVolume, "volume", "", "Volume the backup belongs to (name or ID, required)")
	deleteBackupCmd.Flags().BoolVarP(&deleteBackupYes, "yes", "y", false, "Skip confirmation prompt")
	_ = deleteBackupCmd.MarkFlagRequired("volume")
	deleteCmd.AddCommand(deleteBackupCmd)
}

func runDeleteBackup(cmd *cobra.Command, args []string) error {
	backupID := args[0]

	token, err := getToken()
	if err != nil {
		return err
	}

	client := newAPIClient(token)

	ctx, err := cmdutil.ResolveContext(client, cmdutil.ResolveOpts{
		ProjectName:     getProject(),
		EnvironmentName: getEnvironment(),
		NeedEnvironment: true,
	})
	if err != nil {
		return err
	}

	vol, err := resolveVolumeInstance(client, ctx.Project.ID, ctx.Environment.ID, deleteBackupVolume)
	if err != nil {
		return err
	}

	// A backup is data: a delete-protected environment shields it.
	if err := cmdutil.RequireDeletable(client, ctx.Project.ID, ctx.Environment, "backup", backupID); err != nil {
		return err
	}

	if !deleteBackupYes {
		fmt.Printf("Are you sure you want to delete backup '%s' from volume '%s'? [y/N]: ", backupID, vol.Volume.Name)
		reader := bufio.NewReader(cmd.InOrStdin())
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}
		response = strings.ToLower(strings.TrimSpace(response))
		if response != "y" && response != "yes" {
			fmt.Println("Deletion cancelled.")
			return nil
		}
	}

	if err := client.DeleteVolumeBackup(backupID, vol.ID); err != nil {
		return fmt.Errorf("failed to delete backup: %w", err)
	}

	fmt.Printf("Backup '%s' deleted from volume '%s'.\n", backupID, vol.Volume.Name)
	return nil
}
