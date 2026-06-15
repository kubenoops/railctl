package cmd

import (
	"fmt"
	"strings"

	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/spf13/cobra"
)

var createVolumeCmd = &cobra.Command{
	Use:     "volume <name> --mount-path <path>",
	Aliases: []string{"vol"},
	Short:   "Create a new volume attached to a service",
	Long: `Create a new volume and attach it to a service.

The mount path must start with '/' and specifies where the volume will be mounted.
Each service can only have one volume attached.`,
	Args: cobra.MaximumNArgs(1),
	Example: `  railctl create volume --mount-path /app/data -s backend -p my-project -e production
  railctl create volume my-data --mount-path /app/uploads -s api -p my-project -e production`,
	RunE: runCreateVolume,
}

var (
	volumeMountPath string
)

func init() {
	createVolumeCmd.Flags().StringVar(&volumeMountPath, "mount-path", "", "Mount path for the volume (required, must start with /)")
	createVolumeCmd.MarkFlagRequired("mount-path")
	createCmd.AddCommand(createVolumeCmd)
}

func runCreateVolume(cmd *cobra.Command, args []string) error {
	// Volume name is optional - Railway will auto-generate if not provided
	var volumeName string
	if len(args) > 0 {
		volumeName = args[0]
	}

	token, err := getToken()
	if err != nil {
		return err
	}

	// Validate mount path
	if !strings.HasPrefix(volumeMountPath, "/") {
		return fmt.Errorf("mount path must start with '/'")
	}

	client := newAPIClient(token)

	ctx, err := cmdutil.ResolveContext(client, cmdutil.ResolveOpts{
		ProjectName:     getProject(),
		EnvironmentName: getEnvironment(),
		ServiceName:     getService(),
		NeedEnvironment: true,
		NeedService:     true,
	})
	if err != nil {
		return err
	}

	// Create the volume
	vol, err := client.CreateVolume(ctx.Project.ID, ctx.Environment.ID, ctx.Service.ID, volumeMountPath)
	if err != nil {
		return fmt.Errorf("failed to create volume: %w", err)
	}

	// Rename the volume if the user provided a name and Railway generated a different one.
	if volumeName != "" && vol.Name != volumeName {
		if err := client.UpdateVolumeName(vol.ID, volumeName); err != nil {
			fmt.Printf("Volume '%s' created and attached to service '%s' at '%s'\n",
				vol.Name, ctx.Service.Name, volumeMountPath)
			fmt.Printf("Volume ID: %s\n", vol.ID)
			return fmt.Errorf("failed to rename volume to '%s': %w", volumeName, err)
		}
		vol.Name = volumeName
	}

	fmt.Printf("Volume '%s' created and attached to service '%s' at '%s'\n",
		vol.Name, ctx.Service.Name, volumeMountPath)
	fmt.Printf("Volume ID: %s\n", vol.ID)

	return nil
}
