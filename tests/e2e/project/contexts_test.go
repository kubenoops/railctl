//go:build e2e

// TestConfigContexts walks the full local-contexts journey against live
// Railway: store the minted project token as a credential, build and
// activate a context referencing it, then run a command with every token
// and targeting input supplied ONLY by the config file — the e2e proof of
// the feature's core promise (specs/001-config-contexts, US1+US2).
//
// Token values must never surface in any output (FR-020): every result is
// asserted NotContains against the raw token.
package project

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// configTestEnv returns the per-test env entries pinning the scratch config
// file, plus a shorthand for "the whole config layer is the only input".
func configTestEnv(t *testing.T) []string {
	t.Helper()
	return []string{"RAILCTL_CONFIG=" + filepath.Join(t.TempDir(), "config.yaml")}
}

// assertNoTokenLeak fails the test if the raw project token value appears in
// any stream of the result (FR-020 — the e2e version of the unit-test rule).
func assertNoTokenLeak(t *testing.T, r harness.Result) {
	t.Helper()
	harness.AssertNotContains(t, r.Stdout, projectToken)
	harness.AssertNotContains(t, r.Stderr, projectToken)
}

func TestConfigContexts(t *testing.T) {
	env := fixtureEnv(t)
	e := configTestEnv(t)

	// 1. Store the minted project token as a credential via stdin — the
	// preferred entry path. The live type-detection probe runs here and the
	// detected type must come back as "project".
	r := env.RunWithEnv(strings.NewReader(projectToken+"\n"), e,
		"config", "set-credentials", "e2e-proj", "--token-stdin")
	if r.ExitCode != 0 {
		t.Fatalf("set-credentials failed (exit %d):\nstdout: %s\nstderr: %s",
			r.ExitCode, r.Stdout, r.Stderr)
	}
	harness.AssertContains(t, r.Stdout, "e2e-proj")
	harness.AssertContains(t, r.Stdout, "project")
	assertNoTokenLeak(t, r)

	// 2. A context referencing the credential by name; the context's
	// project/environment match the token's baked scope, so resolution is
	// consistent (no contradiction).
	r = env.RunWithEnv(nil, e, "config", "set-context", "e2e-ctx",
		"-p", fixtureProject, "-e", fixtureEnvName, "--credentials", "e2e-proj")
	if r.ExitCode != 0 {
		t.Fatalf("set-context failed (exit %d):\nstdout: %s\nstderr: %s",
			r.ExitCode, r.Stdout, r.Stderr)
	}
	assertNoTokenLeak(t, r)

	// 3. Activate and verify the active marker round-trips.
	r = env.RunWithEnv(nil, e, "config", "use-context", "e2e-ctx")
	if r.ExitCode != 0 {
		t.Fatalf("use-context failed (exit %d):\nstderr: %s", r.ExitCode, r.Stderr)
	}
	r = env.RunWithEnv(nil, e, "config", "current-context")
	if r.ExitCode != 0 || strings.TrimSpace(r.Stdout) != "e2e-ctx" {
		t.Fatalf("current-context = %q (exit %d), want %q", r.Stdout, r.ExitCode, "e2e-ctx")
	}

	// 4. get-contexts: active marker, token TYPE shown, token VALUE never.
	r = env.RunWithEnv(nil, e, "config", "get-contexts")
	if r.ExitCode != 0 {
		t.Fatalf("get-contexts failed (exit %d):\nstdout: %s", r.ExitCode, r.Stdout)
	}
	for _, want := range []string{"e2e-ctx", fixtureProject, fixtureEnvName, "e2e-proj", "project", "*"} {
		harness.AssertContains(t, r.Stdout, want)
	}
	assertNoTokenLeak(t, r)

	// 5. THE core assertion: a command with NO --token, NO RAILWAY_TOKEN,
	// NO -p/-e flags — every input comes from the config layer (the env is
	// scrubbed by RunWithEnv). The project token's own scope resolves the
	// command; the empty listing is a valid success.
	r = env.RunWithEnv(nil, e, "get", "services")
	if r.ExitCode != 0 {
		t.Fatalf("flag-free get services via context failed (exit %d):\nstdout: %s\nstderr: %s",
			r.ExitCode, r.Stdout, r.Stderr)
	}
	assertNoTokenLeak(t, r)

	// 6. get-credentials lists names and types only.
	r = env.RunWithEnv(nil, e, "config", "get-credentials")
	if r.ExitCode != 0 {
		t.Fatalf("get-credentials failed (exit %d):\nstdout: %s", r.ExitCode, r.Stdout)
	}
	harness.AssertContains(t, r.Stdout, "e2e-proj")
	harness.AssertContains(t, r.Stdout, "project")
	assertNoTokenLeak(t, r)

	// 7. Guards: the credential cannot be deleted while referenced; the
	// context deletion clears the active slot; current-context then fails.
	r = env.RunWithEnv(nil, e, "config", "delete-credentials", "e2e-proj")
	if r.ExitCode == 0 || !strings.Contains(r.Stderr, "e2e-ctx") {
		t.Fatalf("delete-credentials must be refused naming the referencing context; exit %d, stderr: %s",
			r.ExitCode, r.Stderr)
	}

	r = env.RunWithEnv(nil, e, "config", "delete-context", "e2e-ctx")
	if r.ExitCode != 0 {
		t.Fatalf("delete-context failed (exit %d):\nstderr: %s", r.ExitCode, r.Stderr)
	}
	harness.AssertContains(t, r.Stderr, "e2e-ctx") // the clearing notice

	r = env.RunWithEnv(nil, e, "config", "current-context")
	if r.ExitCode == 0 {
		t.Fatalf("current-context must fail after the active context was deleted, got exit 0, stdout: %s", r.Stdout)
	}
}

// TestConfigContexts_TokenConflictWarning proves FR-018 live: with both an
// ambient RAILWAY_TOKEN and the selected context's credentials present, the
// environment token wins and a stderr warning names both sources.
func TestConfigContexts_TokenConflictWarning(t *testing.T) {
	env := fixtureEnv(t)
	e := configTestEnv(t)

	r := env.RunWithEnv(strings.NewReader(projectToken+"\n"), e,
		"config", "set-credentials", "e2e-proj", "--token-stdin")
	if r.ExitCode != 0 {
		t.Fatalf("set-credentials failed (exit %d):\nstderr: %s", r.ExitCode, r.Stderr)
	}
	r = env.RunWithEnv(nil, e, "config", "set-context", "e2e-ctx",
		"-p", fixtureProject, "-e", fixtureEnvName, "--credentials", "e2e-proj")
	if r.ExitCode != 0 {
		t.Fatalf("set-context failed (exit %d):\nstderr: %s", r.ExitCode, r.Stderr)
	}
	r = env.RunWithEnv(nil, e, "config", "use-context", "e2e-ctx")
	if r.ExitCode != 0 {
		t.Fatalf("use-context failed (exit %d):\nstderr: %s", r.ExitCode, r.Stderr)
	}

	// Ambient env token (here: the same project token — validity is what
	// matters, not the identity) plus context credentials: env wins, the
	// warning fires on stderr naming RAILWAY_TOKEN and the context.
	r = env.RunWithEnv(nil, append(e, "RAILWAY_TOKEN="+projectToken), "get", "services")
	if r.ExitCode != 0 {
		t.Fatalf("env-token command must succeed (exit %d):\nstdout: %s\nstderr: %s",
			r.ExitCode, r.Stdout, r.Stderr)
	}
	harness.AssertContains(t, r.Stderr, "RAILWAY_TOKEN")
	harness.AssertContains(t, r.Stderr, "e2e-ctx")
}
