package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/spf13/cobra"
)

var (
	logsTail       int
	logsDeployment string
	logsFollow     bool
)

// logsServiceCmd represents the logs service command.
var logsServiceCmd = &cobra.Command{
	Use:   "logs <service>",
	Short: "View deployment logs for a service",
	Long: `View deployment logs for a service.

By default, shows the last 100 log lines from the latest successful deployment.
Use --tail to adjust the number of lines shown, or --deployment to specify
a particular deployment ID.

Examples:
  # View latest logs for a service
  railctl logs backend -p my-project -e production

  # View only the last 20 log lines
  railctl logs backend -p my-project -e production --tail 20

  # Follow logs in real-time (Ctrl+C to stop)
  railctl logs backend -f -p my-project -e production

  # View logs for a specific deployment
  railctl logs backend -p my-project -e production --deployment abc123
`,
	// One service name; "logs service <name>" is tolerated for compatibility
	// (the old help text and docs taught it for months).
	Args: cobra.RangeArgs(1, 2),
	RunE: runLogsService,
}

func init() {
	rootCmd.AddCommand(logsServiceCmd)
	// Disable interspersed flag parsing restriction
	logsServiceCmd.Flags().SetInterspersed(true)
	logsServiceCmd.Flags().IntVar(&logsTail, "tail", 100, "Number of log lines to show (max: 500)")
	logsServiceCmd.Flags().StringVar(&logsDeployment, "deployment", "", "Specific deployment ID (defaults to latest successful)")
	logsServiceCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Follow log output (stream new logs)")
}

func runLogsService(cmd *cobra.Command, args []string) error {
	serviceName := args[0]
	if len(args) == 2 {
		// Compatibility: `railctl logs service <name>` (the syntax previously
		// shown in help/docs). Anything else with two args is a mistake.
		if args[0] != "service" {
			return fmt.Errorf("unexpected argument %q — usage: railctl logs <service>", args[1])
		}
		serviceName = args[1]
	}

	tkn, err := getToken()
	if err != nil {
		return err
	}
	client := newAPIClient(tkn)

	ctx, err := cmdutil.ResolveContext(client, cmdutil.ResolveOpts{
		ProjectName:     getProject(),
		EnvironmentName: getEnvironment(),
		ServiceName:     serviceName,
		NeedEnvironment: true,
		NeedService:     true,
	})
	if err != nil {
		return err
	}

	// Get deployments for the service
	var deploymentID string
	if logsDeployment != "" {
		deploymentID = logsDeployment
	} else {
		// Get latest deployment (prefer successful ones)
		deployments, err := client.ListDeployments(ctx.Project.ID, ctx.Environment.ID, ctx.Service.ID, 20)
		if err != nil {
			return fmt.Errorf("failed to list deployments: %w", err)
		}

		if len(deployments) == 0 {
			return fmt.Errorf("no deployments found for service %q", serviceName)
		}

		// Find latest successful deployment first
		for _, d := range deployments {
			if d.Status == "SUCCESS" {
				deploymentID = d.ID
				break
			}
		}

		// If no successful deployment, use the latest one
		if deploymentID == "" {
			deploymentID = deployments[0].ID
			fmt.Fprintf(os.Stderr, "Warning: No successful deployments found, using latest deployment (status: %s)\n", deployments[0].Status)
		}
	}

	// Validate tail
	if logsTail < 1 {
		return fmt.Errorf("--tail must be at least 1")
	}
	if logsTail > 500 {
		logsTail = 500
		fmt.Fprintf(os.Stderr, "Warning: --tail limited to 500\n")
	}

	// Handle follow mode
	if logsFollow {
		return followLogs(client, deploymentID, logsTail)
	}

	// Fetch logs
	logs, err := client.GetDeploymentLogs(deploymentID, logsTail)
	if err != nil {
		return fmt.Errorf("failed to fetch logs: %w", err)
	}

	if len(logs) == 0 {
		fmt.Printf("No logs available for deployment %s\n", deploymentID)
		return nil
	}

	// Print logs
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Showing %d log lines for deployment %s\n\n", len(logs), deploymentID)

	for _, log := range logs {
		timestamp := log.Timestamp.Format(time.RFC3339)
		fmt.Fprintf(w, "%s\t%s\n", timestamp, log.Message)
	}
	w.Flush()

	return nil
}

// followLogs streams logs in real-time by polling for new log entries
func followLogs(client api.APIClient, deploymentID string, initialTail int) error {
	// Set up context with cancellation for Ctrl+C handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful exit
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Fprintf(os.Stderr, "\nStopping log stream...\n")
		cancel()
	}()

	// Initial fetch to show recent logs
	logs, err := client.GetDeploymentLogs(deploymentID, initialTail)
	if err != nil {
		return fmt.Errorf("failed to fetch initial logs: %w", err)
	}

	// Print initial logs
	fmt.Printf("Following logs for deployment %s (Ctrl+C to stop)\n\n", deploymentID)
	var lastTimestamp time.Time
	for _, log := range logs {
		fmt.Printf("%s\t%s\n", log.Timestamp.Format(time.RFC3339), log.Message)
		if log.Timestamp.After(lastTimestamp) {
			lastTimestamp = log.Timestamp
		}
	}

	// Polling interval
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Poll for new logs
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			// Fetch new logs
			newLogs, err := client.GetDeploymentLogs(deploymentID, 100)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to fetch logs: %v\n", err)
				continue
			}

			// Filter and print only new logs
			for _, log := range newLogs {
				if log.Timestamp.After(lastTimestamp) {
					fmt.Printf("%s\t%s\n", log.Timestamp.Format(time.RFC3339), log.Message)
					if log.Timestamp.After(lastTimestamp) {
						lastTimestamp = log.Timestamp
					}
				}
			}
		}
	}
}
