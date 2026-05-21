package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/kubenoops/railctl/internal/output"
	"github.com/kubenoops/railctl/internal/types"
	"github.com/spf13/cobra"
)

var getDeploymentsCmd = &cobra.Command{
	Use:     "deployments",
	Aliases: []string{"deploy", "deploys", "dep"},
	Short:   "List deployments for a service",
	Long: `List recent deployments for a service, showing their ID, status, creator, and time.

Requires project, environment, and service to be specified via flags or environment variables.`,
	Example: `  railctl get deployments -p my-project -e production -s api
  railctl get deployments -o json
  railctl get deployments --limit 5`,
	RunE: runGetDeployments,
}

var deploymentsLimit int

func init() {
	getDeploymentsCmd.Flags().IntVar(&deploymentsLimit, "limit", 10, "Maximum number of deployments to show")
	getCmd.AddCommand(getDeploymentsCmd)
}

func runGetDeployments(cmd *cobra.Command, args []string) error {
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

	deployments, err := client.ListDeployments(ctx.Project.ID, ctx.Environment.ID, ctx.Service.ID, deploymentsLimit)
	if err != nil {
		return fmt.Errorf("failed to list deployments: %w", err)
	}

	if len(deployments) == 0 {
		fmt.Println("No deployments found")
		return nil
	}

	printer := output.NewPrinter(format)

	if printer.IsStructured() {
		if format == output.FormatJSON {
			return printer.PrintJSON(deployments)
		}
		return printer.PrintYAML(deployments)
	}

	if format == output.FormatWide {
		printDeploymentsWideTable(deployments)
	} else {
		printDeploymentsTable(deployments)
	}
	return nil
}

func printDeploymentsTable(deployments []api.Deployment) {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSTATUS\tCREATOR\tCREATED")
	for _, d := range deployments {
		age := types.RelativeTime(d.CreatedAt)
		creator := d.CreatorName
		if creator == "" {
			creator = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", d.ID, d.Status, creator, age)
	}
	w.Flush()
}

func printDeploymentsWideTable(deployments []api.Deployment) {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSTATUS\tIMAGE\tCREATOR\tCREATED")
	for _, d := range deployments {
		age := types.RelativeTime(d.CreatedAt)
		creator := d.CreatorName
		if creator == "" {
			creator = "-"
		}
		image := d.Image
		if image == "" {
			image = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", d.ID, d.Status, image, creator, age)
	}
	w.Flush()
}
