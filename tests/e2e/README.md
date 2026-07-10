# railctl E2E Test Suite

End-to-end tests that exercise `railctl` against the live Railway API, structured as
**three groups keyed to Railway token scope**. The suite is the executable form of
[`docs/token-capability-matrix.md`](../../docs/token-capability-matrix.md) — the
authoritative allow/deny matrix, verified live. Each test proves one or more matrix
rows; when Railway's behaviour drifts, these tests fail. Design rationale:
[`docs/designs/2026-07-08-e2e-token-layers.md`](../../docs/designs/2026-07-08-e2e-token-layers.md).

## The three-layer model

A Railway token is a pointer to exactly one node of the containment tree
(account → workspace → project + environment): full access at and below its node,
name-visibility of its own containment chain, hard denial everywhere else. Each e2e
group runs under exactly one token scope and tests only what is **exclusive** to that
scope — no duplication across layers.

| Token | Can list (one level down) | Bound? | What "list" grants |
|---|---|---|---|
| Account | workspaces | not bound — full access to all of them | full access |
| Workspace | projects | not bound — full access to all of them | full access |
| Project | environments | **bound to one** | **names only** for siblings — content access solely in its bound environment |

## Layout

```
tests/e2e/
  harness/     # shared non-test package (build tag e2e): Env runner, assertions,
               # fixtures, and the token-type preflight (ClassifyToken/RequireToken)
  account/     # L1 — runs with RAILWAY_ACCOUNT_TOKEN
  workspace/   # L2 — runs with RAILWAY_WORKSPACE_TOKEN
  project/     # L3 — bootstrapped with RAILWAY_WORKSPACE_TOKEN, runs under a
               # project token minted in TestMain
```

**`harness/`** — the shared package every group imports: `Env` (CLI runner with
`--token` injection and per-command timeouts), assertions, project/environment
fixtures, and the preflight that classifies tokens with the same detection railctl
itself uses (`internal/api`).

**`account/`** — small by design. Exclusive to an account token: workspace
enumeration and `-w` disambiguation (create with no `-w` on a multi-workspace
account fails; explicit `-w` round-trips).

**`workspace/`** — exclusive to a workspace token: project and environment
lifecycle (with implicit workspace inference — no `-w` needed), minting project
tokens for arbitrary projects, `-w` rejection on a bound token, the `TestSmoke`
full-lifecycle walk, the **`DELETE_PROTECTION` resource matrix**
(`TestDeleteProtectionResources`: a protected environment blocks service/volume
deletes but allows image updates + variable set/delete), and the
**least-privilege hint** (`TestLeastPrivilegeHint`: the stderr nudge fires in
text mode, is silent under `-o json` and `RAILCTL_NO_HINTS=1`).

**`project/`** — the bulk. In-scope mechanics (services, variables, volumes,
backups, deployments, apply/diff) run **flag-free**: the token carries its
(project, environment) scope, so dropping `-p`/`-e` is itself the implicit-scoping
assertion. Plus the boundary fail-fasts: project enumeration denied, contradicting
`-p`/`-e` flags refused, self-minting within scope. Also **`exec` and
`port-forward` over Railway's SSH relay** (`TestExec`, `TestPortForward`): both
run under the minted project token — proving the v1.1.0 removal of the SSH
project-token gate — after registering a throwaway SSH key (an account/workspace
operation, so it uses the bootstrap workspace token and revokes it at the end;
these two tests `t.Skip` if `ssh`/`ssh-keygen` are not on `PATH`). Its `TestMain`
fixture: the workspace token creates a fixture project, mints a project token
scoped to it (that bootstrap is itself the proof that workspace→project minting
works), every test runs under the minted token — which lives only in process
memory — and teardown runs with the workspace token.

## How to run

Tokens (see `.envrc.example`; with [direnv](https://direnv.net/), copy it to
`tests/e2e/.envrc` and `direnv allow` — the make targets pick the vars up from the
environment):

| Variable | Needed by |
|---|---|
| `RAILWAY_ACCOUNT_TOKEN` | `account/` — account token (railway.app/account/tokens, **no** workspace selected) |
| `RAILWAY_WORKSPACE_TOKEN` | `workspace/` and `project/` — workspace-scoped token |

Make targets (each builds the binary first):

```bash
make test-e2e-account     # L1, timeout 10m
make test-e2e-workspace   # L2, timeout 20m
make test-e2e-project     # L3 (bulk), timeout 25m
make test-e2e             # all three, top-down: account → workspace → project
make test-smoke           # TestSmoke only (~1min, lives in the workspace group)
```

Run a single test directly:

```bash
RAILCTL=$PWD/railctl RAILWAY_WORKSPACE_TOKEN=... \
  go test -tags e2e -v -run TestBoundaries ./tests/e2e/project/...
```

## Preflight: wrong tokens refuse to run

Each group's `TestMain` calls `harness.RequireToken`, which classifies the token
against the live API using the same detection railctl itself uses
(`IsProjectToken` / `IsWorkspaceToken` / account fallback) and **exits with an
actionable message** if the env var is missing, the token fails detection, or its
scope mismatches the group (e.g. an account token in `RAILWAY_WORKSPACE_TOKEN`).
Wrong-token confusion is structurally impossible: the suite refuses to run rather
than producing misleading results.

Compile-check mode skips the preflight (and the project group's fixture), since no
test executes:

```bash
go test -tags e2e -run '^$' ./tests/e2e/...
```

## Debugging

- **`E2E_KEEP=1`** — on a **failed** run, skips cleanup so you can inspect
  resources in the Railway dashboard (harness fixtures keep their project only if
  the test failed; the project group keeps its fixture only on a non-zero exit).
  Remember to delete the leftovers manually.
- **`E2E_PROJECT=<name>`** — overrides the generated project name in
  `harness.SetupProject` (used by the account and workspace groups). The project
  is still created and torn down — this pins the name, it does not attach to an
  existing project. The project group ignores it (its fixture always generates a
  unique name).
- **`RAILCTL=<path>`** — path to the binary under test; without it the harness
  walks upward from the working directory looking for `railctl`.

## Legacy

`run.sh` is the legacy flat bash suite — untouched, superseded by the Go groups,
candidate for deletion in a follow-up.
