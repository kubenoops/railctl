package cmd

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/kubenoops/railctl/internal/output"
	"github.com/spf13/cobra"
)

var getReplicasCmd = &cobra.Command{
	Use:     "replicas",
	Aliases: []string{"replica"},
	Short:   "List the running replicas of a service",
	Long: `List the running replicas (deployment instances) of a service.

Each replica's ID is a deployment-instance id you can pass to 'railctl exec' or
'railctl port-forward' via --deployment-instance to target that exact replica,
instead of letting Railway's relay pick one for you.

Requires project, environment, and service to be specified via flags or
environment variables.`,
	Example: `  railctl get replicas -p my-project -e production -s api
  railctl get replicas -p my-project -e production -s api -o json
  railctl get replicas -p my-project -e production -s api -o wide`,
	RunE: runGetReplicas,
}

func init() {
	getCmd.AddCommand(getReplicasCmd)
}

func runGetReplicas(cmd *cobra.Command, args []string) error {
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
		ServiceName:     getService(),
		NeedEnvironment: true,
		NeedService:     true,
	})
	if err != nil {
		return err
	}

	list, err := client.ListReplicas(ctx.Environment.ID, ctx.Service.ID)
	if err != nil {
		return fmt.Errorf("failed to list replicas: %w", err)
	}

	printer := output.NewPrinter(format)

	// Structured formats emit the whole ReplicaList (deployment context +
	// replicas) so consumers get the full machine-readable picture.
	if printer.IsStructured() {
		if format == output.FormatJSON {
			return printer.PrintJSON(list)
		}
		return printer.PrintYAML(list)
	}

	if len(list.Replicas) == 0 {
		fmt.Println("No replicas running for this service")
		return nil
	}

	printReplicasHeader(list)
	if format == output.FormatWide {
		printReplicasWideTable(list)
	} else {
		printReplicasTable(list.Replicas)
	}
	return nil
}

// printReplicasHeader prints the parent deployment context and a per-status
// count summary above the table, so the list reads as service health at a
// glance (e.g. "5 replicas: 4 RUNNING, 1 CRASHED").
func printReplicasHeader(list api.ReplicaList) {
	if list.DeploymentID != "" {
		fmt.Printf("Deployment %s (%s) — %s\n\n",
			list.DeploymentID, list.DeploymentStatus, replicaSummary(list.Replicas))
	}
}

// replicaSummary renders "N replicas: X RUNNING, Y CRASHED" with statuses in
// stable alphabetical order for deterministic output.
func replicaSummary(replicas []api.Replica) string {
	counts := make(map[string]int)
	for _, r := range replicas {
		counts[r.Status]++
	}
	statuses := make([]string, 0, len(counts))
	for s := range counts {
		statuses = append(statuses, s)
	}
	sort.Strings(statuses)

	summary := ""
	for i, s := range statuses {
		if i > 0 {
			summary += ", "
		}
		summary += fmt.Sprintf("%d %s", counts[s], s)
	}
	noun := "replicas"
	if len(replicas) == 1 {
		noun = "replica"
	}
	return fmt.Sprintf("%d %s: %s", len(replicas), noun, summary)
}

func printReplicasTable(replicas []api.Replica) {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "INSTANCE ID\tSTATUS")
	for _, r := range replicas {
		fmt.Fprintf(w, "%s\t%s\n", r.ID, r.Status)
	}
	w.Flush()
}

func printReplicasWideTable(list api.ReplicaList) {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "INSTANCE ID\tSTATUS\tDEPLOYMENT ID")
	for _, r := range list.Replicas {
		fmt.Fprintf(w, "%s\t%s\t%s\n", r.ID, r.Status, list.DeploymentID)
	}
	w.Flush()
}
