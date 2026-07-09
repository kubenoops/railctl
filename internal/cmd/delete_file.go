package cmd

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kubenoops/railctl/internal/cmdutil"
)

var (
	deleteFile string
	deleteYes  bool
)

func init() {
	deleteCmd.Flags().StringVarP(&deleteFile, "file", "f", "", "Path to YAML config file or directory (declarative delete)")
	deleteCmd.Flags().BoolVarP(&deleteYes, "yes", "y", false, "Skip confirmation prompt (declarative delete)")
}

// doomedService is a declared service resolved against live state.
type doomedService struct {
	name      string
	serviceID string
}

// doomedVolume is a manifest-declared volume attached to a doomed service.
// Deleting a service orphans its volume, so declared volumes are captured
// (with their instance→service link) before any service is deleted, and
// removed explicitly afterwards.
type doomedVolume struct {
	name       string // volume name, for messages
	volumeID   string // Volume.ID — what DeleteVolume expects
	mountPath  string
	declaredBy string // name of the service that declares it
}

// runDeleteFile implements `railctl delete -f <file-or-directory>`: delete
// exactly what the manifest declares — services in reverse manifest order,
// then their declared volumes. The environment and project are never deleted.
func runDeleteFile(cmd *cobra.Command, args []string) error {
	out := cmd.OutOrStdout()

	// 1. Load config from file or directory — same loader as apply/diff.
	// $env() refs are deliberately NOT expanded: deletion targets service
	// names and volume mount paths, neither of which supports expansion, so
	// teardown works without the apply-time secrets in the environment.
	cfg, err := loadConfig(deleteFile)
	if err != nil {
		return err
	}

	// 2. Resolve project/environment exactly like apply/diff: flags win,
	// then the manifest's project:/environment: fields; flag-free under a
	// project token (scope is baked into the token).
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

	// delete -f tears down services (structure) and their volumes (data), both
	// shielded by a delete-protected environment. Guard the whole operation up
	// front so a protected environment is never partially torn down.
	if err := cmdutil.RequireDeletable(client, projectID, ctx.Environment, "resources", "declared by this manifest"); err != nil {
		return err
	}

	// 3. Compute what exists: intersect declared services with live state.
	services, err := client.ListServices(projectID, envID)
	if err != nil {
		return fmt.Errorf("listing services: %w", err)
	}
	liveByName := make(map[string]string, len(services)) // name → ID
	for _, svc := range services {
		liveByName[svc.Name] = svc.ID
	}

	volumes, err := client.ListVolumes(projectID, envID)
	if err != nil {
		return fmt.Errorf("listing volumes: %w", err)
	}

	var doomed []doomedService    // manifest order
	var doomedVols []doomedVolume // manifest order
	skipped := 0
	for _, svc := range cfg.Services {
		id, ok := liveByName[svc.Name]
		if !ok {
			fmt.Fprintf(out, "Service '%s' not found — skipping.\n", svc.Name)
			skipped++
			continue
		}
		doomed = append(doomed, doomedService{name: svc.Name, serviceID: id})

		if svc.Volume.MountPath == "" {
			continue
		}
		// The declared mountPath identifies the volume among the service's
		// attached instances.
		found := false
		for _, vi := range volumes {
			if vi.ServiceID != nil && *vi.ServiceID == id && vi.MountPath == svc.Volume.MountPath {
				doomedVols = append(doomedVols, doomedVolume{
					name:       vi.Volume.Name,
					volumeID:   vi.Volume.ID,
					mountPath:  vi.MountPath,
					declaredBy: svc.Name,
				})
				found = true
				break
			}
		}
		if !found {
			fmt.Fprintf(out, "Volume at '%s' (declared by '%s') not found — skipping.\n", svc.Volume.MountPath, svc.Name)
			skipped++
		}
	}

	if len(doomed) == 0 && len(doomedVols) == 0 {
		fmt.Fprintf(out, "Nothing to delete.\n0 services deleted, 0 volumes deleted, %d skipped (not found)\n", skipped)
		return nil
	}

	// 4. Report what will be deleted, in deletion (reverse-manifest) order.
	fmt.Fprintf(out, "The following will be deleted from project '%s' environment '%s':\n", ctx.Project.Name, ctx.Environment.Name)
	for i := len(doomed) - 1; i >= 0; i-- {
		fmt.Fprintf(out, "  - service '%s'\n", doomed[i].name)
	}
	for i := len(doomedVols) - 1; i >= 0; i-- {
		v := doomedVols[i]
		fmt.Fprintf(out, "  - volume '%s' (mounted at %s, declared by '%s')\n", v.name, v.mountPath, v.declaredBy)
	}

	// 5. Confirm unless --yes.
	if !deleteYes {
		fmt.Fprintf(out, "Delete %d service(s) and %d volume(s)? This cannot be undone. [y/N]: ", len(doomed), len(doomedVols))
		reader := bufio.NewReader(cmd.InOrStdin())
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}
		response = strings.ToLower(strings.TrimSpace(response))
		if response != "y" && response != "yes" {
			fmt.Fprintln(out, "Deletion cancelled.")
			return nil
		}
	}

	// 6. Delete services in reverse manifest order (dependents declared
	// later go first), then the declared volumes they leave orphaned.
	var errs []error
	failedService := make(map[string]bool)
	servicesDeleted := 0
	for i := len(doomed) - 1; i >= 0; i-- {
		d := doomed[i]
		fmt.Fprintf(out, "Deleting service '%s'...\n", d.name)
		if err := client.DeleteService(d.serviceID); err != nil {
			errs = append(errs, fmt.Errorf("delete service %s: %w", d.name, err))
			failedService[d.name] = true
			continue
		}
		servicesDeleted++
		fmt.Fprintf(out, "✓ Service '%s' deleted\n", d.name)
	}

	volumesDeleted := 0
	for i := len(doomedVols) - 1; i >= 0; i-- {
		v := doomedVols[i]
		if failedService[v.declaredBy] {
			// Still attached — deleting it would fail or strand state.
			fmt.Fprintf(out, "Skipping volume '%s' — service '%s' was not deleted\n", v.name, v.declaredBy)
			continue
		}
		fmt.Fprintf(out, "Deleting volume '%s'...\n", v.name)
		if err := client.DeleteVolume(v.volumeID); err != nil {
			errs = append(errs, fmt.Errorf("delete volume %s: %w", v.name, err))
			continue
		}
		volumesDeleted++
		fmt.Fprintf(out, "✓ Volume '%s' deleted\n", v.name)
	}

	// 7. Summary. Non-zero exit only on actual failures.
	fmt.Fprintf(out, "\n%d services deleted, %d volumes deleted, %d skipped (not found)\n", servicesDeleted, volumesDeleted, skipped)
	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", e)
		}
		return fmt.Errorf("%d error(s) during delete", len(errs))
	}
	return nil
}
