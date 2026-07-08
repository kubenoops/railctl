# Plan: three-layer e2e suite keyed to token scope

**Design:** `docs/designs/2026-07-08-e2e-token-layers.md`
**Branch:** `feat/e2e-token-layers` (stacked on `feat/token-minting`)
**Execution:** subagent-driven; per task: implement → compile-verify (`go vet -tags e2e ./tests/e2e/...`) → review. Live verification with real tokens happens after each *group* task, run by the orchestrator (tokens never go to subagents).

## Verification commands

```bash
export PATH="/a0/usr/.asdf/installs/golang/1.22.0/go/bin:$HOME/go/bin:$PATH"
cd /a0/src/github.com/kubenoops/railctl
go build ./... && go vet -tags e2e ./tests/e2e/...   # every task
go test ./internal/...                                # unit suite stays green
```

## Task 1 — `tests/e2e/harness/` package

Extract `helpers_test.go` into a shared non-test package (build tag `e2e`):

- `env.go`: `Env` (exported fields as today), `Run/RunOK/RunFail`, `WithP/WithPE/WithPES`,
  binary resolution (`RAILCTL` env / walk-up), 3-min per-command timeout.
- `assert.go`: `AssertContains/AssertNotContains/AssertValidJSON/AssertValidYAML`, `truncate`.
- `fixture.go`: `UniqueName`, `WaitForProject/WaitForEnvironment`, `SetupProject/
  SetupEnvironment/SetupService` (now parameterised by explicit token, not `pickToken`),
  `Teardown`, `RequireBinary`.
- `preflight.go`: **new** —
  ```go
  type TokenType int // TokenAccount, TokenWorkspace, TokenProject
  func ClassifyToken(token string) (TokenType, error)   // uses internal/api Client probes
  func RequireToken(m *testing.M, envVar string, want TokenType) string
  // reads envVar, classifies, exits with actionable message on absence/mismatch
  ```
  Classification via `api.NewClient(token)`: `IsProjectToken()`, `IsWorkspaceToken()`,
  else account. Deleting `pickToken` / `RAILWAY_TOKEN_1..3` (superseded by per-group vars).
- Old flat `tests/e2e/*_test.go` keep compiling this task (temporary shim or leave them
  importing nothing — simplest: move them in their group task; this task only ADDS the
  harness package and deletes `helpers_test.go`, updating the flat tests' references via
  a dot-import shim is NOT allowed — instead port `helpers_test.go` wholesale and fix the
  flat tests' calls mechanically (`SetupProject(t)` → `harness.SetupProject(t, token)` is
  deferred to group tasks; to keep green, keep a thin `helpers_test.go` delegating to
  harness with the old signatures).

## Task 2 — L2 `tests/e2e/workspace/`

- `main_test.go`: `TestMain` → `harness.RequireToken(m, "RAILWAY_WORKSPACE_TOKEN", TokenWorkspace)`.
- Migrate `projects_test.go`, `environments_test.go`, `smoke_test.go` (package `workspace`,
  token from TestMain, drop the old shims).
- New `tokens_test.go`: `token create -p -e` → stdout is a 36-char value, stderr note;
  `token list` shows it (masked); `token delete --yes`; delete not-found → clean error.
- New `workspace_flags_test.go`: `-w anything` → warning on stderr, command still works.
- Delete migrated flat files.

## Task 3 — L3 `tests/e2e/project/`

- `main_test.go`: `TestMain` —
  1. `RequireToken("RAILWAY_WORKSPACE_TOKEN", TokenWorkspace)` (bootstrap credential),
  2. create fixture project + custom env (`harness.SetupEnvironment` semantics, but
     package-level, not per-test),
  3. mint project token via `railctl token create e2e-fixture -p <proj> -e <env>`
     (stdout capture — this IS the workspace→project mint proof),
  4. classify minted token == `TokenProject` (sanity),
  5. `m.Run()` with the project token as the group's `Env.Token`,
  6. teardown with the workspace token (delete project). `E2E_KEEP=1` honoured.
- Migrate `services`, `update_service`, `variables`, `volumes`, `backups`, `deployments`,
  `apply_diff`, `edge_cases` → package `project`, **drop `-p`/`-e` flags**, share the
  fixture project (tests create/delete their own services/volumes inside it; unique
  names to avoid collisions).
- New `boundaries_test.go`:
  - `get projects` → non-zero exit, error mentions scoped;
  - `-p other-proj get services` → warning `-p … ignored` on stderr, still lists own;
  - `token create self-mint` (no flags) → succeeds; `token list` contains it; `token
    delete` it.
- Delete migrated flat files (flat dir now holds nothing but `.envrc.example`, `README.md`, `run.sh`).

## Task 4 — L1 `tests/e2e/account/`

- `main_test.go`: `RequireToken("RAILWAY_ACCOUNT_TOKEN", TokenAccount)`.
- `workspaces_test.go`: `get projects -w <each>` (both workspaces listed in matrix doc);
  `create project` no `-w` → error contains `multiple workspaces found` (assumes the
  account sees ≥2 workspaces — preflight logs a skip if only 1); `create project -w` +
  `delete project -w` round-trip.

## Task 5 — `docs/token-capability-matrix.md`

Verified allow/deny table; columns account/workspace/project; every row cites its e2e
test (`workspace/TestProjects/create_no_w`, `project/TestBoundaries/get_projects_denied`, …).
Include the two boundary proofs (cross-project, cross-env) as *harness-level* notes —
they were verified by direct API probes 2026-07-08 and are enforced by Railway, marked
`not exposed — enforced upstream` where railctl has no command surface.

## Task 6 — wiring & docs

- Makefile: `test-e2e-account|workspace|project`, `test-e2e` (all, top-down),
  `test-smoke` → workspace group. Update help text.
- `tests/e2e/.envrc.example`: `RAILWAY_ACCOUNT_TOKEN`, `RAILWAY_WORKSPACE_TOKEN`.
- `tests/e2e/README.md`: rewrite for the three groups + preflight behaviour.
- Delete `run.sh`? **No** — out of scope (design non-goal).

## Live verification (orchestrator, after Tasks 2/3/4)

Run each group against real tokens; expect green; throwaway projects cleaned by the
suite itself. Also: delete the two manual test projects (`test-railctl`,
`test-railctl-2`) using the workspace token — doubles as a live `delete project` check.
