//go:build e2e

// Package project holds the L3 e2e test group: the bulk in-scope mechanics a
// project-scoped token can perform inside its own project/environment, plus
// the boundary fail-fasts (cannot enumerate projects, -p/-e/-w
// contradictions fail, self-minting). See
// docs/designs/2026-07-08-e2e-token-layers.md.
//
// TestMain builds the shared fixture with a workspace token (the bootstrap
// credential), mints a project token for it — that mint is itself the proof
// that workspace→project minting works — and runs every test in the package
// under the minted project token. The raw project token lives only in
// process memory. Teardown runs with the workspace token.
package project

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

var (
	// fixtureProject is the unique project every test in this group shares.
	fixtureProject string
	// projectToken is minted in TestMain and held only in process memory.
	projectToken string
	// bootstrapToken is the workspace token this group was bootstrapped with.
	// The bulk of tests run under the project token, but SSH-key registration
	// (for exec/port-forward) is an account/workspace-level operation a project
	// token cannot perform, so those tests register their ephemeral key with
	// this credential. Held only in process memory.
	bootstrapToken string
)

// fixtureEnvName is the environment the project token is scoped to.
//
// DESIGN ADJUSTMENT (2026-07-08, orchestrator decision, overrides the plan
// doc): the fixture keeps ONLY the default `production` environment — no
// custom environment is created. Railway creates new services in ALL
// environments of a project and railctl auto-cleans the non-target
// instances; a project token scoped to one environment cannot touch the
// other environments (cross-environment access is denied by Railway), so
// with a multi-env fixture service creation would fail for reasons unrelated
// to the behaviour under test. The project token is therefore minted for
// `production`.
//
// A var (not const) so bring-your-own-project-token mode can set it to the
// supplied token's actual environment; the workspace-bootstrap path leaves it
// at the `production` default.
var fixtureEnvName = "production"

func TestMain(m *testing.M) {
	// Bring-your-own-project-token mode: when RAILWAY_PROJECT_TOKEN is set, run
	// the group's fixture tests directly in a project the caller already owns,
	// using that supplied PROJECT token — no workspace power, and no fixture
	// project is created or deleted. Per-test cleanup still removes the throwaway
	// services/volumes each test creates. Tests that need account/workspace power
	// (exec/port-forward SSH-key registration) are unsupported here — select with
	// -run (e.g. -run 'TestRegions|TestApplyDiff_Region').
	if byo := os.Getenv("RAILWAY_PROJECT_TOKEN"); byo != "" && !compileCheckOnly() {
		os.Exit(runByoProject(m, byo))
	}

	wsToken := harness.RequireToken("RAILWAY_WORKSPACE_TOKEN", harness.TokenWorkspace)
	bootstrapToken = wsToken

	// Compile-check mode (-run '^$'): RequireToken skipped classification and
	// may have returned an empty token. No test will execute, so skip all
	// fixture setup — it needs live credentials and live API calls.
	if wsToken == "" || compileCheckOnly() {
		os.Exit(m.Run())
	}

	if harness.Railctl == "" {
		fmt.Fprintln(os.Stderr, "e2e project fixture: railctl binary not found — set RAILCTL or run: make build")
		os.Exit(1)
	}

	// 1. Create the fixture project and wait until it is queryable.
	fixtureProject = harness.UniqueName()
	if _, stderr, code := runCLI(wsToken, "create", "project", fixtureProject); code != 0 {
		fmt.Fprintf(os.Stderr, "e2e project fixture: create project %s failed (exit %d):\n%s\n",
			fixtureProject, code, stderr)
		os.Exit(1)
	}
	queryable := false
	for i := 0; i < 10; i++ {
		if _, _, code := runCLI(wsToken, "describe", "project", fixtureProject); code == 0 {
			queryable = true
			break
		}
		time.Sleep(2 * time.Second)
	}
	if !queryable {
		fmt.Fprintf(os.Stderr, "e2e project fixture: project %s not queryable after 10 attempts\n", fixtureProject)
		teardownFixture(wsToken)
		os.Exit(1)
	}

	// 2. Mint the group's project token with the workspace token — this
	// bootstrap IS the workspace→project mint proof.
	stdout, stderr, code := runCLI(wsToken,
		"token", "create", "e2e-fixture", "-p", fixtureProject, "-e", fixtureEnvName)
	if code != 0 {
		fmt.Fprintf(os.Stderr, "e2e project fixture: token create failed (exit %d):\n%s\n", code, stderr)
		teardownFixture(wsToken)
		os.Exit(1)
	}
	projectToken = strings.TrimSpace(stdout)

	// 3. Sanity: the minted credential must classify as a project token.
	got, err := harness.ClassifyToken(projectToken)
	if err != nil || got != harness.TokenProject {
		fmt.Fprintf(os.Stderr,
			"e2e project fixture: minted token did not classify as a project token (got %s, err %v)\n",
			got, err)
		teardownFixture(wsToken)
		os.Exit(1)
	}

	code = m.Run()

	// 4. Teardown with the workspace token (the project token cannot delete
	// its own project). E2E_KEEP=1 preserves a failed run's fixture.
	if os.Getenv("E2E_KEEP") == "1" && code != 0 {
		fmt.Fprintf(os.Stderr, "E2E_KEEP=1: leaving fixture project %s for debugging\n", fixtureProject)
	} else {
		teardownFixture(wsToken)
	}

	os.Exit(code)
}

// runByoProject runs the group under a caller-supplied project token against the
// project that token is already scoped to. It classifies the token (must be a
// project token), resolves its project/environment for the fixture Env, then runs
// the suite with NO fixture creation and NO project teardown — the caller owns the
// project. Each test still cleans up the throwaway services/volumes it creates.
// Returns the process exit code.
func runByoProject(m *testing.M, token string) int {
	got, err := harness.ClassifyToken(token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e project (BYO): RAILWAY_PROJECT_TOKEN failed type detection: %v\n", err)
		return 1
	}
	if got != harness.TokenProject {
		fmt.Fprintf(os.Stderr,
			"e2e project (BYO): RAILWAY_PROJECT_TOKEN is a %s token — this mode needs a project token.\n", got)
		return 1
	}
	if harness.Railctl == "" {
		fmt.Fprintln(os.Stderr, "e2e project (BYO): railctl binary not found — set RAILCTL or run: make build")
		return 1
	}

	projName, envName, err := harness.ProjectTokenScope(token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e project (BYO): could not resolve the token's project/environment: %v\n", err)
		return 1
	}

	projectToken = token
	fixtureProject = projName
	fixtureEnvName = envName
	// bootstrapToken stays empty: SSH-key registration (exec/port-forward) needs
	// workspace/account power a project token lacks. Those tests must be excluded
	// via -run in BYO mode.

	fmt.Fprintf(os.Stderr,
		"e2e project (BYO): running in existing project %q (env %q) under the supplied project token; "+
			"no fixture project is created or deleted.\n", projName, envName)

	// No fixture teardown — the caller owns the project. Per-test t.Cleanup still
	// deletes the services/volumes each test creates.
	code := m.Run()

	// Leak check: cleanup calls are best-effort (t.Cleanup ignores errors), so a
	// silently failing delete would leave debris invisible forever. Fail the run
	// if any e2e-prefixed resource survived — only e2e names, because in BYO
	// mode the project legitimately contains the caller's own resources.
	if code == 0 {
		if leaks := byoLeaks(token); len(leaks) > 0 {
			fmt.Fprintf(os.Stderr, "e2e project (BYO): LEAK CHECK FAILED — resources left behind:\n")
			for _, l := range leaks {
				fmt.Fprintf(os.Stderr, "  - %s\n", l)
			}
			return 1
		}
		fmt.Fprintln(os.Stderr, "e2e project (BYO): leak check clean — no e2e-prefixed resources remain.")
	}
	return code
}

// byoLeaks lists e2e-prefixed services and volumes still present in the
// token's environment.
func byoLeaks(token string) []string {
	var leaks []string
	for _, kind := range []string{"services", "volumes"} {
		stdout, _, code := runCLI(token, "get", kind, "-o", "json")
		if code != 0 {
			leaks = append(leaks, fmt.Sprintf("(could not list %s for the leak check)", kind))
			continue
		}
		var items []struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(stdout), &items); err != nil {
			continue
		}
		for _, it := range items {
			if strings.HasPrefix(it.Name, "e2e-") {
				leaks = append(leaks, strings.TrimSuffix(kind, "s")+" "+it.Name)
			}
		}
	}
	return leaks
}

// compileCheckOnly mirrors the harness's unexported check: `-run '^$'` is
// the conventional "compile everything, run nothing" invocation, in which
// the fixture must not be built.
func compileCheckOnly() bool {
	if !flag.Parsed() {
		flag.Parse()
	}
	f := flag.Lookup("test.run")
	return f != nil && f.Value.String() == "^$"
}

// runCLI executes railctl directly for TestMain-phase fixture work, where no
// *testing.T exists yet (harness.Env methods log through T). It mirrors
// harness.Env.Run: --token injection and a 3-minute per-command timeout.
func runCLI(token string, args ...string) (stdout, stderr string, code int) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	fullArgs := append([]string{"--token", token}, args...)
	cmd := exec.CommandContext(ctx, harness.Railctl, fullArgs...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()

	code = 0
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			code = -1
			errb.WriteString("\n[TIMEOUT] command exceeded 3-minute deadline")
		} else if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			code = -1
		}
	}
	return out.String(), errb.String(), code
}

// teardownFixture deletes the fixture's services and volumes, then the
// project itself, using the workspace token (mirrors harness.Teardown; the
// workspace token needs explicit -p/-e flags).
func teardownFixture(wsToken string) {
	if fixtureProject == "" {
		return
	}
	fmt.Fprintf(os.Stderr, "e2e project fixture: cleaning up project %s...\n", fixtureProject)

	stdout, _, code := runCLI(wsToken,
		"get", "services", "-p", fixtureProject, "-e", fixtureEnvName, "-o", "json")
	if code == 0 {
		var svcs []map[string]interface{}
		if json.Unmarshal([]byte(stdout), &svcs) == nil {
			for _, svc := range svcs {
				if name, ok := svc["name"].(string); ok {
					runCLI(wsToken, "delete", "service", name,
						"-p", fixtureProject, "-e", fixtureEnvName, "--yes")
				}
			}
		}
	}

	stdout, _, code = runCLI(wsToken,
		"get", "volumes", "-p", fixtureProject, "-e", fixtureEnvName, "-o", "json")
	if code == 0 {
		var vols []map[string]interface{}
		if json.Unmarshal([]byte(stdout), &vols) == nil {
			for _, vol := range vols {
				if name, ok := vol["name"].(string); ok {
					runCLI(wsToken, "delete", "volume", name,
						"-p", fixtureProject, "-e", fixtureEnvName, "--yes")
				}
			}
		}
	}

	time.Sleep(2 * time.Second)

	if _, stderr, code := runCLI(wsToken, "delete", "project", fixtureProject, "--yes"); code != 0 {
		fmt.Fprintf(os.Stderr, "e2e project fixture: could not delete project %s: %s\n",
			fixtureProject, stderr)
	}
}

// fixtureEnv returns an Env bound to the shared fixture project, running
// under the minted PROJECT token. Tests never call harness.SetupProject/
// SetupEnvironment/SetupService or Env.Teardown — TestMain owns the fixture
// lifecycle. Commands run WITHOUT -p/-e flags: the project token carries the
// scope, and running flag-free is itself the implicit-scoping assertion.
func fixtureEnv(t *testing.T) *harness.Env {
	t.Helper()
	harness.RequireBinary(t)
	return &harness.Env{
		T:           t,
		Token:       projectToken,
		ProjectName: fixtureProject,
		EnvName:     fixtureEnvName,
		ServiceImg:  "nginx:1.25-alpine",
	}
}

// createService creates a uniquely named service in the fixture project (no
// -p/-e: the project token carries the scope) and registers a best-effort
// cleanup so the shared fixture stays lean between tests. It waits briefly
// for the initial deployment to start, mirroring the old SetupService.
func createService(t *testing.T, env *harness.Env) string {
	t.Helper()
	name := harness.UniqueName()
	env.RunOK(t, "create", "service", name, "--image", env.ServiceImg)
	env.ServiceName = name
	t.Cleanup(func() {
		env.Run("delete", "service", name, "--yes")
	})
	t.Logf("Created service: %s (waiting for deployment...)", name)
	time.Sleep(3 * time.Second)
	return name
}
