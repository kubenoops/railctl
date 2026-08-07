# Railway token capability matrix

**Verified live against `backboard.railway.com/graphql/v2` on 2026-07-08.**
Every row is proven either by an e2e test (cited) or by a direct API probe
performed during the verification campaign. The three-layer e2e suite
(`tests/e2e/{account,workspace,project}`) is the executable form of this
matrix — when behaviour drifts, those tests fail.

## The mental model

**A token is a pointer to exactly one node of the containment tree**
(account → workspace → project + environment), with:

- full access at and below its node — minus a few workspace-reserved
  mutations (project/environment lifecycle, deployment reactivation),
- name-visibility of its own containment chain,
- hard denial everywhere else.

| Token | Can list (one level down) | Bound? | What "list" grants |
|---|---|---|---|
| Account | workspaces | not bound — full access to all of them | full access |
| Workspace | projects | not bound — full access to all of them | full access |
| Project | environments | **bound to one** | **names only** for siblings — content access solely in its bound environment |

The project token is the only **leaf-bound** token: it is really a
*(project, environment)* token. `ProjectTokenCreateInput.environmentId` and
`ProjectToken.environmentId` are both non-null in the schema — a project
token **cannot** be minted without an environment and cannot exist without
one. There is no "all environments" variant (verified: mint without
`environmentId` is rejected).

## Operation matrix

| Operation | Account | Workspace | Project | Proof |
|---|---|---|---|---|
| List/switch workspaces (`-w` selection) | ✅ | ❌ (bound) | ❌ (bound) | `account/TestWorkspaceDisambiguation` |
| List projects | ✅ | ✅ | ❌ guard: fail-fast | `project/TestBoundaries/get_projects_denied` |
| Create/delete project | ✅ | ✅ (workspace inferred, no `-w` needed) | ❌ guard | `workspace/TestProjects`, `account/.../create_project_with_w_roundtrip` |
| Create/delete environment | ✅ | ✅ | ❌ guard | `workspace/TestEnvironments` |
| Describe project (by name) | ✅ | ✅ | ❌ guard (needs enumeration) | `workspace/TestProjects/describe_*` |
| Services / variables / volumes / backups / deployments / logs / apply / diff | ✅ | ✅ | ✅ within its (project, environment), flag-free | entire `project/` group |
| Deployment rollback (`delete deployment`) | ✅ | ✅ | ✅ | `project/TestDeploymentLifecycle` |
| Deployment **reactivation** (`update deployment --set-active`) | ✅ | ✅ | ❌ `Not Authorized` (workspace-reserved) | `project/.../reactivate_previous_denied`, `workspace/TestDeploymentReactivate` |
| Mint project token (`token create`) | ✅ any project | ✅ any project in workspace | ❌ **denied by Railway** — cannot self-mint | `workspace/TestProjectTokens`, `project/TestBoundaries/self_mint` |
| List / delete project tokens (`token list`/`delete`) | ✅ | ✅ | ❌ **denied by Railway** (delete resolves by listing) | `workspace/TestProjectTokens` |
| List / delete project tokens | ✅ | ✅ | ✅ within its project | same |
| Mint workspace/account token (`apiTokenCreate`) | ✅ (`workspaceId` set → workspace token; omitted → account token) | untested | ❌ (assumed) | direct API probe |
| Exec / port-forward into a service (`exec`, `port-forward`) | ✅ | ✅ | ✅ | works with **any** token — the token only resolves the instance; auth is by a **pre-registered SSH key** the *user* registers at railway.com/account/ssh-keys (railctl does **not** manage keys). Design `2026-07-09-railctl-exec-port-forward.md` |

## Scope-boundary enforcement (Railway-side, verified by probe)

| Probe | Result |
|---|---|
| Project token → read a **different project** (`project(id)`, `environments(projectId)`) | ❌ `Not Authorized` |
| Project token → **content of a sibling environment** (variables, `environment(id)` node) | ❌ `Not Authorized` |
| Project token → deployments listing of a sibling environment | ⚠️ allowed but **silently empty** (filtered) |
| Project token → environment **names** of its project | ✅ (metadata only) |
| Project token → its own project/env content | ✅ |

## Token self-introspection (what a token knows about itself)

| Query | Account | Workspace | Project |
|---|---|---|---|
| `apiToken { workspaces { id name } }` | ✅ all its workspaces | ✅ exactly its one workspace | — |
| `projectToken { projectId environmentId project { name workspace { id name } } environment { name } }` | — | — | ✅ full chain, ids + names |
| `me { workspaces }` | ✅ | ❌ (this denial is detection probe 1) | ❌ |
| Direct `workspace(workspaceId)` | ✅ | ✅ | ❌ — only the nested path through its own project |

**Consequence:** every token type can always name its own containment chain.
This is what makes railctl's flag semantics implementable with no fallbacks.

## railctl flag semantics (the UX contract)

Because bound tokens self-identify, railctl distinguishes three error classes:

1. **Out-of-scope operation** (project token → workspace-scope command):
   fail fast via `cmdutil.RequireWorkspaceScope` —
   `cannot <op> with a project token — it is scoped to a single project and environment; use an account or workspace token`.
2. **Contradiction** (flag names something ≠ the token's binding — `-w` on
   workspace/project tokens, `-p`/`-e` on project tokens): **fail fast**, never
   warn-and-proceed —
   `token is scoped to project 'X' (id) but -p/--project 'Y' was given — refusing to operate on a different project than requested…`.
   A flag that **matches** the binding (id or unique name match) proceeds
   silently. Rationale: warn-and-proceed silently redirected mutations to a
   different target than the user named (e.g. `delete volume data -p my-app`
   under a token scoped elsewhere).
3. **Not-found within an enumerable scope** (account/workspace token naming a
   child that doesn't exist): `project 'foo' not found — available: api, web, …`
   (candidates capped at 10). Never a raw API `Not Authorized`.

## The service / instance model (verified 2026-07-08)

- A **service is a project-level entity**; its **name is unique per project**
  (creating a same-named service targeting a second environment fails with
  "already exists in this project"). Environments hold **instances** of it.
- `serviceCreate(environmentId)` creates the entity plus an instance **in the
  target environment only**. The historical fork-era behavior of creating
  instances in every environment is **fixed upstream** (Railway deprecated
  forked environments in January 2024 in favor of isolated environments;
  cross-environment changes now happen only via the manual, staged Sync).
  railctl's old post-create cleanup workaround was removed accordingly.
- Instances appear in multiple environments only **deliberately** (duplicate
  environment / Sync). `serviceDelete(id, environmentId)` removes exactly one
  environment's instance, **symmetrically** — deleting the original
  environment's instance does not affect duplicates and vice versa; the
  project-level entity survives. `serviceDelete(id)` without an environment
  removes the whole service.
- Caution when interpreting probe results: Railway **auth-masks**
  cross-environment access for project tokens — `Not Authorized` is returned
  whether the target exists or not. A failed cross-env delete is not evidence
  that anything was there.

## Notes

- Token-type detection (railctl `internal/api/client.go`): probe 1
  `me.workspaces` (account) → probe 2 projects listing (workspace) → probe 3
  `Project-Access-Token` header + `projectToken` context (project).
- A project token can `projectCreate` a **new** project via the raw API (it
  then cannot access it) — a Railway quirk, not a cross-tenant hole
  (cross-project access is denied). railctl does not expose this path.
- The e2e suite runs each group under exactly its token type
  (`RAILWAY_ACCOUNT_TOKEN`, `RAILWAY_WORKSPACE_TOKEN`; the project group
  mints its own project token in `TestMain`), with preflight classification
  that refuses to run under a mismatched token.
