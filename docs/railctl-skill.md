---
name: railctl-usage
description: The complete zero-to-hero operating guide for railctl, the kubectl-style CLI for Railway.app — token model and least-privilege doctrine, declarative single-manifest workflow, exhaustive command reference (imperative + declarative), private-registry CI pipelines, domains, monitoring, and deletion safety. Emitted by `railctl skill`.
---

# railctl Usage

`railctl` is a kubectl-style CLI for managing [Railway.app](https://railway.app)
infrastructure: projects, environments, services, variables, volumes, backups,
domains, deployments, and declarative YAML deploys.

This guide is the single source of truth for operating railctl as an agent —
from first contact with a token to a published, monitored, protected service.
Run `railctl skill` to print it; run `railctl <command> --help` for the
authoritative flags of any command. Behavioral claims here are backed by
railctl's live e2e suite (`tests/e2e/`), which runs them against the real
Railway API.

---

## 0. The opinionated model — read this first

railctl is deliberately opinionated. Follow these three principles and the
rest is mostly mechanics:

1. **Railway is your compute provider, not your build provider.** railctl
   deploys **prebuilt Docker images by reference** — it never connects a git
   repo to Railway or triggers Railway-side builds. Build in your own CI,
   publish to a registry (GHCR, Docker Hub, …), deploy the tag. Builds stay
   reproducible, registries portable, Railway swappable.
2. **Declarative from day zero.** One manifest (`stack.yaml`) is the source of
   truth for the whole environment; `railctl diff`/`apply`/`delete -f` form
   the reconcile loop. Imperative commands are for inspection, monitoring,
   and surgical exceptions — not for building up state by hand.
3. **Least privilege, immediately.** The moment a project + environment
   exists, mint a **project token** scoped to exactly that pair and do all
   further work with it. Workspace/account tokens are for provisioning and
   minting only. A leaked project token exposes one project/environment —
   nothing else (verified: Railway itself denies cross-project and
   cross-environment access).

---

## 1. First contact: `railctl whoami`

Whatever token you were handed, classify it before doing anything else:

```bash
railctl whoami            # human table
railctl whoami -o json    # scripts: {"type":"project","workspace":{...},"project":{...},"environment":{...}}
```

It prints the token's **type** (`account` / `workspace` / `project`) and its
containment chain (workspace → project → environment, names + ids) without
ever printing the token value. Everything you may do follows from that type —
see the capability matrix. In scripts, branch on the `type` field.

---

## 2. Authentication & the token model

All auth flows through one env var / flag:

```bash
export RAILWAY_TOKEN=your-token     # or: railctl --token your-token ...
```

**A token is a pointer to one node of the containment tree**
(account → workspace → project + environment): full access at and below its
node (minus a few workspace-reserved mutations), name-visibility of its own
chain, hard denial everywhere else.

| Token | Can list (one level down) | Bound? | What "list" grants |
|---|---|---|---|
| Account | workspaces | not bound — full access to all | full access |
| Workspace | projects | not bound — full access to all | full access |
| Project | environments | **bound to one** | **names only** for siblings — content access solely in its bound environment |

The project token is the only **leaf-bound** token: it is really a
*(project, environment)* token. The API cannot mint an environment-unbound
project token; every project token carries exactly one environment.

### How detection works (so you can debug it)

On the first API call railctl probes in order:

1. **Account probe** — queries `me.workspaces` with `Authorization: Bearer`.
   Succeeds → **account token**.
2. **Workspace probe** — queries projects with `Authorization: Bearer`.
   Succeeds → **workspace token**.
3. **Project probe** — retries with the token in a **`Project-Access-Token`
   header** (a different HTTP header — this is why project tokens fail against
   tools that only send `Authorization: Bearer`). Succeeds → **project token**,
   and the response carries the token's baked-in projectId + environmentId,
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
| `apply` / `diff` / `delete -f` (declarative) | yes | yes | yes — its environment only |
| Deployment **rollback** (`delete deployment`) | yes | yes | yes |
| Deployment **reactivation** (`update deployment --set-active`) | yes | yes | no — workspace-level capability |
| Mint project tokens (`token create`) | yes (any project) | yes (any project in its workspace) | yes — **its own project+environment only** |
| `-w` / `-p` / `-e` flags | honored (selection) | `-w` must match its workspace or the command **errors**; `-p`/`-e` honored | flags must **match** the token's baked scope (then accepted silently) or the command **errors** |

**Key semantics to remember:**

- A **project token** is pinned to exactly one project **and one environment**
  at mint time. You cannot point it elsewhere: a `-w`/`-p`/`-e` value that
  contradicts the baked scope **fails fast** (`token is scoped to … but -e '…'
  was given — refusing to operate …`); a value matching the scope proceeds
  silently. To operate on staging *and* production you need two tokens.
- A **project token cannot** enumerate anything above its project: no
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

### The least-privilege workflow (always)

```bash
# Workspace/account token — provisioning only:
railctl create project my-app                                 # default env: production
CI_TOKEN=$(railctl token create deployer -p my-app -e production)   # shown ONCE

# Everything else — the project token:
export RAILWAY_TOKEN="$CI_TOKEN"
railctl whoami                                                # type=project, right scope
```

The raw token goes to **stdout only, once** — capture immediately, store as a
secret, never echo, never commit. `token list` shows masked values only.
Rotate by minting a replacement and deleting the old id.

---

## 3. Zero → Hero: the canonical path

From "I was handed a token" to "the service is live on its domain, monitored,
and protected". Each step names the token type it needs.

**Step 0 — classify (any token).** `railctl whoami -o json`. Project token?
Your project + environment are fixed — skip to step 3.

**Step 1 — survey (workspace/account).** What infrastructure exists?

```bash
railctl get projects
railctl describe project my-app
railctl get environments -p my-app
railctl get services -p my-app -e production
```

**Step 2 — provision (workspace/account).** Only if the target doesn't exist:

```bash
railctl create project my-app                    # workspace inferred from token
railctl create environment staging -p my-app     # optional extra envs
```

**Step 3 — mint & switch (the least-privilege pivot).**

```bash
TOKEN=$(railctl token create deployer -p my-app -e production)
export RAILWAY_TOKEN="$TOKEN"
railctl whoami                                   # confirm scope
```

From here on no `-p`/`-e` flags are needed — the token carries the scope —
and no command can touch anything outside it.

**Step 4 — author the manifest.** One `stack.yaml` for the whole environment
(schema in §5). Secrets stay out of the file via `$env(VAR)`; cross-service
wiring uses Railway references `${{service.VAR}}`.

**Step 5 — the reconcile loop.**

```bash
railctl diff  -f stack.yaml            # exit 1 while anything would change
railctl apply -f stack.yaml --await    # create/update + wait for SUCCESS
railctl diff  -f stack.yaml            # exit 0: live state matches manifest
```

`diff`'s exit code is the CI gate: 0 = in sync, 1 = drift (an expected
report, not an error — no error styling is printed).

**Step 6 — publish.**

- **Railway domain** (`*.up.railway.app`): declare `networking.domain.port`.
- **Custom domain**: declare `networking.customDomains: [{name, port}]`. On
  apply, railctl creates it and prints the DNS record(s) to add — a `CNAME`/`A`
  for routing and usually a `TXT` for verification, each labeled. Add them at
  the DNS provider; verification follows propagation. Imperative equivalents:
  `create domain` / `get domains` / `delete domain` — **removal is
  imperative-only by design**: `apply` never removes a live domain, so a
  manifest edit can't cause an accidental outage.
- **TCP** (databases etc.): `networking.tcpProxy.port` → Railway assigns a
  public `host:port`.

**Step 7 — private images (the CI pipeline step).** Railway is compute-only
here, so private images come from **your** CI:

1. Create a pipeline that builds and pushes on each release — e.g. GitHub
   Actions to GHCR:

   ```yaml
   # .github/workflows/publish.yml (essentials)
   permissions: { packages: write, contents: read }
   steps:
     - uses: actions/checkout@v4
     - uses: docker/login-action@v3
       with: { registry: ghcr.io, username: ${{ github.actor }}, password: ${{ secrets.GITHUB_TOKEN }} }
     - uses: docker/build-push-action@v6
       with: { push: true, tags: "ghcr.io/OWNER/app:${{ github.sha }}" }
   ```

2. **Ask the user for a least-privilege PULL credential** — do not create one
   yourself and do not accept more scope than needed: GHCR → a PAT with only
   `read:packages`; Docker Hub → a read-only access token. Say explicitly
   that it is pull-only and why.
3. Wire it as a secret, never into the manifest:

   ```yaml
   registry:
     username: "$env(REGISTRY_USER)"
     password: "$env(REGISTRY_PASS)"
   ```

   with the two variables exported locally / set as CI secrets. Imperative:
   `--registry-username/--registry-password` (or
   `RAILCTL_REGISTRY_USERNAME`/`RAILCTL_REGISTRY_PASSWORD`). Private
   registries require a Railway Pro plan.
4. Releasing = bump the image tag in `stack.yaml` + `apply --await`
   (hotfix path: `railctl update service app --image ghcr.io/o/app:SHA
   --await-completion`, then backport the tag to the manifest so `diff`
   returns to 0).

**Step 8 — monitor & operate** (§6: deployments & logs).

**Step 9 — protect & housekeep** (§7): set `DELETE_PROTECTION` on the
environment, declare `backupSchedules` on stateful volumes, rotate tokens,
tear down with `delete -f` when the environment is disposable.

---

## 4. Context resolution: flags → env vars → token scope

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
| `-s` / `--service` | `RAILCTL_SERVICE` | Service name or ID (never baked into a token) |
| `-o` / `--output` | — | `table` (default), `wide`, `json`, `yaml` |
| `--debug` | — | Dump GraphQL requests/responses to stderr |

Name arguments resolve **exact match → case-insensitive substring**; ambiguous
matches error listing candidates. Unknown names list what exists:
`project 'foo' not found — available: api, web, …` (capped at 10). Machine
formats stay machine-readable: listings emit `[]` on empty, never prose.

---

## 5. Declarative reference — the manifest

```yaml
# stack.yaml — one file, the whole environment
project: my-app          # optional; -p / env var / token scope override
environment: production  # optional; same

services:
  - name: api
    image: ghcr.io/owner/app:sha-abc123   # always a prebuilt image reference

    deploy:
      startCommand: "npm start"
      restartPolicy: ON_FAILURE       # ON_FAILURE | ALWAYS | NEVER
      maxRetries: 3                   # requires restartPolicy
      replicas: 2                     # >= 1 if set
      healthcheckPath: /health
      healthcheckTimeout: 300

    networking:
      domain:
        port: 3000                    # Railway domain (*.up.railway.app)
      tcpProxy:
        port: 5432                    # public TCP proxy to this app port
      customDomains:
        - name: app.example.com       # DNS records printed on apply
          port: 3000                  # optional; defaults to domain.port

    volume:
      mountPath: /app/data
      backupSchedules: [daily, weekly]   # daily/weekly/monthly

    variables:
      PORT: "3000"
      DATABASE_URL: "${{db.DATABASE_URL}}"   # Railway-side service reference
      API_KEY: "$env(API_KEY)"               # expanded from local env at apply

    registry:                          # private registries (Pro plan)
      username: "$env(REGISTRY_USER)"
      password: "$env(REGISTRY_PASS)"
```

### The three verbs

| Command | Does | Exit |
|---|---|---|
| `railctl diff -f <file-or-dir> [--prune]` | show create/update/delete deltas, secrets masked | 0 = in sync, 1 = drift |
| `railctl apply -f <file-or-dir> [--await] [--await-timeout N] [--dry-run] [--prune --yes]` | reconcile live state to the manifest | 0 = applied |
| `railctl delete -f <file-or-dir> [--yes]` | delete exactly the **declared** services (reverse manifest order), then their declared volumes | 0 = done / cancelled |

### Semantics that matter

- **Declared state is authoritative for managed fields.** A service with a
  declared `volume.mountPath` is a *managed volume*: omitting
  `backupSchedules` (or `[]`) **clears live schedules** on the next apply —
  with an explicit warning naming what was removed. A service with no
  `volume:` block is left untouched.
- **`apply` never removes custom domains** — removal is `railctl delete
  domain`, deliberately imperative-only.
- **`--prune`** deletes live services not declared in the manifest — the only
  apply-path deletion; prompts unless `--yes`.
- **`delete -f`** touches only what the manifest declares: services in
  reverse order, then their declared volumes (a deleted service orphans its
  volume). It never deletes environments or projects, skips absent resources
  with a note, needs no `$env()` secrets, and prints an itemized plan before
  the `[y/N]` prompt.
- Volumes **cannot change mountPath in place**; a deleted service **orphans**
  its volume.

---

## 6. Command reference (imperative)

Verb-first, kubectl-style: `railctl <verb> <resource> [name] [flags]`.
Listing/describing commands accept `-o table|wide|json|yaml`.

### Identity & meta
```bash
railctl whoami [-o json]        # token type + scope chain; never prints the token
railctl skill                   # print this guide
railctl --version | completion <shell> | <cmd> --help
```

### Projects & environments — workspace/account token required
```bash
railctl get projects                          # project tokens: fails fast by design
railctl describe project my-app
railctl create project my-app                 # workspace inferred from the token
railctl delete project my-app --yes           # refuses if delete-protected or has services
railctl get environments -p my-app            # names visible to any token
railctl describe environment production -p my-app
railctl create environment staging -p my-app
railctl delete environment staging -p my-app --yes   # refuses if delete-protected
```

### Services — any token, within scope
```bash
railctl get services
railctl describe service api [--show-values]
railctl create service api --image ghcr.io/o/app:tag \
    [--start-command CMD] [--restart-policy ON_FAILURE|ALWAYS|NEVER] [--max-retries N] \
    [--replicas N] [--healthcheck-path /health] [--healthcheck-timeout N] \
    [--generate-domain PORT] [--generate-tcp PORT] \
    [--registry-username U --registry-password P]
railctl update service api [--image TAG] [--await-completion] [...same config flags] \
    [--remove-domain] [--remove-tcp]          # update triggers a deployment
railctl delete service api --yes              # orphans its volume — see volumes
```
The service is created **in the target environment only**.

### Variables — service-scoped
```bash
railctl get variables -s api                  # values masked; --show-values reveals
railctl set variable KEY=VALUE [K2=V2 ...] -s api [--skip-deployment]
railctl delete variable KEY -s api --yes
```
`${{service.VAR}}` references are stored as-is and resolve on Railway.
Shared (environment-level) variables are readable by railctl (they power
`DELETE_PROTECTION`) but **cannot be written via the CLI yet** — set them in
the Railway dashboard.

### Volumes & backups
```bash
railctl get volumes | railctl describe volume data
railctl create volume --mount-path /data -s api
railctl update volume data [--name NEW] [--attach -s api | --detach] [--mount-path /p]
railctl delete volume data --yes
railctl get backups data [--schedules]
railctl create backup data [--name pre-migration]        # async — poll get backups
railctl restore backup <backup-id> --volume data --yes
railctl delete backup <backup-id> --volume data --yes
```
**Backups are welded to their volume instance in its environment** (verified):
no cross-volume restore, no following an environment name — deleting the
environment effectively destroys its backups, and recreating a same-named
environment does **not** resurrect them. Treat `delete environment` as
deleting all its backups; export data that must survive. **Restore is
staged**: deploy the service to finalize, and backups newer than the restore
point are removed. Schedule retention is fixed per kind (~6d/1m/3m). Prefer
managing schedules declaratively (`volume.backupSchedules`).

### Deployments & logs (monitoring)
```bash
railctl get deployments -s api [--limit N]    # -o json: [] when empty (script-safe)
railctl create deployment -s api [--await-completion]    # explicit redeploy
railctl delete deployment <id> -s api --yes   # rollback if latest; status → REMOVED
railctl update deployment <id> --set-active   # reactivation — workspace token required
railctl logs api [--tail N(≤500)] [-f] [--deployment <id>]
```
Deployment statuses: `INITIALIZING → BUILDING → DEPLOYING → SUCCESS`, or
`FAILED / CRASHED / REMOVED / SKIPPED`. Poll
`get deployments -o json --limit 1` for the latest status in scripts.
`create service` does **not** reliably deploy by itself — trigger explicitly
with `create deployment` when you need a deterministic first deployment.

### Domains
```bash
railctl get domains -s api                    # railway + custom, verification status
railctl create domain app.example.com -s api [--port N]   # prints DNS records
railctl delete domain app.example.com -s api --yes        # not-found lists available
```

### Tokens
```bash
railctl token create <name> -p my-app -e production   # raw token → stdout, ONCE
railctl token list -p my-app [-e env] [-o wide]       # values masked
railctl token delete <id> -p my-app --yes
```
Works with any token type; a project token self-mints for its own scope
(no flags). Rotate: mint new → switch consumers → delete old id.

---

## 7. Danger zone — deletion semantics & protection

- **`DELETE_PROTECTION`**: an environment whose shared (environment-level)
  variable `DELETE_PROTECTION` is truthy (`true`/`1`/`yes`/`on`,
  case-insensitive) cannot be deleted, nor can its project — railctl refuses
  with **no bypass flag** (`--yes` skips prompts, never protection); unset
  the variable to allow. Unreadable protection state → deletion refused
  (fail-closed). Set it on every environment you care about — today via the
  Railway dashboard (shared variables).
- `delete project` also refuses while the project still has services —
  delete them (or `delete -f` the manifest) first.
- Deleting an **environment** destroys its variable values, volume instances,
  **and their backups** (unrecoverable — §6 backups).
- Deleting a **service** orphans its volume; data survives until the volume
  is deleted explicitly.
- `delete -f` never touches environments/projects; `apply` never removes
  custom domains; `apply --prune` is the only apply-path deletion and prompts.
- Everything destructive prompts `[y/N]` and supports `--yes` for automation.

---

## 8. Troubleshooting

| Symptom | Cause / fix |
|---|---|
| `token is not authorized` | Expired/revoked token, or all three detection probes failed. `railctl whoami`, re-mint. |
| `token is scoped to … but -p/-e/-w '…' was given` | Contradiction fail-fast: flags/env vars disagree with the token's baked scope. Fix the stale `RAILCTL_*` value or use the right token. |
| `cannot … with a project token` | Workspace-scope operation (project/env lifecycle, `get projects`, deployment reactivation). Use a workspace/account token. |
| `… not found — available: a, b, c` | Typo — the listed candidates are what exists. |
| `environment '…' is delete-protected` | `DELETE_PROTECTION` is set — unset it in the dashboard to allow deletion. |
| Token works in the dashboard but railctl says unauthorized | Probably project-scoped and the other tool sends `Authorization: Bearer` only; railctl handles the `Project-Access-Token` header automatically — check for typos/whitespace. |
| `diff` "fails" in CI | Exit 1 means drift, by design (like `git diff --exit-code`): `railctl diff -f stack.yaml \|\| railctl apply -f stack.yaml --await`. |
| Volume/backup op right after creation says not found | Propagation lag; railctl retries with backoff — re-run if it still misses. |
| Backup restore "did nothing" | Restore is staged — **deploy the service** to finalize. |
| Apply cleared backup schedules unexpectedly | The volume is managed and the manifest omitted `backupSchedules` — declared state is authoritative; re-declare them. |
| Custom domain stuck pending | DNS records not added/propagated — `get domains -s <svc>` shows verification status. |
| `--debug` | Global flag: dumps GraphQL traffic to stderr. |

---

## 9. Recipes

**Zero-to-hero in nine lines** (new project, public image):
```bash
railctl whoami                                             # workspace token
railctl create project my-app
export RAILWAY_TOKEN=$(railctl token create deployer -p my-app -e production)
railctl whoami                                             # project token, right scope
$EDITOR stack.yaml                                         # §5 schema
railctl diff  -f stack.yaml
railctl apply -f stack.yaml --await
railctl get domains -s web                                 # where it's published
railctl logs web --tail 50                                 # it's alive
```

**CI deploy gate** (project token in CI secrets; no flags anywhere):
```bash
export RAILWAY_TOKEN="$RAILWAY_PROJECT_TOKEN"
railctl diff -f stack.yaml || railctl apply -f stack.yaml --await
```

**Private image end-to-end**: CI builds & pushes `ghcr.io/owner/app:$SHA` →
ask the user for a read-only pull credential (`read:packages`) → export as
`REGISTRY_USER`/`REGISTRY_PASS` secrets → `registry:` block with `$env()` in
the manifest → bump the tag per release → `apply --await`.

**Pre-migration safety**:
```bash
railctl create backup pg-data --name pre-migration
railctl get backups pg-data                    # wait until it appears
# …run the migration; if it goes wrong:
railctl restore backup <id> --volume pg-data --yes
railctl create deployment -s db --await-completion   # deploy finalizes restore
```

**Token rotation**:
```bash
NEW=$(railctl token create deployer-2)         # project token self-mints its scope
# switch consumers to $NEW, then:
railctl token list -o json                     # find the old id
railctl token delete <old-id> --yes
```

---

## 10. Related

- `docs/token-capability-matrix.md` — the verified capability matrix behind
  this guide's token model; each row cites the e2e test that proves it.
- `docs/declarative-config.md` — full manifest schema reference.
- `tests/e2e/` — the three-layer live suite (account / workspace / project
  token groups); the executable form of everything stated here.
- Repo: `github.com/kubenoops/railctl` (releases: linux/darwin × amd64/arm64).
