package cmd

import (
	"fmt"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/kubenoops/railctl/internal/output"
	"github.com/spf13/cobra"
)

var getBackupsSchedules bool

var getBackupsCmd = &cobra.Command{
	Use:     "backups <volume>",
	Aliases: []string{"backup", "bak"},
	Short:   "List backups (or backup schedules) for a volume",
	Long: `List the backups for a volume, identified by name or ID.

Use --schedules to list the automated backup schedules instead of backups.`,
	Args: cobra.ExactArgs(1),
	Example: `  railctl get backups my-data -p my-project -e production
  railctl get backups my-data --schedules -p my-project -e production
  railctl get backups my-data -o json -p my-project -e production`,
	RunE: runGetBackups,
}

func init() {
	getBackupsCmd.Flags().BoolVar(&getBackupsSchedules, "schedules", false, "List backup schedules instead of backups")
	getCmd.AddCommand(getBackupsCmd)
}

func runGetBackups(cmd *cobra.Command, args []string) error {
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

	if getBackupsSchedules {
		schedules, err := client.ListVolumeBackupSchedules(vol.ID)
		if err != nil {
			return err
		}
		return cmdutil.PrintResult(
			format,
			schedulesToOutput(schedules),
			schedulesToTable(schedules),
			schedulesToWideTable(schedules),
			fmt.Sprintf("No backup schedules found for volume '%s'.", vol.Volume.Name),
		)
	}

	backups, err := client.ListVolumeBackups(vol.ID)
	if err != nil {
		return err
	}
	return cmdutil.PrintResult(
		format,
		backupsToOutput(backups),
		backupsToTable(backups),
		backupsToWideTable(backups),
		fmt.Sprintf("No backups found for volume '%s'.", vol.Volume.Name),
	)
}

// backupOutput is the structured output for a backup.
type backupOutput struct {
	Name         string `json:"name" yaml:"name"`
	ID           string `json:"id" yaml:"id"`
	Type         string `json:"type" yaml:"type"`
	CreatedAt    string `json:"createdAt" yaml:"createdAt"`
	ExpiresAt    string `json:"expiresAt,omitempty" yaml:"expiresAt,omitempty"`
	UsedMB       int    `json:"usedMB" yaml:"usedMB"`
	ReferencedMB int    `json:"referencedMB" yaml:"referencedMB"`
}

func backupType(b api.VolumeBackup) string {
	if b.ScheduleID != "" {
		return "scheduled"
	}
	return "manual"
}

func backupsToOutput(backups []api.VolumeBackup) []backupOutput {
	result := make([]backupOutput, len(backups))
	for i, b := range backups {
		result[i] = backupOutput{
			Name:         b.Name,
			ID:           b.ID,
			Type:         backupType(b),
			CreatedAt:    b.CreatedAt,
			ExpiresAt:    b.ExpiresAt,
			UsedMB:       b.UsedMB,
			ReferencedMB: b.ReferencedMB,
		}
	}
	return result
}

func backupsToTable(backups []api.VolumeBackup) *output.Table {
	table := output.NewTable("NAME", "TYPE", "CREATED", "EXPIRES", "SIZE USED")
	for _, b := range backups {
		table.AddRow(b.Name, backupType(b), formatBackupTime(b.CreatedAt), formatBackupTime(b.ExpiresAt), fmt.Sprintf("%dMB", b.UsedMB))
	}
	return table
}

func backupsToWideTable(backups []api.VolumeBackup) *output.Table {
	table := output.NewTable("NAME", "ID", "TYPE", "CREATED", "EXPIRES", "SIZE USED", "REFERENCED")
	for _, b := range backups {
		table.AddRow(b.Name, b.ID, backupType(b), formatBackupTime(b.CreatedAt), formatBackupTime(b.ExpiresAt), fmt.Sprintf("%dMB", b.UsedMB), fmt.Sprintf("%dMB", b.ReferencedMB))
	}
	return table
}

// scheduleOutput is the structured output for a backup schedule.
type scheduleOutput struct {
	Kind             string `json:"kind" yaml:"kind"`
	Name             string `json:"name" yaml:"name"`
	Cron             string `json:"cron" yaml:"cron"`
	RetentionSeconds int    `json:"retentionSeconds" yaml:"retentionSeconds"`
	ID               string `json:"id" yaml:"id"`
}

func schedulesToOutput(schedules []api.BackupSchedule) []scheduleOutput {
	result := make([]scheduleOutput, len(schedules))
	for i, s := range schedules {
		result[i] = scheduleOutput{
			Kind:             s.Kind,
			Name:             s.Name,
			Cron:             s.Cron,
			RetentionSeconds: s.RetentionSeconds,
			ID:               s.ID,
		}
	}
	return result
}

func schedulesToTable(schedules []api.BackupSchedule) *output.Table {
	table := output.NewTable("KIND", "CRON", "RETENTION")
	for _, s := range schedules {
		table.AddRow(s.Kind, s.Cron, formatRetention(s.RetentionSeconds))
	}
	return table
}

func schedulesToWideTable(schedules []api.BackupSchedule) *output.Table {
	table := output.NewTable("KIND", "NAME", "CRON", "RETENTION", "ID")
	for _, s := range schedules {
		table.AddRow(s.Kind, s.Name, s.Cron, formatRetention(s.RetentionSeconds), s.ID)
	}
	return table
}

// formatRetention renders a retention period in days.
func formatRetention(seconds int) string {
	if seconds <= 0 {
		return "-"
	}
	days := seconds / 86400
	if days > 0 {
		return fmt.Sprintf("%dd", days)
	}
	return fmt.Sprintf("%ds", seconds)
}
