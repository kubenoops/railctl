# Design: three-layer e2e suite keyed to token scope

**Date:** 2026-07-08
**Status:** Approved (design) — pending implementation
**Branch:** `feat/e2e-token-layers` (stacked on `feat/token-minting`)

## Problem

The e2e suite treats the API token as an opaque credential: every test assumes it can
create projects (account/workspace power), and nothing exercises the behaviours that
*differ* by token scope — disambiguation (`-w`), implicit scoping (project tokens ignore
`-p`/`-e`), and the fail-fast errors on out-of-scope operations. Live testing this cycle
produced a verified capability model (see `docs/token-capability-matrix.md`); the suite
should be restructured so that **the tests are the proof of that matrix**.

## Principles (set by the operator)

1. **One layer per token scope**, tested top-down: account → workspace → project.
2. **Each layer tests only what is exclusive to it.** No duplication across layers; the
   bulk of tests land at the project layer.
3. **Raw-API oddities railctl does not expose are out of scope** (e.g. a project token
   calling `projectCreate` directly). They are Railway's to block; railctl neither
   exposes nor tests them.
4. **Local-only.** e2e runs via make targets on a developer machine. No GitHub CI wiring.

## Layout

```
tests/e2e/
  harness/            # shared, non-test package (build tag e2e)
    env.go            # Env, Run/RunOK/RunFail, flag builders
    assert.go         # AssertContains / AssertValidJSON / …
    fixture.go        # uniqueName, waitForProject/Environment, Setup*/Teardown
    preflight.go      # token classification via internal/api + fail-fast
  account/            # L1 — runs with RAILWAY_ACCOUNT_TOKEN
  workspace/          # L2 — runs with RAILWAY_WORKSPACE_TOKEN
  project/            # L3 — runs with a project token MINTED in TestMain
```

Each group is its own Go package with its own `TestMain` that:
1. resolves the group's token from its env var,
2. **classifies it via `internal/api`** (`IsProjectToken` / `IsWorkspaceToken` /
   account probe — same detection railctl itself uses),
3. fails fast with an actionable message on a mismatch
   (e.g. `RAILWAY_WORKSPACE_TOKEN is an account token — mint a workspace token via
   apiTokenCreate or the dashboard`).

Wrong-token confusion becomes structurally impossible: the suite refuses to run rather
than producing misleading results.

## Layers

### L1 `account/` — exclusive: workspace enumeration & disambiguation

| Test | Proves (matrix row) |
|---|---|
| `get projects -w <ws>` for each workspace | account token spans workspaces |
| `create project` with **no** `-w` on a multi-workspace account → error `multiple workspaces found` | ambiguity fail-fast |
| `create project -w <ws>` + delete | explicit disambiguation works |

Small by design (~1 file). The future `api-token` command group (account-level minting,
verified possible via `apiTokenCreate`) lands here when built — **non-goal now**.

### L2 `workspace/` — exclusive: implicit workspace + lifecycle + arbitrary-target minting

| Test | Proves |
|---|---|
| `create project` (no `-w`) → `get/describe projects` → `delete project` | workspace inference fix; project lifecycle |
| `create environment` / `delete environment` | env lifecycle is workspace-level |
| `token create <n> -p <proj> -e <env>` / `token list` / `token delete` | workspace token mints for an arbitrary project |
| `-w <anything>` → ignored-with-warning | workspace token cannot switch workspaces |
| smoke (full lifecycle) | migrated `smoke_test.go` |

Migrates: `projects_test.go`, `environments_test.go`, `smoke_test.go`.

### L3 `project/` — the bulk: in-scope mechanics + boundary fail-fasts

**Fixture (`TestMain`):** the workspace token creates one project + one custom
environment, mints a project token scoped to them (**this bootstrap is itself the proof
that workspace→project minting works**), runs the whole package under that project
token, then tears down with the workspace token. The raw project token lives only in
process memory.

Because the token carries its scope, migrated tests **drop `-p`/`-e` flags** — which is
itself the assertion that implicit scoping works.

| Test | Proves |
|---|---|
| services / variables / volumes / backups / deployments / apply / diff mechanics | project token is fully capable in scope (migrated from existing files) |
| `get projects` → clean scoped error | cannot enumerate projects |
| `-p other -e other` → ignored-with-warning, operates on own scope | flags cannot re-aim a project token |
| `token create` (no flags) → mints within own scope; `token list`/`delete` | self-minting, in-scope token management |

Migrates: `services`, `update_service`, `variables`, `volumes`, `backups`,
`deployments`, `apply_diff`, `edge_cases`.

**Structural consequence of the no-duplication rule:** these tests currently call
`SetupEnvironment` (env creation — an L2 capability). Under L3 the fixture pre-creates
the environment; tests never perform lifecycle operations.

## Capability matrix doc

`docs/token-capability-matrix.md` — the verified allow/deny table (from live testing
2026-07-08: cross-project denied, cross-environment denied, project-token self-mint
allowed, workspace inference on create, account `-w` disambiguation). **Each row cites
the e2e test that proves it.** Rows railctl deliberately does not expose are marked
`not exposed — blocked upstream`.

## Make targets

```
test-e2e-account     # needs RAILWAY_ACCOUNT_TOKEN
test-e2e-workspace   # needs RAILWAY_WORKSPACE_TOKEN
test-e2e-project     # needs RAILWAY_WORKSPACE_TOKEN (fixture+mint) — bulk group
test-e2e             # all three, in top-down order
test-smoke           # unchanged entry point, now backed by the workspace group
```

`tests/e2e/.envrc.example` documents the two env vars. No CI workflow (Principle 4).

## Non-goals

- `api-token` command group (account/workspace token minting via CLI) — future work.
- `run.sh` (legacy 42 KB script) — untouched; candidate for deletion in a follow-up.
- Testing raw-API behaviours railctl does not expose (Principle 3).
- GitHub CI wiring for e2e.
