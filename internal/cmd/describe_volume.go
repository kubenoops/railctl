package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/kubenoops/railctl/internal/output"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var describeVolumeCmd = &cobra.Command{
	Use:     "volume <name-or-id>",
	Aliases: []string{"vol"},
	Short:   "Show detailed information about a volume",
	Args:    cobra.ExactArgs(1),
	Example: `  railctl describe volume my-data -p my-project -e production
  railctl describe volume redis-bhv4-volume -p my-project -e production -o json`,
	RunE: runDescribeVolume,
}

func init() {
	describeCmd.AddCommand(describeVolumeCmd)
}

func runDescribeVolume(cmd *cobra.Command, args []string) error {
	volumeNameOrID := args[0]

	format, err := getOutputFormat()
	if err != nil {
		return err
	}

	token, err := getToken()
	if err != nil {
		return err
	}

	client := newAPIClient(token)

	// Resolve project and environment (derived from the token when it is
	// project-scoped, resolved by name otherwise).
	ctx, err := cmdutil.ResolveContext(client, cmdutil.ResolveOpts{
		ProjectName:     getProject(),
		EnvironmentName: getEnvironment(),
		NeedEnvironment: true,
	})
	if err != nil {
		return err
	}
	project := ctx.Project
	env := ctx.Environment

	// Find volume by name or ID
	volume, err := resolveVolumeInstance(client, project.ID, env.ID, volumeNameOrID)
	if err != nil {
		return err
	}

	// Get service name if attached
	var serviceName string
	if volume.ServiceID != nil {
		services, err := client.ListServices(project.ID, env.ID)
		if err == nil {
			for _, svc := range services {
				if svc.ID == *volume.ServiceID {
					serviceName = svc.Name
					break
				}
			}
		}
		if serviceName == "" {
			serviceName = *volume.ServiceID
		}
	}

	// Backup schedules (best-effort; don't fail describe if unavailable).
	var schedules []string
	if scheds, err := client.ListVolumeBackupSchedules(volume.ID); err == nil {
		for _, s := range scheds {
			schedules = append(schedules, s.Kind)
		}
	}

	switch format {
	case output.FormatJSON:
		data := buildVolumeDetail(volume, serviceName, project.Name, env.Name, schedules)
		out, _ := json.MarshalIndent(data, "", "  ")
		fmt.Println(string(out))
	case output.FormatYAML:
		data := buildVolumeDetail(volume, serviceName, project.Name, env.Name, schedules)
		out, _ := yaml.Marshal(data)
		fmt.Print(string(out))
	default:
		printVolumeDetail(volume, serviceName, project.Name, env.Name, schedules)
	}

	return nil
}

type volumeDetail struct {
	Name            string   `json:"name" yaml:"name"`
	ID              string   `json:"id" yaml:"id"`
	Project         string   `json:"project" yaml:"project"`
	Environment     string   `json:"environment" yaml:"environment"`
	MountPath       string   `json:"mountPath" yaml:"mountPath"`
	AttachedTo      string   `json:"attachedTo,omitempty" yaml:"attachedTo,omitempty"`
	CurrentSizeMB   float64  `json:"currentSizeMB" yaml:"currentSizeMB"`
	TotalSizeMB     int      `json:"totalSizeMB" yaml:"totalSizeMB"`
	UsagePercent    float64  `json:"usagePercent" yaml:"usagePercent"`
	BackupSchedules []string `json:"backupSchedules,omitempty" yaml:"backupSchedules,omitempty"`
}

func buildVolumeDetail(vol *api.VolumeInstance, serviceName, projectName, envName string, schedules []string) volumeDetail {
	attachedTo := ""
	if serviceName != "" {
		attachedTo = serviceName
	} else if vol.ServiceID != nil {
		attachedTo = *vol.ServiceID
	}

	usagePercent := 0.0
	if vol.SizeMB > 0 {
		usagePercent = (vol.CurrentSizeMB / float64(vol.SizeMB)) * 100
	}

	return volumeDetail{
		Name:            vol.Volume.Name,
		ID:              vol.Volume.ID,
		Project:         projectName,
		Environment:     envName,
		MountPath:       vol.MountPath,
		AttachedTo:      attachedTo,
		CurrentSizeMB:   vol.CurrentSizeMB,
		TotalSizeMB:     vol.SizeMB,
		UsagePercent:    usagePercent,
		BackupSchedules: schedules,
	}
}

func printVolumeDetail(vol *api.VolumeInstance, serviceName, projectName, envName string, schedules []string) {
	table := output.NewTable("FIELD", "VALUE")

	table.AddRow("Name", vol.Volume.Name)
	table.AddRow("ID", vol.Volume.ID)
	table.AddRow("Project", projectName)
	table.AddRow("Environment", envName)
	table.AddRow("Mount Path", vol.MountPath)

	if serviceName != "" {
		table.AddRow("Attached To", serviceName)
	} else if vol.ServiceID != nil {
		table.AddRow("Attached To", *vol.ServiceID)
	} else {
		table.AddRow("Attached To", "-")
	}

	usedSize := fmt.Sprintf("%.1f MB", vol.CurrentSizeMB)
	totalSize := fmt.Sprintf("%d MB", vol.SizeMB)
	usagePercent := 0.0
	if vol.SizeMB > 0 {
		usagePercent = (vol.CurrentSizeMB / float64(vol.SizeMB)) * 100
	}

	table.AddRow("Size Used", usedSize)
	table.AddRow("Size Total", totalSize)
	table.AddRow("Usage", fmt.Sprintf("%.1f%%", usagePercent))

	if len(schedules) > 0 {
		table.AddRow("Backup Schedules", strings.Join(schedules, ", "))
	} else {
		table.AddRow("Backup Schedules", "-")
	}

	fmt.Println(table.Render())
}
