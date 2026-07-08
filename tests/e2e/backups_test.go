//go:build e2e

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestBackupSchedules exercises the volume backup-schedule lifecycle end to end.
// It drives the reconciliation that is synchronous (declarative schedule set →
// present → cleared) and checks that the manual backup commands are wired. It
// deliberately does NOT wait for an actual backup workflow to finish — only
// schedule reconciliation and command plumbing are asserted.
//
// Setup: creates project + environment
//
//	go test -tags e2e -v -run TestBackupSchedules ./tests/e2e/...
func TestBackupSchedules(t *testing.T) {
	e := SetupEnvironment(t)

	cfgDir := t.TempDir()
	cfgFile := filepath.Join(cfgDir, "svc.yaml")

	withSchedules := `services:
  - name: backup-svc
    image: postgres:16-alpine
    volume:
      mountPath: /var/lib/postgresql/data
      backupSchedules: [daily, weekly]
`
	if err := os.WriteFile(cfgFile, []byte(withSchedules), 0644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	// 1. Apply: creates the service, its volume, and the backup schedules.
	r := e.RunOK(t, e.WithPE("apply", "-f", cfgFile)...)
	AssertContains(t, r.Stdout, "Created")
	time.Sleep(3 * time.Second)

	// 2. Capture the auto-assigned volume name for the backup commands.
	r = e.RunOK(t, e.WithPE("get", "volumes", "-o", "json")...)
	AssertValidJSON(t, r.Stdout)
	var vols []map[string]interface{}
	if err := json.Unmarshal([]byte(r.Stdout), &vols); err != nil || len(vols) == 0 {
		t.Fatalf("expected at least one volume, got %q (err=%v)", r.Stdout, err)
	}
	volName, _ := vols[0]["name"].(string)
	if volName == "" {
		t.Fatal("could not capture volume name from get volumes output")
	}

	// 3. Schedules should be present (declarative reconcile is synchronous).
	r = e.RunOK(t, e.WithPE("get", "backups", volName, "--schedules")...)
	AssertContains(t, r.Stdout, "DAILY")
	AssertContains(t, r.Stdout, "WEEKLY")

	// 4. describe volume surfaces the schedules too.
	r = e.RunOK(t, e.WithPE("describe", "volume", volName)...)
	AssertContains(t, r.Stdout, "Backup Schedules")

	// 5. Manual backup command is wired. Railway runs it asynchronously and may
	// refuse a backup on a volume that has never been deployed, so we exercise
	// the command path without asserting success.
	_ = e.Run(e.WithPE("create", "backup", volName)...)

	// Listing backups is a read and must always succeed (possibly empty).
	e.RunOK(t, e.WithPE("get", "backups", volName)...)

	// 6. Clear the schedules by omitting backupSchedules on the managed volume.
	noSchedules := `services:
  - name: backup-svc
    image: postgres:16-alpine
    volume:
      mountPath: /var/lib/postgresql/data
`
	if err := os.WriteFile(cfgFile, []byte(noSchedules), 0644); err != nil {
		t.Fatalf("writing cleared config file: %v", err)
	}

	// diff should report the schedules being removed (exit non-zero).
	r = e.Run(e.WithPE("diff", "-f", cfgFile)...)
	if r.ExitCode == 0 {
		t.Fatalf("expected diff to exit non-zero when clearing schedules, got 0\nstdout: %s", r.Stdout)
	}
	AssertContains(t, r.Stdout, "backupSchedules")

	// apply should clear them and warn about it.
	r = e.RunOK(t, e.WithPE("apply", "-f", cfgFile)...)
	AssertContains(t, r.Stdout, "cleared")

	// 7. Schedules are gone.
	r = e.RunOK(t, e.WithPE("get", "backups", volName, "--schedules")...)
	AssertContains(t, r.Stdout, "No backup schedules")
}
