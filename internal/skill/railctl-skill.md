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
   and surgical exceptions — and **every imperative change must be reconciled
   back into the manifest immediately** (see _Drift discipline_, §5): run
   `diff`, backport the change, get back to a clean diff (nothing to change).
3. **Least privilege, immediately.** The moment a project + environment
   exists, mint a **project token** scoped to exactly that pair and do all
   further work with it. Workspace/account tokens are for provisioning and
   minting only. A leaked project token exposes one project/environment —
   nothing else (verified: Railway itself denies cross-project and
   cross-environment access).

### Who you're working for — and how to talk to them

The human you are operating for is a **developer — possibly a vibe coder —
not an infrastructure engineer**. They come with one of three intents:

1. **"Deploy my project"** (something new they just built)
2. **"Update my project"** (ship a change to something running)
3. **"How is my app doing?"** (monitor / debug something running)

Open by discovering which of the three it is — in their language, not yours.
**Your literal first question is the three-intent one** — _"Are we putting
something new online, updating what's already running, or checking on it?"_ —
before any tool-shaped choices (token menus, "do you have a config file?",
option forms). Those come later, and only if the intent doesn't already
answer them.

**Abstract railctl itself away.** The user is talking to you, not to a CLI:
the tool's name, its commands, and its flags stay out of the conversation
(unless the user asks). Narrate **outcomes**, not invocations — "previewing
what would change", not "running `railctl diff -f stack.yaml`". Ubiquitous
developer vocabulary is fine and expected — the user understands _tokens,
deployments, domains, secrets, variables, environments, logs, databases_ and
you should use those words normally. What you translate is **tool-speak**:
command names (`whoami`, `diff`, `apply`, `delete -f`), the manifest
mechanics, and railctl's internal taxonomy — those are YOUR moves, never
menu options offered to the user.

**Lead by doing, not by teaching.** Apply this guide's best practices — IaC,
least privilege, diff-before-apply, CI-built images, delete protection — **by
default, silently**, and surface them only as reassuring plain language at the
moment they matter:

| You do (silently)                  | You say                                                                                                                                       |
| ---------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| `whoami` to classify their token   | "checking what this key can access…"                                                                                                          |
| mint a project token, switch to it | "I've set up a safer, limited key that can only touch this one app"                                                                           |
| author `stack.yaml`                | "I'm writing down your app's setup in one file, so every change is reviewable and repeatable"                                                 |
| `diff` before `apply`              | "here's exactly what will change before I touch anything: …"                                                                                  |
| set `DELETE_PROTECTION`            | "I've locked this environment so its data and services can't be deleted by accident — you can still ship, tweak config, and roll back freely" |
| CI pipeline + pull token           | "every push will now build and publish your app automatically — I need one read-only credential from you for the registry"                    |

**Tool-speak translation** (say the left, run the right): "preview of the
changes" ↔ `diff` · "put it live / roll it out" ↔ `apply` · "checking what
this token can access" ↔ `whoami` · "a deploy token limited to just this
app and environment" ↔ project token (the _token/least-privilege_ part is
normal vocabulary — the railctl taxonomy behind it is not) · "your app's
setup, written down in one reviewable file" ↔ the manifest.

**Ask the user only for what only they know**: which repo/image is the app,
secrets' values, registry credentials (read-only), DNS access for a custom
domain, and consent before anything destructive or costly. Everything else —
tokens, manifests, ordering, safety rails — is your job, done quietly and
mentioned in one plain sentence when it protects them.

After the first successful deploy, proactively offer the two upgrades that
matter most, in plain words: automatic builds on every push (CI), and the
one-file setup (`stack.yaml`) committed next to their code so "deploying" is
just editing that file.

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

**This is a hard gate, not advice.** When a token is handed to you
mid-conversation, `whoami` is the first command that touches it — before any
other API call. Do not infer the type from the token's shape, and do not
hand-roll GraphQL probes to classify it: the one-liner above answers
everything those would, for free.

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

| Token     | Can list (one level down) | Bound?                         | What "list" grants                                                           |
| --------- | ------------------------- | ------------------------------ | ---------------------------------------------------------------------------- |
| Account   | workspaces                | not bound — full access to all | full access                                                                  |
| Workspace | projects                  | not bound — full access to all | full access                                                                  |
| Project   | environments              | **bound to one**               | **names only** for siblings — content access solely in its bound environment |

The project token is the only **leaf-bound** token: it is really a
_(project, environment)_ token. The API cannot mint an environment-unbound
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

| Capability                                                          | Account token        | Workspace token                                                            | Project token                                                                                   |
| ------------------------------------------------------------------- | -------------------- | -------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------- |
| List/switch workspaces (`-w`)                                       | yes                  | no — scoped to its workspace                                               | no                                                                                              |
| List projects                                                       | yes (all workspaces) | yes (its workspace)                                                        | no                                                                                              |
| Create / delete **projects**                                        | yes                  | yes                                                                        | no                                                                                              |
| Create / delete **environments**                                    | yes                  | yes                                                                        | no                                                                                              |
| Services / variables / volumes / backups / domains / logs / deploys | yes                  | yes                                                                        | yes — within its one project+environment                                                        |
| `apply` / `diff` / `delete -f` (declarative)                        | yes                  | yes                                                                        | yes — its environment only                                                                      |
| Deployment **rollback** (`delete deployment`)                       | yes                  | yes                                                                        | yes                                                                                             |
| Deployment **reactivation** (`update deployment --set-active`)      | yes                  | yes                                                                        | no — workspace-level capability                                                                 |
| Mint project tokens (`token create`)                                | yes (any project)    | yes (any project in its workspace)                                         | yes — **its own project+environment only**                                                      |
| `-w` / `-p` / `-e` flags                                            | honored (selection)  | `-w` must match its workspace or the command **errors**; `-p`/`-e` honored | flags must **match** the token's baked scope (then accepted silently) or the command **errors** |

**Key semantics to remember:**

- A **project token** is pinned to exactly one project **and one environment**
  at mint time. You cannot point it elsewhere: a `-w`/`-p`/`-e` value that
  contradicts the baked scope **fails fast** (`token is scoped to … but -e '…'
was given — refusing to operate …`); a value matching the scope proceeds
  silently. To operate on staging _and_ production you need two tokens.
- A **project token cannot** enumerate anything above its project: no
  `get projects`, no workspace queries, no project/environment lifecycle —
  these fail fast (`cannot … with a project token — it is scoped to a single
project and environment; use an account or workspace token`).
- A **workspace token** behaves like an account token _inside_ its workspace
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

If you run a project-scoped command with a broad account/workspace token,
railctl prints a one-line **least-privilege hint** to stderr nudging you toward
a project token (leaf-bound to one project+environment). It's advisory — silence
it per-run with `RAILCTL_NO_HINTS=1`, or better, switch to the project token.

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

**Step 5 — the reconcile loop. Diff first, always.**

Never apply blind: **always run `diff` before `apply`** — interactively at the
console _and_ in CI. It costs one command and shows exactly what is about to
change (creates, field-level updates, prune deletions), with secrets masked.
An apply whose diff you haven't read is an unreviewed change to production.

```bash
railctl diff  -f stack.yaml            # READ THIS — shows everything that would change
railctl apply -f stack.yaml --await    # create/update + wait for SUCCESS
railctl diff  -f stack.yaml            # clean: live state matches manifest
```

`diff` always exits 0 — read its **output**, not its exit code: a summary of
"0 to create, 0 to update, 0 to delete" means in sync; any non-zero count is
drift. A non-zero _exit_ now means a real error (bad file, auth, API). Keep
this loop closed: after ANY imperative change, reconcile (see _Drift discipline_, §5).

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
--await-completion`, then reconcile per _Drift discipline_, §5 — the tag
   goes into the manifest until `diff` returns to 0).

**Step 8 — monitor & operate** (§6: deployments & logs).

**Step 9 — protect & housekeep.** The moment an environment is worth
anything (production from day one; staging once real data lands), arm the
deletion tripwire:

1. Imperatively: `railctl protect environment <env> -p my-app`. This sets the
   environment-level (shared, serviceless) `DELETE_PROTECTION` variable to
   `true` — clobber-safe, so it preserves every other shared variable. Undo it
   with `railctl unprotect environment <env> -p my-app`. Both work with **any**
   token scoped to the environment — including a **project token** — so you can
   arm protection with the same least-privilege token you use for everything
   else (flag-free under a project token; the scope is baked in).
2. Declaratively: add `deleteProtection: true` at the **top level** of the
   manifest (a sibling of `project`/`environment`/`services`, not inside a
   service). `apply` then ensures the flag is set; `deleteProtection: false`
   clears it. **Omitting the field leaves the live state alone** — a dropped
   line never silently unprotects an environment (same safety principle as
   `customDomains`).
3. Verify: `railctl delete environment <env> --yes` must refuse with
   `environment '…' is delete-protected` — and so must `delete project`.

Protection is not just about the environment as a whole: while it is armed, the
environment's **data and structure** are shielded too — `delete service`,
`delete volume`, `delete backup`, `delete -f`, and `apply --prune` all refuse —
while updates, creates, and config/rollback deletes (domain, variable,
deployment) keep working. See §7 for the exact allow/block matrix.

There is **no bypass flag**; the only way to delete is consciously unprotecting
first (`unprotect environment`, or `deleteProtection: false`) — which is exactly
the two-step friction you want on critical environments. Also: declare
`backupSchedules` on stateful volumes,
rotate tokens periodically (§6 Tokens), and tear down disposable
environments with `delete -f`.

---

## 4. Context resolution: flags → env vars → token scope

Every command resolves context in the order **flag → `RAILCTL_*` env var →
default**. With a project token, project/environment come from the token
itself; a flag or env var naming the _same_ project/environment is accepted
silently, while a _different_ one fails fast (contradiction) — stale
`RAILCTL_PROJECT`/`RAILCTL_ENVIRONMENT` values cannot silently redirect
commands to the token's scope.

| Flag                   | Env var               | Meaning                                       |
| ---------------------- | --------------------- | --------------------------------------------- |
| `--token`              | `RAILWAY_TOKEN`       | API token (required)                          |
| `-w` / `--workspace`   | `RAILCTL_WORKSPACE`   | Workspace (account tokens with >1 workspace)  |
| `-p` / `--project`     | `RAILCTL_PROJECT`     | Project name or ID                            |
| `-e` / `--environment` | `RAILCTL_ENVIRONMENT` | Environment name or ID                        |
| `-s` / `--service`     | `RAILCTL_SERVICE`     | Service name or ID (never baked into a token) |
| `-o` / `--output`      | —                     | `table` (default), `wide`, `json`, `yaml`     |
| `--debug`              | —                     | Dump GraphQL requests/responses to stderr     |

Name arguments resolve **exact match → case-insensitive substring**; ambiguous
matches error listing candidates. Unknown names list what exists:
`project 'foo' not found — available: api, web, …` (capped at 10). Machine
formats stay machine-readable: listings emit `[]` on empty, never prose.

---

## 5. Declarative reference — the manifest

```yaml
# stack.yaml — one file, the whole environment
project: my-app # optional; -p / env var / token scope override
environment: production # optional; same
deleteProtection: true # optional; ensures DELETE_PROTECTION on this env.
# false clears it; OMITTING leaves live state alone
# (a dropped line never silently unprotects).

services:
  - name: api
    image: ghcr.io/owner/app:sha-abc123 # always a prebuilt image reference

    deploy:
      startCommand: "npm start"
      restartPolicy: ON_FAILURE # ON_FAILURE | ALWAYS | NEVER
      maxRetries: 3 # requires restartPolicy
      replicas: 2 # >= 1 if set
      healthcheckPath: /health
      healthcheckTimeout: 300

    networking:
      domain:
        port: 3000 # Railway domain (*.up.railway.app)
      tcpProxy:
        port: 5432 # public TCP proxy to this app port
      customDomains:
        - name: app.example.com # DNS records printed on apply
          port: 3000 # optional; defaults to domain.port

    volume:
      mountPath: /app/data
      backupSchedules: [daily, weekly] # daily/weekly/monthly

    variables:
      PORT: "3000"
      DATABASE_URL: "${{db.DATABASE_URL}}" # Railway-side service reference
      API_KEY: "$env(API_KEY)" # expanded from local env at apply

    registry: # private registries (Pro plan)
      username: "$env(REGISTRY_USER)"
      password: "$env(REGISTRY_PASS)"
```

### Wire services with Railway reference variables, never hardcoded endpoints

When one service needs another's host, port, or URL, **always express it as a
Railway reference variable** — `${{other-service.VAR}}`,
`${{other-service.RAILWAY_PRIVATE_DOMAIN}}` — rather than a literal hostname,
port, or URL. This is a standing directive, not a style preference:

- **They resolve at deploy time**, so a value that only exists after the target
  is provisioned (an internal domain, a generated port) is always correct.
- **They survive renames and re-provisioning** — a hardcoded `api.internal:3000`
  breaks the moment the target changes; `${{api.RAILWAY_PRIVATE_DOMAIN}}`
  follows it.
- **They draw the dependency graph in Railway's UI.** A reference creates a
  visible edge between the two services; a hardcoded string shows _no edge_, so
  the topology silently lies and nobody can see what talks to what.

So prefer:

```yaml
services:
  - name: api
    variables:
      DATABASE_URL: "${{db.DATABASE_URL}}"
      REDIS_HOST: "${{redis.RAILWAY_PRIVATE_DOMAIN}}"
      API_URL: "https://${{api.RAILWAY_PRIVATE_DOMAIN}}"
```

over any form that bakes in `db.internal`, a fixed port, or a full literal URL.
Reserve literals for genuinely external endpoints (third-party APIs) that
Railway does not own.

### A complete worked example (n8n queue-mode, four services)

The schema above, exercised for real — a production-shaped stack (database,
cache, web app with a public domain, horizontally scaled workers) in one
manifest. This exact file is live-verified by railctl's example suite:

```yaml
# n8n queue-mode stack — single declarative manifest (railctl apply/diff).
#
# The declarative equivalent of configs/01..04: postgres + redis + n8n primary
# + n8n workers, one file. Deploy with:
#
#   railctl diff  -f stack.yaml           # show what would change
#   railctl apply -f stack.yaml           # reconcile live state to this manifest
#   railctl delete -f stack.yaml --yes    # teardown: delete these services + declared volumes
#
# Works flag-free under a project token (scope is baked into the token);
# with a workspace/account token pass -p/-e as usual.
#
# Secrets come from the local environment at apply time ($env(...)):
#   N8N_POSTGRES_PASSWORD, N8N_REDIS_PASSWORD, N8N_ENCRYPTION_KEY

services:
  - name: n8n-postgres
    image: ghcr.io/railwayapp-templates/postgres-ssl:16
    deploy:
      startCommand: "/bin/sh -c 'unset PGPORT; docker-entrypoint.sh postgres --port=5432'"
      restartPolicy: ON_FAILURE
      maxRetries: 10
    # No public networking: internal clients reach it at n8n-postgres.railway.internal.
    volume:
      mountPath: /var/lib/postgresql/data
      backupSchedules: [daily]
    variables:
      POSTGRES_USER: "postgres"
      POSTGRES_PASSWORD: "$env(N8N_POSTGRES_PASSWORD)"
      POSTGRES_DB: "n8n"
      PGUSER: "${{n8n-postgres.POSTGRES_USER}}"
      PGPASSWORD: "${{n8n-postgres.POSTGRES_PASSWORD}}"
      PGDATABASE: "${{n8n-postgres.POSTGRES_DB}}"
      PGHOST: "${{RAILWAY_PRIVATE_DOMAIN}}"
      PGPORT: "5432"
      PGDATA: "/var/lib/postgresql/data/pgdata"
      DATABASE_URL: "postgresql://${{n8n-postgres.POSTGRES_USER}}:${{n8n-postgres.POSTGRES_PASSWORD}}@${{RAILWAY_PRIVATE_DOMAIN}}:5432/${{n8n-postgres.POSTGRES_DB}}"

  - name: n8n-redis
    image: redis:7-alpine
    deploy:
      restartPolicy: ON_FAILURE
      maxRetries: 10
    volume:
      mountPath: /data
    variables:
      REDIS_PASSWORD: "$env(N8N_REDIS_PASSWORD)"
      REDISHOST: "${{RAILWAY_PRIVATE_DOMAIN}}"
      REDISPORT: "6379"
      REDISUSER: "default"
      REDIS_URL: "redis://${{n8n-redis.REDISUSER}}:${{n8n-redis.REDIS_PASSWORD}}@${{RAILWAY_PRIVATE_DOMAIN}}:6379"

  - name: n8n-primary
    image: n8nio/n8n:latest
    deploy:
      startCommand: "n8n start"
      restartPolicy: ON_FAILURE
      maxRetries: 10
    networking:
      domain:
        port: 5678
    variables:
      # Database connection (Railway service references resolve at runtime)
      DB_TYPE: "postgresdb"
      DB_POSTGRESDB_DATABASE: "${{n8n-postgres.POSTGRES_DB}}"
      DB_POSTGRESDB_HOST: "${{n8n-postgres.PGHOST}}"
      DB_POSTGRESDB_PASSWORD: "${{n8n-postgres.POSTGRES_PASSWORD}}"
      DB_POSTGRESDB_PORT: "${{n8n-postgres.PGPORT}}"
      DB_POSTGRESDB_USER: "${{n8n-postgres.POSTGRES_USER}}"
      # Redis connection for queue mode
      QUEUE_BULL_REDIS_HOST: "${{n8n-redis.REDISHOST}}"
      QUEUE_BULL_REDIS_PORT: "${{n8n-redis.REDISPORT}}"
      QUEUE_BULL_REDIS_USERNAME: "${{n8n-redis.REDISUSER}}"
      QUEUE_BULL_REDIS_PASSWORD: "${{n8n-redis.REDIS_PASSWORD}}"
      # n8n configuration
      EXECUTIONS_MODE: "queue"
      N8N_ENCRYPTION_KEY: "$env(N8N_ENCRYPTION_KEY)"
      N8N_EDITOR_BASE_URL: "https://${{RAILWAY_PUBLIC_DOMAIN}}"
      WEBHOOK_URL: "https://${{RAILWAY_PUBLIC_DOMAIN}}"
      PORT: "5678"

  - name: n8n-worker
    image: n8nio/n8n:latest
    deploy:
      startCommand: "n8n worker"
      restartPolicy: ON_FAILURE
      maxRetries: 10
      replicas: 2
    variables:
      # Database connection
      DB_TYPE: "postgresdb"
      DB_POSTGRESDB_DATABASE: "${{n8n-postgres.POSTGRES_DB}}"
      DB_POSTGRESDB_HOST: "${{n8n-postgres.PGHOST}}"
      DB_POSTGRESDB_PASSWORD: "${{n8n-postgres.POSTGRES_PASSWORD}}"
      DB_POSTGRESDB_PORT: "${{n8n-postgres.PGPORT}}"
      DB_POSTGRESDB_USER: "${{n8n-postgres.POSTGRES_USER}}"
      # Redis connection for queue mode
      QUEUE_BULL_REDIS_HOST: "${{n8n-redis.REDISHOST}}"
      QUEUE_BULL_REDIS_PORT: "${{n8n-redis.REDISPORT}}"
      QUEUE_BULL_REDIS_USERNAME: "${{n8n-redis.REDISUSER}}"
      QUEUE_BULL_REDIS_PASSWORD: "${{n8n-redis.REDIS_PASSWORD}}"
      # Worker configuration
      EXECUTIONS_MODE: "queue"
      N8N_ENCRYPTION_KEY: "${{n8n-primary.N8N_ENCRYPTION_KEY}}"
      WEBHOOK_URL: "https://${{n8n-primary.RAILWAY_PUBLIC_DOMAIN}}"
      PORT: "5678"
```

Deploy it with a project token and three secrets in the environment —
nothing else:

```bash
export RAILWAY_TOKEN=<project token>   # scope baked in; no -p/-e anywhere
export N8N_POSTGRES_PASSWORD=… N8N_REDIS_PASSWORD=… N8N_ENCRYPTION_KEY=…
railctl diff  -f stack.yaml
railctl apply -f stack.yaml --await
```

### A second worked example (Temporal, incl. a private-registry worker)

The same schema deploying a Temporal cluster — notable extras over the n8n
example: the **worker image comes from `$env(...)`** (your CI publishes it and
the tag is injected per release), and its **private-registry pull credentials**
ride in via the `registry` block, never in the file:

```yaml
# Temporal stack — single declarative manifest (railctl apply/diff/delete -f).
#
# The declarative equivalent of configs/01..04: postgres + temporal server
# (auto-setup) + web UI + a worker template, one file. Deploy with:
#
#   railctl diff  -f stack.yaml     # ALWAYS diff first — read what would change
#   railctl apply -f stack.yaml --await
#   railctl delete -f stack.yaml --yes   # teardown: declared services + volumes
#
# Works flag-free under a project token (scope is baked into the token);
# with a workspace/account token pass -p/-e as usual.
#
# Secrets/inputs come from the local environment at apply time ($env(...)):
#   TEMPORAL_POSTGRES_PASSWORD              — database password
#   TEMPORAL_WORKER_IMAGE                   — your worker image (Temporal SDK build)
#   TEMPORAL_TASK_QUEUE                     — the worker's task queue
#   REGISTRY_USERNAME / REGISTRY_PASSWORD   — read-only pull credentials if the
#                                             worker image is in a private registry

services:
  - name: temporal-postgres
    image: postgres:15
    deploy:
      restartPolicy: ON_FAILURE
      maxRetries: 5
    # No public networking: internal clients reach it at temporal-postgres.railway.internal.
    volume:
      mountPath: /var/lib/postgresql/data
      backupSchedules: [daily]
    variables:
      POSTGRES_USER: "temporal"
      POSTGRES_PASSWORD: "$env(TEMPORAL_POSTGRES_PASSWORD)"
      POSTGRES_DB: "temporal"
      PGDATA: "/var/lib/postgresql/data/pgdata"

  - name: temporal-server
    image: temporalio/auto-setup:1.29.5
    deploy:
      restartPolicy: ON_FAILURE
      maxRetries: 5
    # Internal-only by default: the worker below reaches the frontend at
    # temporal-server.railway.internal:7233. Temporal's frontend has NO auth
    # out of the box — only add a public `tcpProxy` here if external clients
    # must connect, and put authentication (mTLS) in front of it first.
    variables:
      DB: "postgres12"
      DB_PORT: "5432"
      POSTGRES_USER: "${{temporal-postgres.POSTGRES_USER}}"
      POSTGRES_PWD: "${{temporal-postgres.POSTGRES_PASSWORD}}"
      POSTGRES_SEEDS: "${{temporal-postgres.RAILWAY_PRIVATE_DOMAIN}}"
      DYNAMIC_CONFIG_FILE_PATH: "config/dynamicconfig/docker.yaml"

  - name: temporal-ui
    image: temporalio/ui:2.48.1
    deploy:
      restartPolicy: ON_FAILURE
      maxRetries: 3
    networking:
      domain:
        port: 8080
    variables:
      TEMPORAL_ADDRESS: "${{temporal-server.RAILWAY_PRIVATE_DOMAIN}}:7233"
      TEMPORAL_CORS_ORIGINS: "http://localhost:3000"
      PORT: "8080"

  # Template for your own Temporal worker — replace the image with your build
  # from the Temporal SDK, published by YOUR CI (railctl treats Railway as a
  # compute provider: images are always prebuilt references).
  - name: temporal-worker
    image: "$env(TEMPORAL_WORKER_IMAGE)"
    deploy:
      restartPolicy: ON_FAILURE
      maxRetries: 10
      replicas: 2
    variables:
      TEMPORAL_ADDRESS: "${{temporal-server.RAILWAY_PRIVATE_DOMAIN}}:7233"
      TEMPORAL_NAMESPACE: "default"
      TEMPORAL_TASK_QUEUE: "$env(TEMPORAL_TASK_QUEUE)"
    registry:
      username: "$env(REGISTRY_USERNAME)"
      password: "$env(REGISTRY_PASSWORD)"
```

### Networking: internal by default, public only on purpose

**A `tcpProxy` or `domain` block is PUBLIC internet exposure — not "networking".**
Getting this wrong is the most dangerous default in the whole tool.

- **Service-to-service traffic needs neither.** Every service is reachable
  from its siblings at `<service-name>.railway.internal` over Railway's
  private mesh — free, and never internet-visible. A database, cache, or
  internal API talks to its consumers over `.railway.internal` with **no
  networking block at all**.
- **`networking.domain.port`** publishes an HTTPS `*.up.railway.app` URL —
  use it for the one or two services that are genuinely a public web surface
  (the UI/API front door).
- **`networking.tcpProxy.port`** opens a public TCP endpoint (`host:port`) —
  use it only when something _outside_ Railway must connect (e.g. you need to
  reach a database from your laptop).

**Hard rule: never put a service with no authentication on a public proxy.**
An unauthenticated Postgres, Redis, etcd, or internal gRPC service on a
`tcpProxy` is an open door to the internet. If a datastore only serves other
services in the stack, it gets **no** `networking` block — full stop. When in
doubt, start internal and add exposure deliberately, one service at a time.

**You do not need to expose a port just to reach it yourself.** The old reason
people opened a `tcpProxy` — "but I need to connect from my laptop to run a
migration / open a psql shell / poke the admin API" — is gone: `railctl
port-forward` tunnels straight into a **private** service over SSH, and
`railctl exec` opens a shell inside it, both with **no public surface** (see the
monitoring commands in §6). So a public proxy on an unauthenticated datastore is
now not just dangerous, it's **unnecessary**: forward to it for admin/debug and
leave it internal. Reserve `tcpProxy`/`domain` for traffic that genuinely
originates _outside_ Railway (real end users, third-party webhooks).

**Un-exposing is declarative too:** removing a `tcpProxy`/`domain` block and
re-applying closes the port (`diff` shows `- networking.tcpProxy.port: …`,
`apply` removes it). So if you inherit an over-exposed stack, delete the
blocks and apply. (User-owned `customDomains` are the one exception — they are
removed with `delete domain`, never silently on apply.)

### The three verbs

| Command                                                                                    | Does                                                                                           | Exit                                      |
| ------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------- | ----------------------------------------- |
| `railctl diff -f <file-or-dir> [--prune]`                                                  | show create/update/delete deltas, secrets masked                                               | always 0; read the summary line for drift |
| `railctl apply -f <file-or-dir> [--await] [--await-timeout N] [--dry-run] [--prune --yes]` | reconcile live state to the manifest                                                           | 0 = applied                               |
| `railctl delete -f <file-or-dir> [--yes]`                                                  | delete exactly the **declared** services (reverse manifest order), then their declared volumes | 0 = done / cancelled                      |

### Semantics that matter

- **Declared state is authoritative for managed fields.** A service with a
  declared `volume.mountPath` is a _managed volume_: omitting
  `backupSchedules` (or `[]`) **clears live schedules** on the next apply —
  with an explicit warning naming what was removed. A service with no
  `volume:` block is left untouched.
- **`domain.port` and `tcpProxy.port` reconcile removal**: omitting the block
  and re-applying closes the port (railctl-generated public surface is fully
  declarative — this is how you un-expose a service). **`customDomains` are the
  exception** — user-owned, so they are never removed on absence; use
  `railctl delete domain` (avoids an accidental outage + DNS rework).
- **`--prune`** deletes live services not declared in the manifest — the only
  apply-path deletion; prompts unless `--yes`.
- **`delete -f`** touches only what the manifest declares: services in
  reverse order, then their declared volumes (a deleted service orphans its
  volume). It never deletes environments or projects, skips absent resources
  with a note, needs no `$env()` secrets, and prints an itemized plan before
  the `[y/N]` prompt.
- Volumes **cannot change mountPath in place**; a deleted service **orphans**
  its volume.

### Drift discipline — reconcile every imperative change

The manifest is only the source of truth while it matches reality. **Any
change made imperatively** (`update service --image`, `set variable`,
`create domain`, `update volume`, …) **creates drift and must be reconciled
into the manifest immediately**:

```bash
railctl diff -f stack.yaml       # shows exactly what you changed out-of-band
$EDITOR stack.yaml               # backport the change (or apply to revert it)
railctl diff -f stack.yaml       # clean diff — truth restored
```

`diff` makes this trivial: it shows the delta field-by-field, so backporting
is a copy of what it prints. The alternative direction also works — if the
imperative change was a mistake, `apply` reverts live state to the manifest.

**The loop is `diff → review → apply` EVERY time, not just the first.**
During iterative troubleshooting it is tempting to edit the manifest and jump
straight to `apply` — don't: each skipped diff is an unreviewed change, and
skipping breeds the worse habit of _claiming_ sync status from memory. Never
state "no drift" / "state matches the manifest" to the user without a fresh
`diff` exit-0 **run after your last apply** to back it.

**Known blind spot:** `diff`/`apply` reconcile only what railctl models
(services, deploy config, variables, volumes + backup schedules, domains,
TCP proxies). Configuration made in the **Railway console** that railctl does
not support is invisible to `diff` and will be neither detected nor reverted
— it silently coexists. Keep unsupported console-side settings to a minimum,
and document them next to the manifest when unavoidable.

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
railctl protect environment production -p my-app     # sets DELETE_PROTECTION=true (shared var)
railctl unprotect environment production -p my-app   # clears it (DELETE_PROTECTION=false)
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
`DELETE_PROTECTION`). There is no general shared-variable write command, but the
one shared variable that matters operationally — `DELETE_PROTECTION` — is
managed first-class via `railctl protect`/`unprotect environment` (imperative)
or the top-level `deleteProtection` manifest field (declarative). Other shared
variables are still set in the Railway dashboard.

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
railctl get replicas -s api                   # running replicas (INSTANCE ID + STATUS) for --deployment-instance
railctl create deployment -s api [--await-completion]    # explicit redeploy
railctl delete deployment <id> -s api --yes   # rollback if latest; status → REMOVED
railctl update deployment <id> --set-active   # reactivation — workspace token required
railctl logs api [--tail N(≤500)] [-f] [--deployment <id>]   # logs <service> — one arg
```

If any command errors unexpectedly on syntax, check `railctl <cmd> --help`
before retrying variations — the help text is authoritative for the installed
version.
Deployment statuses: `INITIALIZING → BUILDING → DEPLOYING → SUCCESS`, or
`FAILED / CRASHED / REMOVED / SKIPPED`. Poll
`get deployments -o json --limit 1` for the latest status in scripts.
`create service` does **not** reliably deploy by itself — trigger explicitly
with `create deployment` when you need a deterministic first deployment.

**When logs aren't enough, go inside.** `logs` is the first-line monitoring
tool; when you need to _interact_ with a running service — open a shell, inspect
files, run a one-off admin/migration command, or point a local client at an
internal port — reach for **`railctl exec`** and **`railctl port-forward`**
(next). They work on **private** services with no public exposure, over
Railway's SSH relay. One-time prerequisite so it isn't a surprise later:
register your SSH key **once** at
[railway.com/account/ssh-keys](https://railway.com/account/ssh-keys) (railctl
never manages keys); after that, exec/port-forward work with **any** token.

### Exec — shell into a service container (SSH)

```bash
railctl exec api -p my-project -e production                  # interactive shell (kubectl-exec style)
railctl exec api -p my-project -e production -- ls -la /data  # one-off command; exit code propagated
railctl exec api ... -i ~/.ssh/id_ed25519 -- env             # use a specific private key
railctl exec api ... --deployment-instance <id> -- <cmd>     # target a specific replica (list ids: railctl get replicas -s api)
```

The service is a **positional argument** (like `logs <service>`, not `-s`);
everything after `--` is the remote command, passed verbatim (omit it for an
interactive shell). railctl shells out to your **local `ssh` binary** and dials
Railway's global relay (`ssh.railway.com`), which brokers the session into the
container docker-exec style — **the container needs NO sshd of its own**, but
you DO need a local `ssh` binary and an SSH key. **railctl does not manage SSH
keys** — register your key **once** at
[railway.com/account/ssh-keys](https://railway.com/account/ssh-keys); ssh then
authenticates with it (agent / `~/.ssh` defaults, or the `-i` override).
**Token scope: exec works with ANY token — account, workspace, OR project.** The
token is used only to resolve the service instance; authentication is by your
SSH key, not the token. If ssh fails with a publickey/permission error, register
your key at the URL above and retry. See the design in
`docs/designs/2026-07-09-railctl-exec-port-forward.md`.

### Port-forward — reach a service's ports over SSH (incl. private services)

```bash
railctl port-forward db 5432 -p my-project -e production            # localhost:5432 -> db's own 127.0.0.1:5432
railctl port-forward kube-apiserver 6443 -p my-project -e production # reach a PRIVATE service directly (no public exposure)
railctl port-forward db 6543:5432 -p my-project -e production       # map a different local port
railctl port-forward db 5432 6379 -p my-project -e production       # multiple ports, ONE ssh connection
railctl port-forward db 5432 --address 0.0.0.0 -i ~/.ssh/id_ed25519 # share on the LAN + specific key
```

kubectl-`port-forward`-style local forwarding over Railway's SSH relay. The
service is a **positional argument**; every bare positional after it is a port
spec (multiple `-L` forwards ride **one** ssh connection). It runs in the
**foreground** and streams until **Ctrl-C**. Same transport and token model as
`exec` (local `ssh` binary, your pre-registered SSH key, no sshd in the
container; works with any token).

**Reaching a private service — you forward directly INTO it.** This is kubectl's
actual model (`kubectl port-forward pod` targets the pod itself, not a bastion).
Name the private service — it works with no public domain/proxy:

```bash
railctl port-forward kube-apiserver 6443    # then: kubectl --server https://127.0.0.1:6443 …
```

There is **no jump/bastion form** — verified live, Railway's relay forwards
only to the _target container's own loopback_, not to other hosts through it.

**Port-spec grammar:**

| Form                              | Emits                              | Meaning                    |
| --------------------------------- | ---------------------------------- | -------------------------- |
| `REMOTE` (e.g. `8080`)            | `-L 127.0.0.1:8080:127.0.0.1:8080` | local == remote            |
| `LOCAL:REMOTE` (e.g. `6543:5432`) | `-L 127.0.0.1:6543:127.0.0.1:5432` | map a different local port |

The remote side is always the service's own loopback (`127.0.0.1`); a bare
number can never smuggle `localhost` (which resolves to an unreachable mesh
address). A three-field `LOCAL:HOST:REMOTE` spec is rejected.

> ⚠️ **The target must listen on IPv4 (verified live).** The forward lands on
> the service's `127.0.0.1`, so the service must bind IPv4 loopback or
> `0.0.0.0`. A service that binds **IPv6-only** (`[::]`) is not reachable this
> way — the `-L` to `127.0.0.1` finds nothing (an "empty reply"). Most servers
> (Postgres, Redis, kube-apiserver with `--bind-address 0.0.0.0`) bind IPv4;
> ensure yours does if you need to forward to it.

**Token scope: port-forward works with ANY token — account, workspace, OR
project** (same model as `exec`: the token only resolves the instance;
authentication is by your **pre-registered SSH key**, which you register once at
[railway.com/account/ssh-keys](https://railway.com/account/ssh-keys) — railctl
does not manage keys). Flags: `-i/--identity-file`, `--deployment-instance
<id>`, `--address` (local bind, default `127.0.0.1`; `0.0.0.0` to share on the
LAN). See the design in
`docs/designs/2026-07-09-railctl-exec-port-forward.md`.

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

- **`DELETE_PROTECTION` — what a protected environment actually means.** An
  environment whose shared (environment-level) variable `DELETE_PROTECTION` is
  truthy (`true`/`1`/`yes`/`on`, case-insensitive) is a **read-mostly, no-delete
  zone**: it protects **data and structure** while leaving day-to-day operation
  fully open. Concretely, in a protected environment:
  - **Blocked** (its data + structure are shielded): deleting the **environment**
    itself or its **project**, and deleting a **service**, **volume**, or
    **backup** — including `delete -f` (teardown) and `apply --prune` (which
    deletes services). railctl refuses with **no bypass flag** (`--yes` skips
    prompts, never protection); the message names the resource and points at
    `unprotect environment <env>`.
  - **Allowed** (configuration + operational, nothing destroyed irreversibly):
    every **update** and **create**, plus deleting a **domain**, a **variable**,
    or a **deployment** (a rollback). You keep shipping, tweaking config, and
    rolling back at full speed; you just can't tear down the data or the
    services holding it.
  - Unreadable protection state → the delete is refused (fail-closed).
    Arm it on every environment you care about — imperatively with
    `railctl protect environment <env>` (undo: `unprotect environment <env>`), or
    declaratively with the top-level `deleteProtection: true` manifest field
    (`false` clears it; **omitting it leaves the live state alone** — a dropped
    line never silently unprotects). `apply --prune` checks **live** protection,
    so unprotect first (or in a prior apply) to prune a protected environment.
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

| Symptom                                                    | Cause / fix                                                                                                                                                                                                                                   |
| ---------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `token is not authorized`                                  | Expired/revoked token, or all three detection probes failed. `railctl whoami`, re-mint.                                                                                                                                                       |
| `token is scoped to … but -p/-e/-w '…' was given`          | Contradiction fail-fast: flags/env vars disagree with the token's baked scope. Fix the stale `RAILCTL_*` value or use the right token.                                                                                                        |
| `cannot … with a project token`                            | Workspace-scope operation (project/env lifecycle, `get projects`, deployment reactivation). Use a workspace/account token.                                                                                                                    |
| `… not found — available: a, b, c`                         | Typo — the listed candidates are what exists.                                                                                                                                                                                                 |
| `environment '…' is delete-protected`                      | `DELETE_PROTECTION` is set — run `railctl unprotect environment <env>` (or set `deleteProtection: false` and `apply`) to allow deletion.                                                                                                      |
| Token works in the dashboard but railctl says unauthorized | Probably project-scoped and the other tool sends `Authorization: Bearer` only; railctl handles the `Project-Access-Token` header automatically — check for typos/whitespace.                                                                  |
| `diff` "fails" in CI                                       | `diff` always exits 0 on drift — a non-zero exit now means a real error (bad file, auth, API); read the message. To gate CI on drift, parse the summary line (`0 to create, 0 to update, 0 to delete`), not the exit code.                    |
| Container exits instantly / `startCommand` seems ignored   | The image likely has a fixed **ENTRYPOINT**: Railway appends `startCommand` as CMD args and does **not** override the entrypoint, which can silently swallow your command. Use an image with a shell entrypoint or build a thin custom image. |
| `logs` prints nothing, no error                            | Logs default to the **latest successful** deployment — if none succeeded yet there is nothing to show. Use `--deployment <id>` (ids from `get deployments`) to read a failed deployment's logs.                                               |
| Volume/backup op right after creation says not found       | Propagation lag; railctl retries with backoff — re-run if it still misses.                                                                                                                                                                    |
| Backup restore "did nothing"                               | Restore is staged — **deploy the service** to finalize.                                                                                                                                                                                       |
| Apply cleared backup schedules unexpectedly                | The volume is managed and the manifest omitted `backupSchedules` — declared state is authoritative; re-declare them.                                                                                                                          |
| Custom domain stuck pending                                | DNS records not added/propagated — `get domains -s <svc>` shows verification status.                                                                                                                                                          |
| `--debug`                                                  | Global flag: dumps GraphQL traffic to stderr.                                                                                                                                                                                                 |

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

**CI deploy gate** (project token in CI secrets; no flags anywhere). Diff
first even in CI — its output in the job log is the reviewable change record:

```bash
export RAILWAY_TOKEN="$RAILWAY_PROJECT_TOKEN"
railctl diff -f stack.yaml                 # prints the pending changes to the log
railctl apply -f stack.yaml --await        # idempotent — a no-op when already in sync
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
