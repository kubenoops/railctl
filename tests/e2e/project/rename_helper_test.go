//go:build e2e

package project

import (
	"strings"
	"testing"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// tryRenameVolume attempts `update volume <vol> --name <unique>` and returns
// the volume's name afterwards. The expected outcome is STRICT per credential
// mode — a behavior change on Railway's side fails the suite loudly in either
// direction (no tolerated in-between):
//
//   - BYO mode (bare project token, bootstrapToken == ""): Railway DENIES the
//     volumeUpdate mutation to raw project tokens (verified live 2026-08-30;
//     the official Railway CLI fails identically), so the rename MUST fail
//     with Not Authorized. If it ever succeeds, Railway changed the token
//     scope — the test fails so it gets noticed and re-specced.
//   - Fixture mode (project token minted from a workspace token): the rename
//     MUST succeed and is verified against `get volumes`.
func tryRenameVolume(t *testing.T, env *harness.Env, volName string) string {
	t.Helper()
	renamed := harness.UniqueName()
	r := env.Run("update", "volume", volName, "--name", renamed)

	if bootstrapToken == "" {
		// BYO: the denial is the specified behavior.
		if r.ExitCode == 0 {
			t.Fatalf("volume rename SUCCEEDED under a bare project token — Railway's volumeUpdate scope changed; update the spec and this expectation (renamed %s → %s)", volName, renamed)
		}
		if !strings.Contains(r.Stderr, "Not Authorized") {
			t.Fatalf("volume rename failed but not with the expected Not Authorized denial (exit %d):\nstdout: %s\nstderr: %s", r.ExitCode, r.Stdout, r.Stderr)
		}
		t.Logf("volume rename denied as specified for a bare project token (keeping %q)", volName)
		return volName
	}

	// Fixture mode: rename must work.
	if r.ExitCode != 0 {
		t.Fatalf("volume rename failed under the minted project token (exit %d):\nstdout: %s\nstderr: %s", r.ExitCode, r.Stdout, r.Stderr)
	}
	list := env.RunOK(t, "get", "volumes")
	harness.AssertContains(t, list.Stdout, renamed)
	t.Logf("volume renamed %s → %s", volName, renamed)
	return renamed
}
