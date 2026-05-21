package cmd

import (
	"fmt"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/kubenoops/railctl/internal/output"
	"github.com/kubenoops/railctl/internal/types"
	"github.com/spf13/cobra"
)

var getServicesCmd = &cobra.Command{
	Use:     "services",
	Aliases: []string{"service", "svc"},
	Short:   "List services in an environment",
	Example: `  railctl get services -p my-project -e production
  railctl get services -p my-project -e production -o json
  railctl get services -p my-project -e production -o wide`,
	RunE: runGetServices,
}

func init() {
	getCmd.AddCommand(getServicesCmd)
}

func runGetServices(cmd *cobra.Command, args []string) error {
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

	services, err := client.ListServices(ctx.Project.ID, ctx.Environment.ID)
	if err != nil {
		return err
	}

	enrichServicesForRichOutput(client, format, ctx.Project.ID, ctx.Environment.ID, services)

	return cmdutil.PrintResult(
		format,
		servicesToOutput(services),
		servicesToTable(services),
		servicesToWideTable(services),
		"No services found.",
	)
}

func enrichServicesForRichOutput(client api.APIClient, format output.Format, projectID, environmentID string, services []types.ServiceDetail) {
	if format != output.FormatWide && format != output.FormatJSON && format != output.FormatYAML {
		return
	}

	for i := range services {
		domains, err := client.ListDomains(projectID, environmentID, services[i].ID)
		if err == nil {
			services[i].ServiceDomains = mapServiceDomains(domains.ServiceDomains)
			services[i].CustomDomains = mapCustomDomains(domains.CustomDomains)
		}

		tcpProxies, err := client.ListTCPProxies(environmentID, services[i].ID)
		if err == nil {
			services[i].TCPProxies = mapTCPProxies(tcpProxies)
		}
	}
}

func summarizeServiceDomain(svc types.ServiceDetail) string {
	if len(svc.ServiceDomains) > 0 {
		return svc.ServiceDomains[0].Domain
	}
	if len(svc.CustomDomains) > 0 {
		return svc.CustomDomains[0].Domain
	}
	return "-"
}

func summarizeTCPProxy(svc types.ServiceDetail) string {
	if len(svc.TCPProxies) == 0 {
		return "-"
	}

	proxy := svc.TCPProxies[0]
	if proxy.Domain == "" || proxy.ProxyPort == 0 {
		return "-"
	}
	return fmt.Sprintf("%s:%d", proxy.Domain, proxy.ProxyPort)
}

// serviceOutput is the structured output for service listing.
type serviceOutput struct {
	Name           string                `json:"name" yaml:"name"`
	ID             string                `json:"id" yaml:"id"`
	Source         string                `json:"source,omitempty" yaml:"source,omitempty"`
	SourceType     string                `json:"sourceType,omitempty" yaml:"sourceType,omitempty"`
	Status         string                `json:"status,omitempty" yaml:"status,omitempty"`
	UpdatedAt      string                `json:"updatedAt,omitempty" yaml:"updatedAt,omitempty"`
	ServiceDomains []types.ServiceDomain `json:"serviceDomains,omitempty" yaml:"serviceDomains,omitempty"`
	CustomDomains  []types.CustomDomain  `json:"customDomains,omitempty" yaml:"customDomains,omitempty"`
	TCPProxies     []types.TCPProxy      `json:"tcpProxies,omitempty" yaml:"tcpProxies,omitempty"`
}

func servicesToOutput(services []types.ServiceDetail) []serviceOutput {
	result := make([]serviceOutput, len(services))
	for i, svc := range services {
		result[i] = serviceOutput{
			Name:           svc.Name,
			ID:             svc.ID,
			Source:         svc.Source,
			SourceType:     svc.SourceType,
			Status:         svc.Status,
			ServiceDomains: svc.ServiceDomains,
			CustomDomains:  svc.CustomDomains,
			TCPProxies:     svc.TCPProxies,
		}
		if !svc.UpdatedAt.IsZero() {
			result[i].UpdatedAt = svc.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
		}
	}
	return result
}

func servicesToTable(services []types.ServiceDetail) *output.Table {
	table := output.NewTable("NAME", "SOURCE", "STATUS", "UPDATED")
	for _, svc := range services {
		source := "-"
		if svc.Source != "" {
			source = svc.Source
		}
		status := "-"
		if svc.Status != "" {
			status = svc.Status
		}
		updated := "-"
		if !svc.UpdatedAt.IsZero() {
			updated = types.RelativeTime(svc.UpdatedAt)
		}
		table.AddRow(svc.Name, source, status, updated)
	}
	return table
}

func servicesToWideTable(services []types.ServiceDetail) *output.Table {
	table := output.NewTable("NAME", "ID", "SOURCE", "TYPE", "STATUS", "DOMAIN", "TCP", "UPDATED")
	for _, svc := range services {
		id := svc.ID
		if len(id) > 12 {
			id = id[:12]
		}
		source := "-"
		if svc.Source != "" {
			source = truncate(svc.Source, 40)
		}
		sourceType := "-"
		if svc.SourceType != "" {
			sourceType = svc.SourceType
		}
		status := "-"
		if svc.Status != "" {
			status = svc.Status
		}
		updated := "-"
		if !svc.UpdatedAt.IsZero() {
			updated = types.RelativeTime(svc.UpdatedAt)
		}
		table.AddRow(svc.Name, id, source, sourceType, status, summarizeServiceDomain(svc), summarizeTCPProxy(svc), updated)
	}
	return table
}

// truncate shortens a string to the specified length, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
