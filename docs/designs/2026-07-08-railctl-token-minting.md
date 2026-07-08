# Design: `railctl token` — project/environment token minting

**Date:** 2026-07-08
**Status:** Approved (design) — pending implementation
**Scope:** One feature, one PR (`feat/token-minting`)

## Problem

railctl can already *consume* a project-scoped token (via the `Project-Access-Token`
header, auto-detected in `internal/api/client.go`), but it cannot *mint* one. The
Railway GraphQL API exposes `projectTokenCreate`, which produces a token scoped to a
single project + environment — a much smaller blast radius than a workspace or account
token. Users (and CI harnesses) want to create, list, and revoke these tokens from the
CLI instead of the dashboard.

Verified live against `backboard.railway.com/graphql/v2`:
- `projectTokenCreate(input: {projectId, environmentId, name}): String!` — returns the
  raw token once. **Any token type can mint** (confirmed live via railctl): account and
  workspace tokens mint for the given project+environment; a project token mints within
  its own project+environment scope.
- `projectTokenDelete(id: String!): Boolean!` — revoke by id.
- `projectTokens(projectId: String!): …` — list; exposes `id`, `name`, `environmentId`,
  `createdAt`, and a masked `displayToken` (never the raw value).

## Command surface

A new top-level **`token`** command group (`internal/cmd/token.go`). This is
noun-grouped, unlike the verb-first `create`/`get`/`delete` groups — a deliberate choice
for a clean namespace with room for a future `api-token` sibling.

| Command | Args / flags | Behaviour |
|---|---|---|
| `token create <name>` | `-p` required, `-e` **required** | Mint via `projectTokenCreate`. Raw token → **stdout**; human note → **stderr**. `-o json/yaml` supported. |
| `token list` | `-p` required, `-e` optional filter | `projectTokens(projectId)`. Default table `NAME  ENVIRONMENT  ID  CREATED`; `wide` adds masked `displayToken`. All `-o` formats via `PrintResult`. |
| `token delete <id>` | `-p` **required**, `-y/--yes` | Look up the id within the project (friendly name + "not found in project X" check), confirm `[y/N]`, then `projectTokenDelete(id)`. |

- `-p` is required on all three (delete uses it to resolve the token's name for the
  confirmation prompt and to give an actionable not-found error, mirroring how
  `delete backup` scoped to `--volume`).
- `-e` is required for `create` (a token is bound to one environment) and an optional
  filter for `list`.
- Project/environment resolution goes through `cmdutil.ResolveContext` (which uses the
  `resolver` contract). Output renders via `cmdutil.PrintResult` + `output.NewTable`.
- Each subcommand lives in its own file (`token_create.go`, `token_list.go`,
  `token_delete.go`), sets full `Use/Short/Long/Example`, uses `RunE`, and registers in
  `init()`.

## API layer — `internal/api/tokens.go`

```go
type ProjectToken struct {
    ID            string
    Name          string
    EnvironmentID string
    CreatedAt     string
    DisplayToken  string // masked; never the raw value
}

func (c *Client) CreateProjectToken(projectID, environmentID, name string) (string, error) // → raw token
func (c *Client) ListProjectTokens(projectID string) ([]ProjectToken, error)
func (c *Client) DeleteProjectToken(tokenID string) error
```

Added to the `APIClient` interface and `MockClient` (with `…Func` fields), matching the
existing backups pattern. Internal failures wrapped `fmt.Errorf("...: %w", err)`;
API-level errors flow through the existing `execute` surface. Every method added is wired
to a command — no unused surface (the `LockVolumeBackup` dead-code smell is avoided).

## Auth & error handling

`token create` works with **any** token type. Project/environment resolution goes through
`cmdutil.ResolveContext`: with an account/workspace token, `-p`/`-e` select the target;
with a project token, the new token is minted within that token's own project+environment
(`-p`/`-e` are ignored, per the standard project-token behaviour). Any API error on the
mutation is wrapped with `fmt.Errorf("...: %w", err)`.

> Note: an earlier revision guarded `token create` against project tokens, on the
> assumption they could not mint. Live testing disproved that — a project token mints
> within its own scope — so the guard was removed.

## Secret handling

The raw token is emitted **only by `token create`**, whose sole purpose is to surface it
once — the sanctioned equivalent of the repo's `--show-values` exception (styleguide §1).

- **stdout** carries *only* the token, so it pipes cleanly:
  `TOKEN=$(railctl token create ci -p app -e production)`.
- **stderr** carries the human note: `Created project token 'ci' (project app /
  production). Store it now — it will not be shown again.`
- `token list` never has the raw value (the API returns only the masked `displayToken`).
- No secrets are logged; no hardcoded credentials in source, tests, or examples.

For `-o json/yaml`, `token create` emits `{name, projectId, environmentId, token}` (the
id is not returned by the mutation; `token list` surfaces ids).

## Testing (styleguide §4)

- **Client** (`tokens_client_test.go`, `httptest`): create returns the token; list parses
  the connection; delete fires the mutation with the id.
- **Command** (`token_test.go`, `MockClient`): create success; create `-o json`; list
  `-o json`; delete success; delete **cancelled** (answer `n`); delete **not-found**
  (unknown id). Table-driven where shapes align.

## Docs (styleguide §7)

- `README.md`: new **"Project Tokens"** section — the three commands plus the "shown
  once" security note. No `RAILCTL_*` env var is added, so the env-var table is untouched
  and `docs-guard` is satisfied by the README edit.
- Full help text (`Long`/`Example`) on every command.

## Non-goals

- `api-token` (account/workspace token) management — the `token` group leaves room for it
  but this change does not implement it.
- Rotating/locking tokens.
- Any e2e coverage (project/env token minting is a natural fit for the separate e2e
  two-group refactor, but that is out of scope here).
