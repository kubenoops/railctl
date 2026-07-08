package cmd

import (
	"fmt"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/kubenoops/railctl/internal/output"
	"github.com/spf13/cobra"
)

var getDomainsCmd = &cobra.Command{
	Use:     "domains",
	Aliases: []string{"domain"},
	Short:   "List domains for a service",
	Long: `List all domains attached to a service in an environment: both
Railway-generated domains (*.up.railway.app) and user-owned custom domains.

For custom domains the STATUS column shows the DNS verification state
(verified or pending). Railway-generated domains need no verification.`,
	Example: `  railctl get domains -s api -p my-project -e production
  railctl get domains -s api -p my-project -e production -o json
  railctl get domains -s api -p my-project -e production -o wide`,
	RunE: runGetDomains,
}

func init() {
	getCmd.AddCommand(getDomainsCmd)
}

func runGetDomains(cmd *cobra.Command, args []string) error {
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

	domains, err := client.ListDomains(ctx.Project.ID, ctx.Environment.ID, ctx.Service.ID)
	if err != nil {
		return err
	}

	rows := domainsToRows(domains)

	return cmdutil.PrintResult(
		format,
		rows,
		domainsToTable(rows),
		domainsToWideTable(rows),
		"No domains found.",
	)
}

// domainRow is the structured output for domain listing. Type is "railway"
// for Railway-generated service domains and "custom" for user-owned domains.
type domainRow struct {
	Type       string `json:"type" yaml:"type"`
	Domain     string `json:"domain" yaml:"domain"`
	TargetPort *int   `json:"targetPort,omitempty" yaml:"targetPort,omitempty"`
	Status     string `json:"status,omitempty" yaml:"status,omitempty"`
	ID         string `json:"id" yaml:"id"`
}

// domainsToRows flattens a DomainList into display rows: Railway-generated
// domains first, then custom domains with their verification status.
func domainsToRows(domains api.DomainList) []domainRow {
	rows := make([]domainRow, 0, len(domains.ServiceDomains)+len(domains.CustomDomains))
	for _, d := range domains.ServiceDomains {
		rows = append(rows, domainRow{
			Type:       "railway",
			Domain:     d.Domain,
			TargetPort: d.TargetPort,
			ID:         d.ID,
		})
	}
	for _, d := range domains.CustomDomains {
		rows = append(rows, domainRow{
			Type:       "custom",
			Domain:     d.Domain,
			TargetPort: d.TargetPort,
			Status:     customDomainStatus(d),
			ID:         d.ID,
		})
	}
	return rows
}

// customDomainStatus renders the DNS verification state of a custom domain.
func customDomainStatus(d api.CustomDomain) string {
	if d.Status == nil {
		return ""
	}
	if d.Status.Verified {
		return "verified"
	}
	return "pending"
}

func domainsToTable(rows []domainRow) *output.Table {
	table := output.NewTable("TYPE", "DOMAIN", "PORT", "STATUS")
	for _, r := range rows {
		table.AddRow(r.Type, r.Domain, formatPort(r.TargetPort), formatDash(r.Status))
	}
	return table
}

func domainsToWideTable(rows []domainRow) *output.Table {
	table := output.NewTable("TYPE", "DOMAIN", "PORT", "STATUS", "ID")
	for _, r := range rows {
		table.AddRow(r.Type, r.Domain, formatPort(r.TargetPort), formatDash(r.Status), r.ID)
	}
	return table
}

// formatPort renders a nullable target port for table output.
func formatPort(port *int) string {
	if port == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *port)
}

// formatDash substitutes "-" for empty table cells.
func formatDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
