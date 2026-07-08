package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/spf13/cobra"
)

var (
	restoreBackupVolume string
	restoreBackupYes    bool
)

var restoreBackupCmd = &cobra.Command{
	Use:   "backup <backup-id>",
	Short: "Restore a volume from a backup",
	Long: `Restore a volume from a backup, identified by ID.

Railway stages the restore as a new volume mounted at the original location;
the change must be finalized with a deployment. Restoring removes any backups
created after the restored point in time (earlier backups are kept).`,
	Args: cobra.ExactArgs(1),
	Example: `  railctl restore backup backup-id-123 --volume my-data -p my-project -e production
  railctl restore backup backup-id-123 --volume my-data --yes -p my-project -e production`,
	RunE: runRestoreBackup,
}

func init() {
	restoreBackupCmd.Flags().StringVar(&restoreBackupVolume, "volume", "", "Volume to restore (name or ID, required)")
	restoreBackupCmd.Flags().BoolVarP(&restoreBackupYes, "yes", "y", false, "Skip confirmation prompt")
	_ = restoreBackupCmd.MarkFlagRequired("volume")
	restoreCmd.AddCommand(restoreBackupCmd)
}

func runRestoreBackup(cmd *cobra.Command, args []string) error {
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

	vol, err := resolveVolumeInstance(client, ctx.Project.ID, ctx.Environment.ID, restoreBackupVolume)
	if err != nil {
		return err
	}

	if !restoreBackupYes {
		fmt.Printf("Restore volume '%s' from backup '%s'?\n", vol.Volume.Name, backupID)
		fmt.Println("  - A new volume will be staged; deploy the service to finalize the restore.")
		fmt.Println("  - Backups created after this point in time will be removed.")
		fmt.Print("Continue? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}
		response = strings.ToLower(strings.TrimSpace(response))
		if response != "y" && response != "yes" {
			fmt.Println("Restore cancelled.")
			return nil
		}
	}

	if err := client.RestoreVolumeBackup(backupID, vol.ID); err != nil {
		return fmt.Errorf("failed to restore backup: %w", err)
	}

	fmt.Printf("Restore of volume '%s' from backup '%s' initiated.\n", vol.Volume.Name, backupID)
	fmt.Println("Deploy the service to finalize the restore.")
	return nil
}
