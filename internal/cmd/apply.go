package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/apply"
	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/kubenoops/railctl/internal/config"
	"github.com/kubenoops/railctl/internal/diff"
)

var (
	applyFile         string
	applyDryRun       bool
	applyPrune        bool
	applyPruneYes     bool
	applyAwait        bool
	applyAwaitTimeout int
	applyNoColor      bool
	applyColor        bool
)

var applyCmd = &cobra.Command{
	Use:   "apply -f <file-or-directory>",
	Short: "Apply a declarative config to Railway",
	Long: `Read a YAML config file and create/update Railway resources to match the desired state.

Supports both single-file configs and directories of configs.
Project and environment can be specified in the YAML or via -p/-e flags (flags take precedence).

The config file format supports:
- Multiple services per file
- Environment variable expansion via $env(VAR)
- Railway service references via ${{service.VAR}}
- Legacy single-service configs (auto-detected and converted)

Use --dry-run to preview changes without applying them.
Use --prune to delete services not in the config file.`,
	Example: `  # Apply a single config file
  railctl apply -f service.yaml -p my-project -e production

  # Apply all configs from a directory
  railctl apply -f configs/ -p my-project -e production

  # Preview changes without applying
  railctl apply -f stack.yaml --dry-run

  # Apply and delete services not in the config
  railctl apply -f stack.yaml --prune --yes

  # Apply and wait for deployments to complete
  railctl apply -f stack.yaml --await`,
	RunE: runApply,
}

func init() {
	applyCmd.Flags().StringVarP(&applyFile, "file", "f", "", "Path to YAML config file or directory (required)")
	applyCmd.Flags().BoolVar(&applyDryRun, "dry-run", false, "Show what would change without applying")
	applyCmd.Flags().BoolVar(&applyPrune, "prune", false, "Delete services not in the config file")
	applyCmd.Flags().BoolVar(&applyPruneYes, "yes", false, "Skip confirmation prompt for --prune deletions")
	applyCmd.Flags().BoolVar(&applyAwait, "await", false, "Wait for deployments to reach terminal status")
	applyCmd.Flags().IntVar(&applyAwaitTimeout, "await-timeout", 600, "Timeout in seconds for --await")
	applyCmd.Flags().BoolVar(&applyNoColor, "no-color", false, "Disable colored output")
	applyCmd.Flags().BoolVar(&applyColor, "color", false, "Force colored output even when not a terminal (e.g. CI)")
	_ = applyCmd.MarkFlagRequired("file")
	rootCmd.AddCommand(applyCmd)
}

func runApply(cmd *cobra.Command, args []string) error {
	// 1. Load config from file or directory.
	cfg, err := loadConfig(applyFile)
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
	liveServices, err := fetchLiveState(client, projectID, envID, cfg.Services)
	if err != nil {
		return fmt.Errorf("fetching live state: %w", err)
	}

	// 5. Compute diff.
	cs := diff.Compute(cfg.Services, liveServices, applyPrune)

	// 5b. Environment-level deleteProtection. Only read the live state when the
	// manifest declares the field — an omitted field is left alone (nil), so we
	// avoid an extra shared-variables read in the common case.
	if cfg.DeleteProtection != nil {
		liveProtected, err := cmdutil.EnvironmentIsProtected(client, projectID, envID)
		if err != nil {
			return fmt.Errorf("reading delete protection: %w", err)
		}
		cs.Environment = diff.ComputeEnvironment(cfg.DeleteProtection, liveProtected)
	}

	// Determine color support.
	useColor := !applyNoColor && (applyColor || diff.IsColorSupported(os.Stdout))

	// 6. If dry-run or no changes, render and exit.
	if applyDryRun {
		diff.Render(cs, os.Stdout, useColor)
		fmt.Fprintf(os.Stdout, "\n%s\n", cs.Summary())
		return nil
	}

	if !cs.HasChanges() {
		fmt.Println("No changes needed. Railway state matches the config.")
		return nil
	}

	// 7. Render diff so user sees what will change.
	diff.Render(cs, os.Stdout, useColor)
	fmt.Fprintln(os.Stdout)

	// 8. If --prune will delete services, prompt for confirmation unless --yes.
	if applyPrune && !applyPruneYes {
		hasDeletes := false
		for _, rc := range cs.Changes {
			if rc.Type == diff.ChangeDelete {
				hasDeletes = true
				break
			}
		}
		if hasDeletes {
			fmt.Fprint(os.Stdout, "Prune will delete the services listed above. Are you sure? (y/N): ")
			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer != "y" && answer != "yes" {
				return fmt.Errorf("aborted by user")
			}
		}
	}

	// 9. Build config map for apply.
	configMap := make(map[string]config.ServiceConfig, len(cfg.Services))
	for _, svc := range cfg.Services {
		configMap[svc.Name] = svc
	}

	// 10. Apply changes.
	result := apply.Apply(client, cs, projectID, envID, configMap, apply.Opts{
		DryRun: false,
		Prune:  applyPrune,
		Output: os.Stdout,
	})

	// 11. Print result summary.
	if len(result.Created) > 0 {
		fmt.Printf("Created: %v\n", result.Created)
	}
	if len(result.Updated) > 0 {
		fmt.Printf("Updated: %v\n", result.Updated)
	}
	if len(result.Deleted) > 0 {
		fmt.Printf("Deleted: %v\n", result.Deleted)
	}
	for _, e := range result.Errors {
		fmt.Fprintf(os.Stderr, "Error: %v\n", e)
	}
	if len(result.Errors) > 0 {
		return fmt.Errorf("%d error(s) during apply", len(result.Errors))
	}

	// 12. If --await and there are created/updated services, await deployments.
	if applyAwait && (len(result.Created) > 0 || len(result.Updated) > 0) {
		// Re-fetch services to get deployment IDs.
		services, err := client.ListServices(projectID, envID)
		if err != nil {
			return fmt.Errorf("fetching services for await: %w", err)
		}

		awaitNames := make(map[string]bool)
		for _, name := range result.Created {
			awaitNames[name] = true
		}
		for _, name := range result.Updated {
			awaitNames[name] = true
		}

		for _, svc := range services {
			if !awaitNames[svc.Name] {
				continue
			}
			if svc.DeploymentID == "" {
				continue
			}
			if err := awaitDeployment(client, projectID, envID, svc.ID, svc.DeploymentID, svc.Name, applyAwaitTimeout); err != nil {
				return err
			}
		}
	}

	return nil
}

// loadConfig loads a Config from a file path or directory.
func loadConfig(path string) (*config.Config, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("accessing config path: %w", err)
	}

	if info.IsDir() {
		return config.LoadDir(path)
	}
	return config.Load(path)
}

// fetchLiveState retrieves the current state of all services in the given
// project/environment from the Railway API and returns them as LiveService structs.
// desired is the declared config; it is used to limit backup-schedule reads to
// volumes actually under declarative management (services declaring a volume).
func fetchLiveState(client api.APIClient, projectID, envID string, desired []config.ServiceConfig) ([]diff.LiveService, error) {
	// Services that declare a volume — only these need their live backup
	// schedules read (avoids an N+1 for volumes not under management).
	managesVolume := make(map[string]bool, len(desired))
	for _, d := range desired {
		if d.Volume.MountPath != "" {
			managesVolume[d.Name] = true
		}
	}

	// Get all services.
	services, err := client.ListServices(projectID, envID)
	if err != nil {
		return nil, fmt.Errorf("listing services: %w", err)
	}

	// Get all volumes for this environment (one call, filter per-service).
	volumes, err := client.ListVolumes(projectID, envID)
	if err != nil {
		return nil, fmt.Errorf("listing volumes: %w", err)
	}

	var liveServices []diff.LiveService

	for _, svc := range services {
		ls := diff.LiveService{
			Name:  svc.Name,
			Image: svc.Source,
			Deploy: diff.LiveDeployConfig{
				StartCommand:       svc.StartCommand,
				RestartPolicy:      svc.RestartPolicy,
				MaxRetries:         svc.MaxRetries,
				Replicas:           svc.Replicas,
				HealthcheckPath:    svc.HealthcheckPath,
				HealthcheckTimeout: svc.HealthcheckTimeout,
			},
		}

		// Variables. Use raw (unrendered) values so ${{...}} references compare
		// against the config template instead of Railway's resolved value.
		vars, err := client.GetRawVariables(projectID, envID, svc.ID)
		if err != nil {
			return nil, fmt.Errorf("getting variables for service %q: %w", svc.Name, err)
		}
		ls.Variables = vars

		// Volumes: filter VolumeInstances for this service.
		for _, vi := range volumes {
			if vi.ServiceID != nil && *vi.ServiceID == svc.ID {
				lv := diff.LiveVolume{
					MountPath:        vi.MountPath,
					VolumeInstanceID: vi.ID,
				}
				// Only read schedules for volumes under declarative management.
				// Warn (don't fail) on error so a transient read doesn't present
				// a degraded state as truth.
				if managesVolume[svc.Name] {
					if schedules, err := client.ListVolumeBackupSchedules(vi.ID); err == nil {
						for _, s := range schedules {
							lv.BackupSchedules = append(lv.BackupSchedules, s.Kind)
						}
					} else {
						fmt.Fprintf(os.Stderr, "Warning: could not read backup schedules for volume %q; treating as none: %v\n", vi.Volume.Name, err)
					}
				}
				ls.Volumes = append(ls.Volumes, lv)
			}
		}

		// Domains.
		domainList, err := client.ListDomains(projectID, envID, svc.ID)
		if err != nil {
			return nil, fmt.Errorf("listing domains for service %q: %w", svc.Name, err)
		}
		for _, sd := range domainList.ServiceDomains {
			port := 0
			if sd.TargetPort != nil {
				port = *sd.TargetPort
			}
			ls.Domains = append(ls.Domains, diff.LiveDomain{
				Domain: sd.Domain,
				Port:   port,
			})
		}
		for _, cd := range domainList.CustomDomains {
			port := 0
			if cd.TargetPort != nil {
				port = *cd.TargetPort
			}
			ls.CustomDomains = append(ls.CustomDomains, diff.LiveDomain{
				Domain: cd.Domain,
				Port:   port,
			})
		}

		// TCP Proxies.
		tcpProxies, err := client.ListTCPProxies(envID, svc.ID)
		if err != nil {
			return nil, fmt.Errorf("listing TCP proxies for service %q: %w", svc.Name, err)
		}
		for _, tp := range tcpProxies {
			ls.TCPProxies = append(ls.TCPProxies, diff.LiveTCPProxy{
				ApplicationPort: tp.ApplicationPort,
				ProxyPort:       tp.ProxyPort,
				Domain:          tp.Domain,
			})
		}

		liveServices = append(liveServices, ls)
	}

	return liveServices, nil
}
