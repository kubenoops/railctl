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
	diffFile     string
	diffPrune    bool
	diffNoColor  bool
	diffColor    bool
	diffExitCode bool
)

var diffCmd = &cobra.Command{
	Use:   "diff -f <file-or-directory>",
	Short: "Show what would change without applying",
	Long: `Compare a YAML config file against the current Railway state and show the differences.

Exits 0 whether or not there are changes (a diff with changes is a report,
not a failure). With --exit-code, exits 1 when there are changes, 0 when the
live state matches the manifest, and 2 on a real error (bad file, auth, API).
Combined with --prune, unmanaged live resources count as changes too.`,
	Example: `  # Show diff for a config file
  railctl diff -f service.yaml -p my-project -e production

  # Show diff for a directory of configs
  railctl diff -f configs/ -p my-project -e production

  # Include unmanaged resources in diff
  railctl diff -f stack.yaml --prune

  # Gate CI on drift: exit 1 on changes, 0 when in sync, 2 on error
  railctl diff -f stack.yaml --exit-code`,
	RunE: runDiff,
}

func init() {
	diffCmd.Flags().StringVarP(&diffFile, "file", "f", "", "Path to YAML config file or directory (required)")
	diffCmd.Flags().BoolVar(&diffPrune, "prune", false, "Include unmanaged resources in diff")
	diffCmd.Flags().BoolVar(&diffNoColor, "no-color", false, "Disable colored output")
	diffCmd.Flags().BoolVar(&diffColor, "color", false, "Force colored output even when not a terminal (e.g. CI)")
	diffCmd.Flags().BoolVar(&diffExitCode, "exit-code", false, "Exit 1 if there are changes, 0 if none, 2 on error")
	_ = diffCmd.MarkFlagRequired("file")
	rootCmd.AddCommand(diffCmd)
}

func runDiff(cmd *cobra.Command, args []string) error {
	cs, err := computeDiffChangeSet()
	if err != nil {
		if diffExitCode {
			// Exit 1 is reserved for "changes found".
			return &exitCodeError{code: 2, err: err}
		}
		return err
	}

	useColor := !diffNoColor && (diffColor || diff.IsColorSupported(os.Stdout))
	diff.Render(cs, os.Stdout, useColor)

	fmt.Fprintf(os.Stdout, "\n%s\n", cs.Summary())

	if diffExitCode && cs.HasChanges() {
		// Silent: the rendered diff is the output.
		return &exitCodeError{code: 1}
	}

	// Without --exit-code a diff with changes is a report, not a failure.
	return nil
}

// computeDiffChangeSet loads the manifest, fetches live state, and computes
// the change set.
func computeDiffChangeSet() (*diff.ChangeSet, error) {
	cfg, err := loadConfig(diffFile)
	if err != nil {
		return nil, err
	}

	if err := config.ExpandConfigEnvRefs(cfg); err != nil {
		return nil, fmt.Errorf("expanding environment references: %w", err)
	}

	tkn, err := getToken()
	if err != nil {
		return nil, err
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
		return nil, err
	}

	projectID := ctx.Project.ID
	envID := ctx.Environment.ID

	liveServices, err := fetchLiveState(client, projectID, envID, cfg.Services)
	if err != nil {
		return nil, fmt.Errorf("fetching live state: %w", err)
	}

	cs := diff.Compute(cfg.Services, liveServices, diffPrune)

	// Environment-level deleteProtection: only read live state when the
	// manifest declares the field — an omitted field is left alone (nil).
	if cfg.DeleteProtection != nil {
		liveProtected, err := cmdutil.EnvironmentIsProtected(client, projectID, envID)
		if err != nil {
			return nil, fmt.Errorf("reading delete protection: %w", err)
		}
		cs.Environment = diff.ComputeEnvironment(cfg.DeleteProtection, liveProtected)
	}

	return cs, nil
}
