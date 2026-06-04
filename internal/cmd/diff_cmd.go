package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/kubenoops/railctl/internal/config"
	"github.com/kubenoops/railctl/internal/diff"
)

var (
	diffFile    string
	diffPrune   bool
	diffNoColor bool
)

var diffCmd = &cobra.Command{
	Use:   "diff -f <file-or-directory>",
	Short: "Show what would change without applying",
	Long: `Compare a YAML config file against the current Railway state and show the differences.

Exits with code 0 if no changes, 1 if there are changes (useful for CI/CD pipelines).`,
	Example: `  # Show diff for a config file
  railctl diff -f service.yaml -p my-project -e production

  # Show diff for a directory of configs
  railctl diff -f configs/ -p my-project -e production

  # Include unmanaged resources in diff
  railctl diff -f stack.yaml --prune`,
	RunE: runDiff,
}

func init() {
	diffCmd.Flags().StringVarP(&diffFile, "file", "f", "", "Path to YAML config file or directory (required)")
	diffCmd.Flags().BoolVar(&diffPrune, "prune", false, "Include unmanaged resources in diff")
	diffCmd.Flags().BoolVar(&diffNoColor, "no-color", false, "Disable colored output")
	_ = diffCmd.MarkFlagRequired("file")
	rootCmd.AddCommand(diffCmd)
}

func runDiff(cmd *cobra.Command, args []string) error {
	// 1. Load config.
	cfg, err := loadConfig(diffFile)
	if err != nil {
		return err
	}

	// 2. Expand env refs.
	if err := config.ExpandConfigEnvRefs(cfg); err != nil {
		return fmt.Errorf("expanding environment references: %w", err)
	}

	// 3. Resolve project/environment.
	tkn, err := getToken()
	if err != nil {
		return err
	}
	client := newAPIClient(tkn)

	projectName := getProject()
	if projectName == "" {
		projectName = cfg.Project
	}
	envName := getEnvironment()
	if envName == "" {
		envName = cfg.Environment
	}

	ctx, err := cmdutil.ResolveContext(client, cmdutil.ResolveOpts{
		ProjectName:     projectName,
		EnvironmentName: envName,
		NeedEnvironment: true,
	})
	if err != nil {
		return err
	}

	projectID := ctx.Project.ID
	envID := ctx.Environment.ID

	// 4. Fetch live state.
	liveServices, err := fetchLiveState(client, projectID, envID)
	if err != nil {
		return fmt.Errorf("fetching live state: %w", err)
	}

	// 5. Compute diff.
	cs := diff.Compute(cfg.Services, liveServices, diffPrune)

	// 6. Render diff with colors.
	useColor := diff.IsColorSupported(os.Stdout) && !diffNoColor
	diff.Render(cs, os.Stdout, useColor)

	// 7. Print summary.
	fmt.Fprintf(os.Stdout, "\n%s\n", cs.Summary())

	// 8. If changes exist, return error so rootCmd exits with code 1.
	if cs.HasChanges() {
		return fmt.Errorf("differences detected")
	}

	return nil
}
