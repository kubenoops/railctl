package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/spf13/cobra"
)

var deleteVolumeCmd = &cobra.Command{
	Use:     "volume <name-or-id>",
	Aliases: []string{"vol"},
	Short:   "Delete a volume",
	Long: `Delete a volume by name or ID.

This operation is irreversible and will delete all data in the volume.`,
	Args: cobra.ExactArgs(1),
	Example: `  railctl delete volume my-data -p my-project -e production
  railctl delete volume volume-id-123 -p my-project -e production
  railctl delete volume my-data --yes -p my-project -e production`,
	RunE: runDeleteVolume,
}

var (
	deleteVolumeYes bool
)

func init() {
	deleteVolumeCmd.Flags().BoolVarP(&deleteVolumeYes, "yes", "y", false, "Skip confirmation prompt")
	deleteCmd.AddCommand(deleteVolumeCmd)
}

func runDeleteVolume(cmd *cobra.Command, args []string) error {
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

	// Get volumes to find the volume
	volumes, err := client.ListVolumes(ctx.Project.ID, ctx.Environment.ID)
	if err != nil {
		return err
	}

	// Find volume by name or ID
	var volumeID, volumeName string
	found := false
	for _, vol := range volumes {
		if vol.Volume.Name == volumeNameOrID || vol.Volume.ID == volumeNameOrID {
			volumeID = vol.Volume.ID
			volumeName = vol.Volume.Name
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("volume '%s' not found in environment", volumeNameOrID)
	}

	// Confirm deletion unless --yes is specified
	if !deleteVolumeYes {
		fmt.Printf("Are you sure you want to delete volume '%s'? This will delete all data. [y/N]: ", volumeName)
		reader := bufio.NewReader(os.Stdin)
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

	// Delete the volume
	if err := client.DeleteVolume(volumeID); err != nil {
		return fmt.Errorf("failed to delete volume: %w", err)
	}

	fmt.Printf("Volume '%s' deleted successfully.\n", volumeName)
	return nil
}
