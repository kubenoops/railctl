//go:build e2e

package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// TestBackupSchedules exercises the volume backup-schedule lifecycle end to
// end inside the shared fixture project under the minted project token (no
// -p/-e flags). It drives the reconciliation that is synchronous
// (declarative schedule set → present → cleared) and checks that the manual
// backup commands are wired. It deliberately does NOT wait for an actual
// backup workflow to finish — only schedule reconciliation and command
// plumbing are asserted.
//
//	go test -tags e2e -v -run TestBackupSchedules ./tests/e2e/project/...
func TestBackupSchedules(t *testing.T) {
	e := fixtureEnv(t)
	svcName := harness.UniqueName()

	volName := ""
	t.Cleanup(func() {
		// The apply-managed service + volume are this test's to clean up —
		// the fixture project is shared across the group.
		if volName != "" {
			e.Run("delete", "volume", volName, "--yes")
		}
		e.Run("delete", "service", svcName, "--yes")
	})

	cfgDir := t.TempDir()
	cfgFile := filepath.Join(cfgDir, "svc.yaml")

	withSchedules := fmt.Sprintf(`services:
  - name: %s
    image: postgres:16-alpine
    volume:
      mountPath: /var/lib/postgresql/data
      backupSchedules: [daily, weekly]
`, svcName)
	if err := os.WriteFile(cfgFile, []byte(withSchedules), 0644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	// 1. Apply: creates the service, its volume, and the backup schedules.
	r := e.RunOK(t, "apply", "-f", cfgFile)
	harness.AssertContains(t, r.Stdout, "Created")
	time.Sleep(3 * time.Second)

	// 2. Capture the auto-assigned volume name for the backup commands.
	// Per-test cleanup keeps the shared fixture empty between tests, so the
	// first listed volume is this test's.
	r = e.RunOK(t, "get", "volumes", "-o", "json")
	harness.AssertValidJSON(t, r.Stdout)
	var vols []map[string]interface{}
	if err := json.Unmarshal([]byte(r.Stdout), &vols); err != nil || len(vols) == 0 {
		t.Fatalf("expected at least one volume, got %q (err=%v)", r.Stdout, err)
	}
	volName, _ = vols[0]["name"].(string)
	if volName == "" {
		t.Fatal("could not capture volume name from get volumes output")
	}

	// 3. Schedules should be present (declarative reconcile is synchronous).
	r = e.RunOK(t, "get", "backups", volName, "--schedules")
	harness.AssertContains(t, r.Stdout, "DAILY")
	harness.AssertContains(t, r.Stdout, "WEEKLY")

	// 4. describe volume surfaces the schedules too.
	r = e.RunOK(t, "describe", "volume", volName)
	harness.AssertContains(t, r.Stdout, "Backup Schedules")

	// 5. Manual backup command is wired. Railway runs it asynchronously and
	// may refuse a backup on a volume that has never been deployed, so we
	// exercise the command path without asserting success.
	_ = e.Run("create", "backup", volName)

	// Listing backups is a read and must always succeed (possibly empty).
	e.RunOK(t, "get", "backups", volName)

	// 6. Clear the schedules by omitting backupSchedules on the managed volume.
	noSchedules := fmt.Sprintf(`services:
  - name: %s
    image: postgres:16-alpine
    volume:
      mountPath: /var/lib/postgresql/data
`, svcName)
	if err := os.WriteFile(cfgFile, []byte(noSchedules), 0644); err != nil {
		t.Fatalf("writing cleared config file: %v", err)
	}

	// diff should report the schedules being removed (exit non-zero).
	r = e.Run("diff", "-f", cfgFile)
	if r.ExitCode == 0 {
		t.Fatalf("expected diff to exit non-zero when clearing schedules, got 0\nstdout: %s", r.Stdout)
	}
	harness.AssertContains(t, r.Stdout, "backupSchedules")

	// apply should clear them and warn about it.
	r = e.RunOK(t, "apply", "-f", cfgFile)
	harness.AssertContains(t, r.Stdout, "cleared")

	// 7. Schedules are gone.
	r = e.RunOK(t, "get", "backups", volName, "--schedules")
	harness.AssertContains(t, r.Stdout, "No backup schedules")
}
