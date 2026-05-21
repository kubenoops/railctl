package cmd

import (
	"fmt"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/kubenoops/railctl/internal/output"
	"github.com/spf13/cobra"
)

var getVolumesCmd = &cobra.Command{
	Use:     "volumes",
	Aliases: []string{"volume", "vol"},
	Short:   "List volumes in an environment",
	Example: `  railctl get volumes -p my-project -e production
  railctl get volumes -p my-project -e production -o json
  railctl get volumes -p my-project -e production -o wide`,
	RunE: runGetVolumes,
}

func init() {
	getCmd.AddCommand(getVolumesCmd)
}

func runGetVolumes(cmd *cobra.Command, args []string) error {
	format, err := getOutputFormat()
	if err != nil {
		return err
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

	volumes, err := client.ListVolumes(ctx.Project.ID, ctx.Environment.ID)
	if err != nil {
		return err
	}

	// Get services to resolve service IDs to names
	services, err := client.ListServices(ctx.Project.ID, ctx.Environment.ID)
	if err != nil {
		return err
	}

	serviceMap := make(map[string]string)
	for _, svc := range services {
		serviceMap[svc.ID] = svc.Name
	}

	return cmdutil.PrintResult(
		format,
		volumesToOutput(volumes, serviceMap),
		volumesToTable(volumes, serviceMap),
		volumesToWideTable(volumes, serviceMap),
		"No volumes found.",
	)
}

// volumeOutput is the structured output for volume listing.
type volumeOutput struct {
	Name          string  `json:"name" yaml:"name"`
	ID            string  `json:"id" yaml:"id"`
	MountPath     string  `json:"mountPath" yaml:"mountPath"`
	AttachedTo    string  `json:"attachedTo,omitempty" yaml:"attachedTo,omitempty"`
	CurrentSizeMB float64 `json:"currentSizeMB" yaml:"currentSizeMB"`
	SizeMB        int     `json:"sizeMB" yaml:"sizeMB"`
}

func resolveAttachedTo(vol api.VolumeInstance, serviceMap map[string]string) string {
	if vol.ServiceID == nil {
		return ""
	}
	if name, ok := serviceMap[*vol.ServiceID]; ok {
		return name
	}
	return *vol.ServiceID
}

func volumesToOutput(volumes []api.VolumeInstance, serviceMap map[string]string) []volumeOutput {
	result := make([]volumeOutput, len(volumes))
	for i, vol := range volumes {
		result[i] = volumeOutput{
			Name:          vol.Volume.Name,
			ID:            vol.Volume.ID,
			MountPath:     vol.MountPath,
			AttachedTo:    resolveAttachedTo(vol, serviceMap),
			CurrentSizeMB: vol.CurrentSizeMB,
			SizeMB:        vol.SizeMB,
		}
	}
	return result
}

func volumesToTable(volumes []api.VolumeInstance, serviceMap map[string]string) *output.Table {
	table := output.NewTable("NAME", "MOUNT PATH", "ATTACHED TO", "SIZE USED")
	for _, vol := range volumes {
		attachedTo := resolveAttachedTo(vol, serviceMap)
		if attachedTo == "" {
			attachedTo = "-"
		}
		sizeUsed := fmt.Sprintf("%.1fMB/%dMB", vol.CurrentSizeMB, vol.SizeMB)
		table.AddRow(vol.Volume.Name, vol.MountPath, attachedTo, sizeUsed)
	}
	return table
}

func volumesToWideTable(volumes []api.VolumeInstance, serviceMap map[string]string) *output.Table {
	table := output.NewTable("NAME", "ID", "MOUNT PATH", "ATTACHED TO", "SIZE USED", "SIZE TOTAL")
	for _, vol := range volumes {
		attachedTo := resolveAttachedTo(vol, serviceMap)
		if attachedTo == "" {
			attachedTo = "-"
		}
		currentSize := fmt.Sprintf("%.1fMB", vol.CurrentSizeMB)
		totalSize := fmt.Sprintf("%dMB", vol.SizeMB)
		table.AddRow(vol.Volume.Name, vol.Volume.ID, vol.MountPath, attachedTo, currentSize, totalSize)
	}
	return table
}
