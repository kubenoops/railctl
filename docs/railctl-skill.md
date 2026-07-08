---
name: railctl-usage
description: How to use railctl, the kubectl-style CLI for Railway.app — full command surface, declarative apply/diff, volume backups, and the token model (account/workspace vs project-scoped tokens, detection, and per-type limitations). Emitted by `railctl skill`.
---

# railctl Usage

`railctl` is a kubectl-style CLI for managing [Railway.app](https://railway.app)
infrastructure: projects, environments, services, variables, volumes, backups,
domains, and declarative YAML deploys.

Run `railctl skill` to print this guide. Run `railctl <command> --help` for the
authoritative flags on any command.

---

## 1. Authentication & the token model (read this first)

All auth flows through one env var / flag:

```bash
export RAILWAY_TOKEN=your-token     # or: railctl --token your-token ...
```

Railway issues **three token scopes**, which railctl groups into **two access
models** — *workspace-level* (account + workspace tokens) and *project-level*
(project tokens). railctl **auto-detects** the type on first use; no
configuration is needed.

### How detection works (so you can debug it)

On the first API call railctl probes in order:

1. **Account probe** — queries `me.workspaces` with `Authorization: Bearer`.
   Succeeds → **account token**.
2. **Workspace probe** — queries projects with `Authorization: Bearer`.
   Succeeds → **workspace token**.
3. **Project probe** — retries with the token in a **`Project-Access-Token`
   header** (a different HTTP header — this is why project tokens fail against
   tools that only send `Authorization: Bearer`). Succeeds → **project token**,
   and the response carries the token's baked-in **projectId + environmentId**,
   which railctl caches as the working context.

All three probes failing → `token is not authorized` (expired/revoked token).
The result is cached for the process; detection runs once per invocation.

### Capability matrix — what each token can and cannot do

| Capability | Account token | Workspace token | Project token |
|---|---|---|---|
| List/switch workspaces (`-w`) | yes | no — scoped to its workspace | no |
| List projects | yes (all workspaces) | yes (its workspace) | no |
| Create / delete **projects** | yes | yes | no |
| Create / delete **environments** | yes | yes | no |
| Services / variables / volumes / backups / domains / logs / deploys | yes | yes | yes — within its one project+environment |
| `apply` / `diff` (declarative) | yes | yes | yes — its environment only |
| Deployment **rollback** (`delete deployment`) | yes | yes | yes |
| Deployment **reactivation** (`update deployment --set-active`) | yes | yes | no — workspace-level capability |
| Mint project tokens (`token create`) | yes (any project) | yes (any project in its workspace) | yes — **its own project+environment only** |
| `-w` / `-p` / `-e` flags | honored (selection) | `-w` must match its workspace or the command **errors**; `-p`/`-e` honored | flags must **match** the token's baked scope (then accepted silently) or the command **errors** |

**Key semantics to remember:**

- A **project token** is pinned to exactly one project **and one environment**
  at mint time (the API cannot mint an environment-unbound project token). You
  cannot point it elsewhere: a `-w`/`-p`/`-e` value that contradicts the baked
  scope **fails fast** (`token is scoped to … but -e '…' was given — refusing
  to operate …`); a value matching the scope proceeds silently. To operate on
  staging *and* production you need two tokens.
- A **project token** cannot enumerate anything above its project: no
  `get projects`, no workspace queries, no project/environment lifecycle —
  these fail fast (`cannot … with a project token — it is scoped to a single
  project and environment; use an account or workspace token`).
- A **workspace token** behaves like an account token *inside* its workspace
  but cannot see or switch workspaces — a mismatching `-w` **errors**, a
  matching one is accepted silently. `create project` infers the workspace
  from the token; no `-w` needed.
- **Any token can mint project tokens** within its reach: account/workspace
  tokens target any project they can see (`-p`/`-e` required); a project token
  self-mints for its own scope only (no flags needed).

### Which token to use

| Situation | Use |
|---|---|
| Interactive ops across projects | Account (or workspace) token |
| CI/CD deploying one app | **Project token** — smallest blast radius; a leak exposes one project/env |
| Project/env lifecycle automation (create/tear down projects) | Account/workspace token — nothing smaller works |
| Untrusted scripts, e2e suites against a live project | **Project token** — structurally cannot touch other projects |

Get account tokens at `https://railway.app/account/tokens`. Project tokens are
minted in the Railway dashboard (project → settings → tokens), or with
`railctl token create <name> -p <project> -e <env>` where that command group is
available (`railctl token --help`).

---

## 2. Context resolution: flags → env vars → token scope

Every command resolves context in the order **flag → `RAILCTL_*` env var →
default**. With a project token, project/environment come from the token
itself; a flag or env var naming the *same* project/environment is accepted
silently, while a *different* one fails fast (contradiction) — stale
`RAILCTL_PROJECT`/`RAILCTL_ENVIRONMENT` values cannot silently redirect
commands to the token's scope.

| Flag | Env var | Meaning |
|---|---|---|
| `--token` | `RAILWAY_TOKEN` | API token (required) |
| `-w` / `--workspace` | `RAILCTL_WORKSPACE` | Workspace (account tokens with >1 workspace) |
| `-p` / `--project` | `RAILCTL_PROJECT` | Project name or ID |
| `-e` / `--environment` | `RAILCTL_ENVIRONMENT` | Environment name or ID |
| `-s` / `--service` | `RAILCTL_SERVICE` | Service name or ID |
| `-o` / `--output` | — | `table` (default), `wide`, `json`, `yaml` |

```bash
export RAILCTL_PROJECT=my-app RAILCTL_ENVIRONMENT=production
railctl get services      # flags now optional
```

Name arguments resolve **exact match → case-insensitive substring**; ambiguous
matches error out listing candidates, so unique prefixes are safe. Unknown
names also list what exists: `project 'foo' not found — available: api, web, …`
(capped at 10).

---

## 3. Command surface

Verb-first, kubectl-style: `railctl <verb> <resource> [name] [flags]`.

### Projects & environments (need an account/workspace token)
```bash
railctl get projects                        # -o wide/json/yaml
railctl describe project my-app
railctl create project my-new-app
railctl delete project old-app --yes
railctl get environments -p my-app          # alias: envs
railctl create environment staging -p my-app
railctl delete environment staging -p my-app --yes
```

### Services
```bash
railctl get services -p my-app -e production             # alias: svc
railctl describe service api -p my-app -e production
railctl create service api --image node:20-alpine -p my-app
railctl create service api --image node:20 \
  --start-command "npm start" --restart-policy ON_FAILURE --max-retries 3 \
  --replicas 2 --healthcheck-path /health --healthcheck-timeout 60 -p my-app
railctl update service api --image node:20 -p my-app -e production   # triggers deploy
railctl delete service api -p my-app -e production --yes
```
Restart policies: `ON_FAILURE`, `ALWAYS`, `NEVER`. Railway creates a service in
**all** environments; railctl cleans up non-target instances automatically.

### Variables
```bash
railctl get variables -p my-app -e production -s api     # alias: vars
railctl set variable DATABASE_URL=postgres://... -p my-app -e production -s api
railctl set variable A=1 B=2 --skip-deployment -p my-app -e production -s api
railctl delete variable OLD_KEY -p my-app -e production -s api --yes
```
Sensitive values are masked; use `--show-values` to reveal deliberately.
`${{service.VAR}}` references are Railway-side and passed through as-is.

### Logs & deployments
```bash
railctl logs service api --tail 50 -p my-app -e production
railctl logs service api -f -p my-app -e production          # follow
railctl get deployments -s api -p my-app -e production
```

### Volumes
```bash
railctl get volumes -p my-app -e production
railctl create volume --mount-path /app/data -s backend -p my-app -e production
railctl update volume my-data --name uploads ...             # rename
railctl update volume my-data --attach -s backend ...        # attach/detach
railctl delete volume my-data --yes -p my-app -e production
```
Volumes cannot change mount path in place via apply; deleting a service orphans
its volume — delete volumes explicitly.

### Volume backups
```bash
railctl get backups my-data -p my-app -e production          # list backups
railctl get backups my-data --schedules                      # list automated schedules
railctl create backup my-data --name pre-migration ...       # manual backup (async)
railctl delete backup <backup-id> --volume my-data --yes ...
railctl restore backup <backup-id> --volume my-data ...
```
- Schedules are `DAILY` / `WEEKLY` / `MONTHLY`; retention is **fixed by Railway
  per kind** (~6 days / 1 month / 3 months), not configurable.
- **Restore semantics:** Railway stages a new volume — you must **deploy the
  service to finalize**, and backups newer than the restore point are removed.
- **A backup is welded to its volume instance in its environment** (verified):
  it cannot be restored onto a different volume, cannot follow an environment
  name (recreating a same-named environment does not resurrect it), and
  deleting the environment effectively destroys it. Treat
  `delete environment` as deleting all its backups too — export data that must
  survive.
- Prefer managing schedules declaratively (next section).

### Project tokens
```bash
railctl token create ci -p my-app -e production   # raw token → stdout, ONCE
railctl token list -p my-app                      # values masked
railctl token delete <id> -p my-app --yes
```
Works with any token type; capture stdout immediately
(`TOKEN=$(railctl token create ci -p my-app -e production)`). Under a project
token, `token create <name>` self-mints for the token's own scope (flags
unnecessary; mismatching flags error).

---

## 4. Declarative configuration (`apply` / `diff`)

Infrastructure-as-code for a whole environment:

```yaml
# stack.yaml
services:
  - name: db
    image: postgres:16
    networking:
      tcpProxy:
        port: 5432
    volume:
      mountPath: /var/lib/postgresql/data
      backupSchedules: [daily, weekly]   # reconciled to exactly this list
    variables:
      POSTGRES_USER: "app"
      POSTGRES_PASSWORD: "$env(POSTGRES_PASSWORD)"   # expanded from local env
  - name: api
    image: node:20-alpine
    deploy:
      startCommand: "npm start"
      replicas: 2
    networking:
      domain:
        port: 3000
      customDomains:
        - name: app.example.com          # prints DNS records to configure
    variables:
      PORT: "3000"
      DATABASE_URL: "${{db.DATABASE_URL}}"           # Railway-side reference
```

```bash
railctl diff  -f stack.yaml -p my-app -e production   # exit != 0 when changes exist
railctl apply -f stack.yaml -p my-app -e production --dry-run
railctl apply -f stack.yaml -p my-app -e production --await   # wait for deploy
railctl apply -f dir-of-yamls/ -p my-app -e production        # whole directory
```

Semantics worth knowing:

- **Declared state is authoritative** for managed fields. A service with a
  declared `volume.mountPath` is a *managed volume*: omitting `backupSchedules`
  (or `[]`) **clears existing schedules** on the next apply — railctl prints an
  explicit warning naming what was removed. A service with no `volume:` block is
  left untouched.
- `diff` exits non-zero when changes exist (script-friendly); apply prints
  per-change progress and only deploys when staged changes require it.
- Secrets: `$env(NAME)` pulls from your local environment at apply time — keep
  secrets out of the YAML; pair with `.envrc`.
- `--prune` deletes services not present in the config — use deliberately.

---

## 5. Recipes

**CI deploy with least privilege** (project token):
```bash
# One-time, with a workspace/account token:
CI_TOKEN=$(railctl token create ci -p my-app -e production)
# In CI (no -p/-e needed or honored — baked into the token):
export RAILWAY_TOKEN=$CI_TOKEN
railctl apply -f stack.yaml --await
```

**Pre-migration safety backup:**
```bash
railctl create backup pg-data --name pre-migration -p my-app -e production
railctl get backups pg-data -p my-app -e production   # wait until it appears
# ...run migration; if it goes wrong:
railctl restore backup <id> --volume pg-data -p my-app -e production
railctl update service db --image postgres:16 -p my-app -e production  # deploy finalizes restore
```

**Full stack from scratch:**
```bash
railctl create project my-app                       # account/workspace token
railctl apply -f stack.yaml -p my-app -e production --await
railctl get services -p my-app -e production -o wide
```

---

## 6. Troubleshooting

| Symptom | Cause / fix |
|---|---|
| `token is not authorized` | Token expired/revoked, or all three detection probes failed. Re-mint the token. |
| `-p`/`-e` "ignored" warning | You're on a project token — scope is baked in. Use a workspace/account token for another project/env. |
| `get projects` fails on a token that deploys fine | Project token: cannot enumerate projects. Expected. |
| Token works in dashboard but railctl says unauthorized | Likely project-scoped; railctl handles the `Project-Access-Token` header automatically — check for whitespace/typos in `RAILWAY_TOKEN`. |
| Volume/backup op right after creation says instance not found | Propagation lag; railctl retries ~3× with backoff — re-run if it still misses. |
| Backup restore "did nothing" | Restore is staged: **deploy the service** to finalize. |
| Apply cleared backup schedules unexpectedly | The volume is managed and the config omitted `backupSchedules` — declared state is authoritative. Re-declare them. |
| Ambiguous name error | Multiple resources match the substring; use the full name or ID from `-o wide`. |
| Need API-level detail | `--debug` dumps GraphQL requests/responses to stderr. |
