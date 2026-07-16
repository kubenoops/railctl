// Package cmd provides the CLI command structure using Cobra.
package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/kubenoops/railctl/internal/output"
)

// Build info - injected via ldflags at build time.
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

var (
	// Global flags
	token        string
	outputFormat string
	workspace    string
	project      string
	environment  string
	service      string
	debug        bool

	// newAPIClient is a factory function for creating API clients.
	// It can be overridden in tests for dependency injection.
	newAPIClient = func(tkn string) api.APIClient {
		client := api.NewClient(tkn)
		client.Debug = debug
		client.Workspace = getWorkspace()
		client.WarnFn = func(msg string) { fmt.Fprintln(os.Stderr, msg) }
		return client
	}
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "railctl",
	Short: "A kubectl-style CLI for Railway",
	Long: `railctl is a stateless CLI for managing Railway resources.

It provides kubectl-inspired commands for managing projects, environments,
services, variables, and deployments via the Railway GraphQL API.

Authentication:
  Set RAILWAY_TOKEN environment variable or use --token flag.
  Token type (account, workspace, or project) is detected automatically.

Workspace selection:
  When your token has access to multiple workspaces, specify one with
  -w <name> or RAILCTL_WORKSPACE=<name>. Single-workspace tokens are
  auto-detected.

Examples:
  railctl get projects -w my-team
  railctl get projects -o json
  railctl describe project my-app -w my-team
  railctl get services -p my-app -e production`,
	SilenceUsage: true,
	// Errors are printed once by Execute() below — without this, cobra prints
	// its own "Error: …" line first and every failure appears twice.
	SilenceErrors: true,
	Version:       version,
	// PersistentPreRunE runs before every subcommand's RunE. It publishes the
	// resolved output mode so cmdutil's advisory hints fire only in text mode
	// (never when emitting json/yaml a caller may be piping).
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		format, err := getOutputFormat()
		if err != nil {
			return err
		}
		cmdutil.OutputIsText = format == output.FormatTable
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	// Customize error output to show error first, then usage
	rootCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		fmt.Fprintf(os.Stderr, "\n❌ Error: %v\n\n", err)
		cmd.Usage()
		return err
	})

	if err := rootCmd.Execute(); err != nil {
		code := 1
		var ec *exitCodeError
		if errors.As(err, &ec) {
			code = ec.code
		}
		// A silent exitCodeError (nil err) exits without printing.
		if ec == nil || ec.err != nil {
			fmt.Fprintf(os.Stderr, "\n❌ Error: %v\n", err)
		}
		os.Exit(code)
	}
}

// exitCodeError lets command handlers request a specific process exit code
// instead of calling os.Exit. Execute() prints the wrapped error as usual;
// a nil err exits silently (e.g. diff --exit-code reporting changes).
type exitCodeError struct {
	code int
	err  error
}

func (e *exitCodeError) Error() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *exitCodeError) Unwrap() error { return e.err }

func init() {
	// Global persistent flags (available to all subcommands)
	rootCmd.PersistentFlags().StringVar(&token, "token", "",
		"Railway API token (default: RAILWAY_TOKEN env var)")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table",
		fmt.Sprintf("Output format: %v", output.ValidFormats()))
	rootCmd.PersistentFlags().StringVarP(&workspace, "workspace", "w", "",
		"Workspace name (default: RAILCTL_WORKSPACE env var)")
	rootCmd.PersistentFlags().StringVarP(&project, "project", "p", "",
		"Project name (default: RAILCTL_PROJECT env var)")
	rootCmd.PersistentFlags().StringVarP(&environment, "environment", "e", "",
		"Environment name (default: RAILCTL_ENVIRONMENT env var)")
	rootCmd.PersistentFlags().StringVarP(&service, "service", "s", "",
		"Service name (default: RAILCTL_SERVICE env var)")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false,
		"Enable debug logging (shows GraphQL requests/responses)")

	// Customize version template
	rootCmd.SetVersionTemplate(fmt.Sprintf("railctl version {{.Version}} (commit: %s, built: %s)\n", commit, date))
}

// getToken returns the API token from flag or environment variable.
func getToken() (string, error) {
	if token != "" {
		return token, nil
	}
	if envToken := os.Getenv("RAILWAY_TOKEN"); envToken != "" {
		return envToken, nil
	}
	return "", fmt.Errorf("no API token provided. Set RAILWAY_TOKEN environment variable or use --token flag")
}

// getWorkspace returns the workspace name from flag or environment variable.
func getWorkspace() string {
	if workspace != "" {
		return workspace
	}
	return os.Getenv("RAILCTL_WORKSPACE")
}

// getProject returns the project name from flag or environment variable.
func getProject() string {
	if project != "" {
		return project
	}
	return os.Getenv("RAILCTL_PROJECT")
}

// getEnvironment returns the environment name from flag or environment variable.
func getEnvironment() string {
	if environment != "" {
		return environment
	}
	return os.Getenv("RAILCTL_ENVIRONMENT")
}

// getService returns the service name from flag or environment variable.
func getService() string {
	if service != "" {
		return service
	}
	return os.Getenv("RAILCTL_SERVICE")
}

// getOutputFormat returns the parsed output format.
func getOutputFormat() (output.Format, error) {
	return output.ParseFormat(outputFormat)
}

func init2() {
	// Add all subcommands
	rootCmd.AddCommand(getCmd)
	rootCmd.AddCommand(describeCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(logsServiceCmd)
}
