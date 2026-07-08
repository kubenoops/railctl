package cmd

import (
	"fmt"
	"strings"

	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/kubenoops/railctl/internal/resolver"
	"github.com/spf13/cobra"
)

var updateVolumeCmd = &cobra.Command{
	Use:     "volume <name-or-id>",
	Aliases: []string{"vol"},
	Short:   "Update a volume's properties",
	Long: `Update a volume's name, mount path, or attachment.

You can rename, change mount path, attach to a service, or detach from a service.
Multiple operations can be performed in a single command.`,
	Args: cobra.ExactArgs(1),
	Example: `  # Rename a volume
  railctl update volume my-data --name uploads -p my-project -e production

  # Change mount path
  railctl update volume my-data --mount-path /app/uploads -p my-project -e production

  # Attach to a different service
  railctl update volume my-data --attach -s backend -p my-project -e production

  # Detach from service
  railctl update volume my-data --detach -p my-project -e production

  # Combine multiple operations
  railctl update volume my-data --name uploads --mount-path /app/uploads -p my-project -e production`,
	RunE: runUpdateVolume,
}

var (
	updateVolumeName      string
	updateVolumeMountPath string
	updateVolumeAttach    bool
	updateVolumeDetach    bool
)

func init() {
	updateVolumeCmd.Flags().StringVar(&updateVolumeName, "name", "", "New name for the volume")
	updateVolumeCmd.Flags().StringVar(&updateVolumeMountPath, "mount-path", "", "New mount path (must start with /)")
	updateVolumeCmd.Flags().BoolVar(&updateVolumeAttach, "attach", false, "Attach volume to service (requires --service)")
	updateVolumeCmd.Flags().BoolVar(&updateVolumeDetach, "detach", false, "Detach volume from service")
	updateCmd.AddCommand(updateVolumeCmd)
}

func runUpdateVolume(cmd *cobra.Command, args []string) error {
	volumeNameOrID := args[0]

	// Validate that at least one update flag is specified
	if updateVolumeName == "" && updateVolumeMountPath == "" && !updateVolumeAttach && !updateVolumeDetach {
		return fmt.Errorf("at least one of --name, --mount-path, --attach, or --detach must be specified")
	}

	// Validate that --attach and --detach are not both set
	if updateVolumeAttach && updateVolumeDetach {
		return fmt.Errorf("cannot specify both --attach and --detach")
	}

	// Validate mount path if specified
	if updateVolumeMountPath != "" && !strings.HasPrefix(updateVolumeMountPath, "/") {
		return fmt.Errorf("mount path must start with '/'")
	}

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

	// Find volume by name or ID
	volume, err := resolveVolumeInstance(client, ctx.Project.ID, ctx.Environment.ID, volumeNameOrID)
	if err != nil {
		return err
	}

	// Perform updates
	var updates []string

	// Update name
	if updateVolumeName != "" {
		if err := client.UpdateVolumeName(volume.Volume.ID, updateVolumeName); err != nil {
			return fmt.Errorf("failed to update volume name: %w", err)
		}
		updates = append(updates, fmt.Sprintf("renamed to '%s'", updateVolumeName))
	}

	// Attach to service
	if updateVolumeAttach {
		serviceFlag := getService()
		if serviceFlag == "" {
			return fmt.Errorf("--service is required when using --attach")
		}

		// Resolve service name to ID
		services, err := client.ListServices(ctx.Project.ID, ctx.Environment.ID)
		if err != nil {
			return err
		}

		svc, err := resolver.ResolveService(services, serviceFlag)
		if err != nil {
			return fmt.Errorf("service '%s' not found in environment", serviceFlag)
		}

		if err := client.AttachVolume(volume.Volume.ID, svc.ID, ctx.Environment.ID); err != nil {
			return fmt.Errorf("failed to attach volume: %w", err)
		}
		updates = append(updates, fmt.Sprintf("attached to service '%s'", svc.Name))
	}

	// Detach from service
	if updateVolumeDetach {
		if err := client.DetachVolume(volume.Volume.ID, ctx.Environment.ID); err != nil {
			return fmt.Errorf("failed to detach volume: %w", err)
		}
		updates = append(updates, "detached from service")
	}

	// Update mount path
	if updateVolumeMountPath != "" {
		// For mount path updates, we need the current service ID
		var currentServiceID string
		if volume.ServiceID != nil {
			currentServiceID = *volume.ServiceID
		} else if !updateVolumeAttach {
			return fmt.Errorf("cannot update mount path for detached volume (attach it first)")
		} else {
			// If attaching, use the newly attached service
			serviceFlag := getService()
			services, err := client.ListServices(ctx.Project.ID, ctx.Environment.ID)
			if err != nil {
				return err
			}
			svc, err := resolver.ResolveService(services, serviceFlag)
			if err != nil {
				return fmt.Errorf("service '%s' not found", serviceFlag)
			}
			currentServiceID = svc.ID
		}

		if err := client.UpdateVolumeMountPath(volume.Volume.ID, currentServiceID, ctx.Environment.ID, updateVolumeMountPath); err != nil {
			return fmt.Errorf("failed to update mount path: %w", err)
		}
		updates = append(updates, fmt.Sprintf("mount path changed to '%s'", updateVolumeMountPath))
	}

	// Print success message
	volumeDisplay := volume.Volume.Name
	if updateVolumeName != "" {
		volumeDisplay = updateVolumeName
	}
	fmt.Printf("Volume '%s' updated:\n", volumeDisplay)
	for _, update := range updates {
		fmt.Printf("  - %s\n", update)
	}

	return nil
}
