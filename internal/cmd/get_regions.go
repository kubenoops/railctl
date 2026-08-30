package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/kubenoops/railctl/internal/output"
	"github.com/kubenoops/railctl/internal/types"
	"github.com/spf13/cobra"
)

var getRegionsCmd = &cobra.Command{
	Use:     "regions",
	Aliases: []string{"region"},
	Short:   "List available Railway regions",
	Long: `List the regions a service can be deployed to.

Use a region's NAME (e.g. us-west1) with 'create service --region',
'update service --region', or 'deploy.region' in a config manifest.

Regions are account/workspace scoped; no project or environment is required.`,
	Example: `  railctl get regions
  railctl get regions -o wide
  railctl get regions -o json`,
	RunE: runGetRegions,
}

func init() {
	getCmd.AddCommand(getRegionsCmd)
}

func runGetRegions(cmd *cobra.Command, args []string) error {
	format, err := getOutputFormat()
	if err != nil {
		return err
	}

	token, err := getToken()
	if err != nil {
		return err
	}

	client := newAPIClient(token)

	regions, err := client.ListRegions()
	if err != nil {
		return fmt.Errorf("failed to list regions: %w", err)
	}

	printer := output.NewPrinter(format)
	if printer.IsStructured() {
		if format == output.FormatJSON {
			return printer.PrintJSON(regions)
		}
		return printer.PrintYAML(regions)
	}

	if len(regions) == 0 {
		fmt.Println("No regions found")
		return nil
	}

	if format == output.FormatWide {
		printRegionsWideTable(regions)
	} else {
		printRegionsTable(regions)
	}
	return nil
}

func printRegionsTable(regions []types.Region) {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tLOCATION")
	for _, r := range regions {
		fmt.Fprintf(w, "%s\t%s\n", r.Name, r.Location)
	}
	w.Flush()
}

func printRegionsWideTable(regions []types.Region) {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tLOCATION\tCOUNTRY")
	for _, r := range regions {
		fmt.Fprintf(w, "%s\t%s\t%s\n", r.Name, r.Location, r.Country)
	}
	w.Flush()
}
