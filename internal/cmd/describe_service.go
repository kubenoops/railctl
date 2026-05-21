package cmd

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/kubenoops/railctl/internal/output"
	"github.com/kubenoops/railctl/internal/types"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	showValuesFlag bool
)

var describeServiceCmd = &cobra.Command{
	Use:     "service <name>",
	Aliases: []string{"svc"},
	Short:   "Show detailed information about a service",
	Args:    cobra.ExactArgs(1),
	Example: `  railctl describe service api -p my-project -e production
  railctl describe service api -p my-project -e production -o json
  railctl describe service api -p my-project -e production --show-values`,
	RunE: runDescribeService,
}

func init() {
	describeServiceCmd.Flags().BoolVar(&showValuesFlag, "show-values", false, "Show sensitive variable values (keys containing KEY, SECRET, PASSWORD, TOKEN, etc. are masked by default)")
	describeCmd.AddCommand(describeServiceCmd)
}

func runDescribeService(cmd *cobra.Command, args []string) error {
	serviceName := args[0]

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
		ServiceName:     serviceName,
		NeedEnvironment: true,
		NeedService:     true,
	})
	if err != nil {
		return err
	}

	targetService := ctx.Service

	// If deployment failed and we don't have an error message, fetch build logs
	if targetService.Status == "FAILED" && targetService.DeploymentError == "" && targetService.DeploymentID != "" {
		logs, err := client.GetBuildLogs(targetService.DeploymentID, 20)
		if err == nil && len(logs) > 0 {
			targetService.DeploymentError = api.ExtractErrorFromLogs(logs)
		}
	}

	// Fetch variables for this service
	variables, err := client.GetVariables(ctx.Project.ID, ctx.Environment.ID, targetService.ID)
	if err != nil {
		// Non-fatal: just show empty variables section
		variables = make(map[string]string)
	}

	// Fetch sealed status
	sealedMap, err := client.GetSealedVariables(ctx.Environment.ID, targetService.ID)
	if err != nil {
		// Non-fatal - continue without sealed info
		sealedMap = make(map[string]bool)
	}

	// Fetch networking metadata
	domains, err := client.ListDomains(ctx.Project.ID, ctx.Environment.ID, targetService.ID)
	if err == nil {
		targetService.ServiceDomains = mapServiceDomains(domains.ServiceDomains)
		targetService.CustomDomains = mapCustomDomains(domains.CustomDomains)
	}

	tcpProxies, err := client.ListTCPProxies(ctx.Environment.ID, targetService.ID)
	if err == nil {
		targetService.TCPProxies = mapTCPProxies(tcpProxies)
	}

	// Merge sealed variables that don't have values
	for name, isSealed := range sealedMap {
		if isSealed {
			if _, exists := variables[name]; !exists {
				variables[name] = ""
			}
		}
	}

	switch format {
	case output.FormatJSON:
		data := serviceDetailToOutput(*targetService, ctx.Project.Name, ctx.Environment.Name, variables, sealedMap, showValuesFlag)
		out, _ := json.MarshalIndent(data, "", "  ")
		fmt.Println(string(out))
	case output.FormatYAML:
		data := serviceDetailToOutput(*targetService, ctx.Project.Name, ctx.Environment.Name, variables, sealedMap, showValuesFlag)
		out, _ := yaml.Marshal(data)
		fmt.Print(string(out))
	default:
		return printServiceDetail(*targetService, ctx.Project.Name, ctx.Environment.Name, variables, sealedMap, showValuesFlag)
	}
	return nil
}

// svcDetailOutput is the structured output for service detail.
type svcDetailOutput struct {
	Name            string                `json:"name" yaml:"name"`
	ID              string                `json:"id" yaml:"id"`
	Project         string                `json:"project" yaml:"project"`
	Environment     string                `json:"environment" yaml:"environment"`
	Source          string                `json:"source,omitempty" yaml:"source,omitempty"`
	SourceType      string                `json:"sourceType,omitempty" yaml:"sourceType,omitempty"`
	StartCommand    string                `json:"startCommand,omitempty" yaml:"startCommand,omitempty"`
	Status          string                `json:"status,omitempty" yaml:"status,omitempty"`
	DeploymentError string                `json:"deploymentError,omitempty" yaml:"deploymentError,omitempty"`
	UpdatedAt       string                `json:"updatedAt,omitempty" yaml:"updatedAt,omitempty"`
	ServiceDomains  []types.ServiceDomain `json:"serviceDomains,omitempty" yaml:"serviceDomains,omitempty"`
	CustomDomains   []types.CustomDomain  `json:"customDomains,omitempty" yaml:"customDomains,omitempty"`
	TCPProxies      []types.TCPProxy      `json:"tcpProxies,omitempty" yaml:"tcpProxies,omitempty"`
	Variables       map[string]string     `json:"variables,omitempty" yaml:"variables,omitempty"`
}

func serviceDetailToOutput(svc types.ServiceDetail, projectName, envName string, variables map[string]string, sealedMap map[string]bool, showValues bool) svcDetailOutput {
	out := svcDetailOutput{
		Name:            svc.Name,
		ID:              svc.ID,
		Project:         projectName,
		Environment:     envName,
		Source:          svc.Source,
		SourceType:      svc.SourceType,
		StartCommand:    svc.StartCommand,
		Status:          svc.Status,
		DeploymentError: svc.DeploymentError,
		ServiceDomains:  svc.ServiceDomains,
		CustomDomains:   svc.CustomDomains,
		TCPProxies:      svc.TCPProxies,
	}
	if !svc.UpdatedAt.IsZero() {
		out.UpdatedAt = svc.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
	}

	// Add variables (masking sensitive values unless --show-values is set)
	if len(variables) > 0 {
		out.Variables = make(map[string]string, len(variables))
		for k, v := range variables {
			if sealedMap[k] {
				out.Variables[k] = "[SEALED]"
			} else if !showValues && api.IsSensitiveKey(k) {
				out.Variables[k] = api.MaskValue(v)
			} else {
				out.Variables[k] = v
			}
		}
	}

	return out
}

// mapServiceDomains converts API service domains to CLI-facing types.
func mapServiceDomains(apiDomains []api.ServiceDomain) []types.ServiceDomain {
	if len(apiDomains) == 0 {
		return nil
	}
	out := make([]types.ServiceDomain, len(apiDomains))
	for i, d := range apiDomains {
		out[i] = types.ServiceDomain{ID: d.ID, Domain: d.Domain, TargetPort: d.TargetPort}
	}
	return out
}

// mapCustomDomains converts API custom domains to CLI-facing types.
func mapCustomDomains(apiDomains []api.CustomDomain) []types.CustomDomain {
	if len(apiDomains) == 0 {
		return nil
	}
	out := make([]types.CustomDomain, len(apiDomains))
	for i, d := range apiDomains {
		out[i] = types.CustomDomain{ID: d.ID, Domain: d.Domain, TargetPort: d.TargetPort}
	}
	return out
}

// mapTCPProxies converts API TCP proxies to CLI-facing types.
func mapTCPProxies(apiProxies []api.TCPProxy) []types.TCPProxy {
	if len(apiProxies) == 0 {
		return nil
	}
	out := make([]types.TCPProxy, len(apiProxies))
	for i, p := range apiProxies {
		out[i] = types.TCPProxy{ID: p.ID, Domain: p.Domain, ProxyPort: p.ProxyPort, ApplicationPort: p.ApplicationPort}
	}
	return out
}

func printServiceDetail(svc types.ServiceDetail, projectName, envName string, variables map[string]string, sealedMap map[string]bool, showValues bool) error {
	fmt.Printf("Name:         %s\n", svc.Name)
	fmt.Printf("ID:           %s\n", svc.ID)
	fmt.Printf("Project:      %s\n", projectName)
	fmt.Printf("Environment:  %s\n", envName)

	if svc.Source != "" {
		fmt.Printf("Source:       %s (%s)\n", svc.Source, svc.SourceType)
	} else {
		fmt.Printf("Source:       (none)\n")
	}

	if svc.StartCommand != "" {
		fmt.Printf("Start Cmd:    %s\n", svc.StartCommand)
	}

	if svc.Status != "" {
		fmt.Printf("Status:       %s\n", svc.Status)
	}

	if svc.DeploymentError != "" {
		fmt.Printf("Error:        %s\n", svc.DeploymentError)
	}

	if !svc.UpdatedAt.IsZero() {
		fmt.Printf("Updated:      %s\n", types.RelativeTime(svc.UpdatedAt))
	}

	// Variables section
	if len(variables) > 0 {
		// Count sealed
		sealedCount := 0
		for k := range variables {
			if sealedMap[k] {
				sealedCount++
			}
		}
		if sealedCount > 0 {
			fmt.Printf("\nVariables:    (%d, %d sealed)\n", len(variables), sealedCount)
		} else {
			fmt.Printf("\nVariables:    (%d)\n", len(variables))
		}

		// Sort keys for consistent output
		keys := make([]string, 0, len(variables))
		for k := range variables {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, key := range keys {
			var value string
			if sealedMap[key] {
				value = "[SEALED]"
			} else {
				value = variables[key]
				if !showValues && api.IsSensitiveKey(key) {
					value = api.MaskValue(value)
				}
				// Truncate long values in table display
				if len(value) > 60 {
					value = value[:57] + "..."
				}
			}
			fmt.Printf("  %-30s %s\n", key, value)
		}

		if !showValues {
			fmt.Printf("\n  (Sensitive values masked. Use --show-values to reveal)\n")
		}
	} else {
		fmt.Printf("\nVariables:    (none)\n")
	}

	return nil
}
