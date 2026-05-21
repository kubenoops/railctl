package cmd

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/kubenoops/railctl/internal/output"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var getVariablesCmd = &cobra.Command{
	Use:     "variables",
	Aliases: []string{"vars", "var", "v"},
	Short:   "List environment variables for a service",
	Long: `List all environment variables configured for a service in an environment.

Variables can be scoped to a specific service within an environment. This command
retrieves all variables for the specified service.

Sensitive values (keys containing KEY, SECRET, PASSWORD, TOKEN, etc.) are masked
by default. Use --show-values to reveal them.

Required flags can be provided via environment variables:
  --project, -p     or RAILCTL_PROJECT
  --environment, -e or RAILCTL_ENVIRONMENT
  --service, -s     or RAILCTL_SERVICE`,
	Example: `  # List variables for a service (sensitive values masked)
  railctl get variables -p myproject -e production -s web

  # Show all values including sensitive ones
  railctl get variables -p myproject -e production -s web --show-values

  # List variables in JSON format
  railctl get variables -p myproject -e production -s web -o json

  # List variables in YAML format
  railctl get variables -p myproject -e production -s web -o yaml

  # Using environment variables
  export RAILCTL_PROJECT=myproject
  export RAILCTL_ENVIRONMENT=production
  export RAILCTL_SERVICE=web
  railctl get variables`,
	RunE: runGetVariables,
}

var getVarsShowValues bool

func init() {
	getVariablesCmd.Flags().BoolVar(&getVarsShowValues, "show-values", false, "Show sensitive variable values (masked by default)")
	getCmd.AddCommand(getVariablesCmd)
}

func runGetVariables(cmd *cobra.Command, args []string) error {
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

	// Get variables (values)
	variables, err := client.GetVariables(ctx.Project.ID, ctx.Environment.ID, ctx.Service.ID)
	if err != nil {
		return fmt.Errorf("failed to get variables: %w", err)
	}

	// Get sealed status
	sealedMap, err := client.GetSealedVariables(ctx.Environment.ID, ctx.Service.ID)
	if err != nil {
		// Non-fatal - continue without sealed info
		sealedMap = make(map[string]bool)
	}

	// Merge sealed variables that don't have values (sealed vars are excluded from GetVariables)
	for name, isSealed := range sealedMap {
		if isSealed {
			if _, exists := variables[name]; !exists {
				variables[name] = "" // Sealed variables have no readable value
			}
		}
	}

	// Format output
	switch format {
	case output.FormatJSON:
		result := buildVarsWithSealed(variables, sealedMap, getVarsShowValues)
		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(out))
	case output.FormatYAML:
		result := buildVarsWithSealed(variables, sealedMap, getVarsShowValues)
		out, _ := yaml.Marshal(result)
		fmt.Print(string(out))
	default:
		printVariablesTable(variables, sealedMap, ctx.Service.Name, ctx.Environment.Name, getVarsShowValues)
	}

	return nil
}

type varWithSealed struct {
	Value    string `json:"value" yaml:"value"`
	IsSealed bool   `json:"isSealed,omitempty" yaml:"isSealed,omitempty"`
}

func buildVarsWithSealed(variables map[string]string, sealedMap map[string]bool, showValues bool) map[string]varWithSealed {
	result := make(map[string]varWithSealed)
	for k, v := range variables {
		value := v
		if sealedMap[k] {
			value = "[SEALED]"
		} else if !showValues && api.IsSensitiveKey(k) {
			value = api.MaskValue(v)
		}
		result[k] = varWithSealed{Value: value, IsSealed: sealedMap[k]}
	}
	return result
}

func printVariablesTable(variables map[string]string, sealedMap map[string]bool, serviceName, envName string, showValues bool) {
	if len(variables) == 0 {
		fmt.Printf("No variables found for service '%s' in environment '%s'\n", serviceName, envName)
		return
	}

	// Sort keys for consistent output
	keys := make([]string, 0, len(variables))
	for k := range variables {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fmt.Printf("Variables for service '%s' in environment '%s':\n\n", serviceName, envName)
	fmt.Printf("%-30s %s\n", "KEY", "VALUE")
	fmt.Printf("%-30s %s\n", "---", "-----")
	for _, key := range keys {
		value := variables[key]
		if sealedMap[key] {
			value = "[SEALED]"
		} else if !showValues && api.IsSensitiveKey(key) {
			value = api.MaskValue(value)
		} else if len(value) > 50 {
			value = value[:47] + "..."
		}
		fmt.Printf("%-30s %s\n", key, value)
	}

	sealedCount := 0
	for _, key := range keys {
		if sealedMap[key] {
			sealedCount++
		}
	}
	if sealedCount > 0 {
		fmt.Printf("\nTotal: %d variable(s), %d sealed\n", len(variables), sealedCount)
	} else {
		fmt.Printf("\nTotal: %d variable(s)\n", len(variables))
	}
	if !showValues {
		fmt.Printf("\n  (Sensitive values masked. Use --show-values to reveal)\n")
	}
}
