//go:build e2e

package account

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// ambiguityRe extracts the comma-separated workspace names from the
// GetWorkspaceID ambiguity error:
//
//	multiple workspaces found (A, B): use -w <name> or set RAILCTL_WORKSPACE=<name>
var ambiguityRe = regexp.MustCompile(`multiple workspaces found \(([^)]+)\)`)

// combined returns stdout+stderr so assertions don't depend on which stream
// the CLI printed the error to.
func combined(r harness.Result) string {
	return r.Stdout + r.Stderr
}

// discoverWorkspaces probes the account's workspace visibility by running
// `get projects` with NO -w flag. Two outcomes:
//
//	(a) exit 0  → single workspace (or auto-selected): multi=false, no names.
//	(b) exit !=0 with the "multiple workspaces found (A, B): use -w ..."
//	    ambiguity error → multi=true, names parsed from the error text.
//
// Any other failure is unexpected and fatals the test.
func discoverWorkspaces(t *testing.T, env *harness.Env) (multi bool, names []string, ambiguityOut string) {
	t.Helper()
	r := env.Run("get", "projects")
	out := combined(r)
	if r.ExitCode == 0 {
		t.Log("`get projects` without -w succeeded: account sees a single workspace (or auto-selected)")
		return false, nil, ""
	}
	m := ambiguityRe.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("`get projects` without -w failed, but not with the workspace-ambiguity error:\n%s", out)
	}
	for _, n := range strings.Split(m[1], ",") {
		if n = strings.TrimSpace(n); n != "" {
			names = append(names, n)
		}
	}
	t.Logf("account sees %d workspaces: %v", len(names), names)
	return true, names, out
}

// TestWorkspaceDisambiguation proves the L1 (account-token) exclusive
// behaviours: workspace enumeration and disambiguation via -w.
func TestWorkspaceDisambiguation(t *testing.T) {
	harness.RequireBinary(t)
	env := &harness.Env{T: t, Token: token}

	multi, names, ambiguityOut := discoverWorkspaces(t, env)

	t.Run("ambiguity_fail_fast", func(t *testing.T) {
		if !multi {
			t.Skip("account sees a single workspace")
		}
		harness.AssertContains(t, ambiguityOut, "multiple workspaces found")
		harness.AssertContains(t, ambiguityOut, "-w")
		if len(names) < 2 {
			t.Errorf("expected the ambiguity error to name at least 2 workspaces, parsed %d: %v", len(names), names)
		}
	})

	t.Run("w_resolves_each", func(t *testing.T) {
		if len(names) == 0 {
			env.RunOK(t, "get", "projects")
			t.Skip("no workspace names discovered (single-workspace account); plain `get projects` verified instead")
		}
		for _, ws := range names {
			r := env.RunOK(t, "get", "projects", "-w", ws)
			t.Logf("get projects -w %s OK (%d bytes of output)", ws, len(r.Stdout))
		}
	})

	t.Run("w_nonexistent_lists_available", func(t *testing.T) {
		r := env.RunFail(t, "get", "projects", "-w", "definitely-not-a-workspace-xyz")
		out := combined(r)
		harness.AssertContains(t, out, "not found")
		harness.AssertContains(t, out, "available:")
	})

	t.Run("create_project_no_w_ambiguity", func(t *testing.T) {
		if !multi {
			t.Skip("account sees a single workspace")
		}
		name := harness.UniqueName()
		r := env.Run("create", "project", name)
		if r.ExitCode == 0 {
			// Defensive: should never happen on a multi-workspace account —
			// clean up the accidentally-created project before failing.
			t.Cleanup(func() {
				env.Run("delete", "project", name, "--yes", "-w", names[0])
			})
			t.Fatalf("expected `create project` without -w to fail on a multi-workspace account, but it succeeded:\n%s", r.Stdout)
		}
		harness.AssertContains(t, combined(r), "multiple workspaces found")
	})

	t.Run("create_project_with_w_roundtrip", func(t *testing.T) {
		if len(names) == 0 {
			t.Skip("no workspace names discovered (single-workspace account)")
		}
		ws := names[0]
		name := harness.UniqueName()

		env.RunOK(t, "create", "project", name, "-w", ws)

		deleted := false
		t.Cleanup(func() {
			if deleted {
				return
			}
			env.Run("delete", "project", name, "--yes", "-w", ws)
		})

		// Poll until the project is queryable (mirrors harness.WaitForProject,
		// hand-rolled because the helper does not pass -w).
		queryable := false
		for i := 0; i < 10; i++ {
			r := env.Run("describe", "project", name, "-w", ws)
			if r.ExitCode == 0 {
				t.Logf("project %s confirmed queryable in workspace %s after %d poll(s)", name, ws, i+1)
				queryable = true
				break
			}
			t.Logf("waiting for project %s to propagate (attempt %d/10)...", name, i+1)
			time.Sleep(2 * time.Second)
		}
		if !queryable {
			t.Fatalf("project %s not queryable in workspace %s after 10 attempts", name, ws)
		}

		env.RunOK(t, "delete", "project", name, "--yes", "-w", ws)
		deleted = true
	})
}
