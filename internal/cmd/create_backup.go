package cmd

import (
	"fmt"

	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/spf13/cobra"
)

var createBackupName string

var createBackupCmd = &cobra.Command{
	Use:     "backup <volume>",
	Aliases: []string{"bak"},
	Short:   "Create a manual backup of a volume",
	Long: `Create a manual backup of a volume, identified by name or ID.

Railway runs the backup asynchronously; use 'railctl get backups <volume>' to
see it once complete.`,
	Args: cobra.ExactArgs(1),
	Example: `  railctl create backup my-data -p my-project -e production
  railctl create backup my-data --name pre-migration -p my-project -e production`,
	RunE: runCreateBackup,
}

func init() {
	createBackupCmd.Flags().StringVar(&createBackupName, "name", "", "Name for the backup (optional)")
	createCmd.AddCommand(createBackupCmd)
}

func runCreateBackup(cmd *cobra.Command, args []string) error {
	volumeNameOrID := args[0]

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

	vol, err := resolveVolumeInstance(client, ctx.Project.ID, ctx.Environment.ID, volumeNameOrID)
	if err != nil {
		return err
	}

	if _, err := client.CreateVolumeBackup(vol.ID, createBackupName); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	if createBackupName != "" {
		fmt.Printf("Backup '%s' requested for volume '%s'.\n", createBackupName, vol.Volume.Name)
	} else {
		fmt.Printf("Backup requested for volume '%s'.\n", vol.Volume.Name)
	}
	return nil
}
