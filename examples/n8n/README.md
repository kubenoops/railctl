# n8n Queue-Mode Deployment

Deploy [n8n](https://n8n.io) in **queue mode** on [Railway](https://railway.app) using `railctl`.

Queue mode separates the web UI from workflow execution, giving you independent
scaling of workers and zero-downtime webhook processing.

## Declarative alternative (one manifest)

The four imperative configs collapse into a single declarative manifest,
[`stack.yaml`](stack.yaml), reconciled by `railctl apply`:

```bash
source .envrc                      # token + secrets (see above)
railctl diff  -f stack.yaml        # exit != 0 while anything would change
railctl apply -f stack.yaml --await
railctl diff  -f stack.yaml        # now empty — state matches the manifest
railctl delete -f stack.yaml --yes # teardown — deletes the declared services + postgres volume
```

With a project token no `-p`/`-e` flags are needed — the token carries its
scope. The manifest also manages the postgres volume's daily backup schedule
declaratively.

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│  Railway Project                                        │
│                                                         │
│  ┌──────────────┐       ┌──────────────┐                │
│  │  PostgreSQL   │◄──────│  n8n Primary │──► public URL  │
│  │  (persistent) │       │  (web UI)    │                │
│  └──────┬───────┘       └──────┬───────┘                │
│         │                      │                        │
│         │   ┌──────────────┐   │                        │
│         │   │    Redis     │◄──┘                        │
│         │   │  (job queue) │                            │
│         │   └──────┬───────┘                            │
│         │          │                                    │
│         │   ┌──────┴───────┐                            │
│         └──►│  n8n Worker  │  (×2 replicas)             │
│             │  (execution) │                            │
│             └──────────────┘                            │
└─────────────────────────────────────────────────────────┘
```

| Service | Image | Purpose |
|---------|-------|---------|
| `n8n-postgres` | `ghcr.io/railwayapp-templates/postgres-ssl:16` | Workflow & credential storage |
| `n8n-redis` | `redis:7-alpine` | Bull job queue for async execution |
| `n8n-primary` | `n8nio/n8n:latest` | Web editor, API, webhook receiver |
| `n8n-worker` | `n8nio/n8n:latest` | Workflow execution (2 replicas) |

## Prerequisites

- **railctl** — [installation instructions](../../README.md#installation)
- **yq** — YAML processor ([install](https://github.com/mikefarah/yq#install))
- **Railway account** — [sign up](https://railway.app) and create an API token

## Quick Start

### 1. Configure secrets

```bash
cp .envrc.example .envrc
```

Edit `.envrc` and fill in your values:

```bash
export RAILWAY_TOKEN="your-railway-api-token"
export RAILCTL_PROJECT="my-n8n"
export RAILCTL_ENVIRONMENT="production"

# Generate secrets:
#   openssl rand -hex 16
export N8N_POSTGRES_PASSWORD="<generated>"
export N8N_REDIS_PASSWORD="<generated>"
export N8N_ENCRYPTION_KEY="<generated>"
```

Load the environment:

```bash
source .envrc
# or, if using direnv:
direnv allow
```

### 2. Deploy

```bash
chmod +x deploy.sh
./deploy.sh
```

The script will:
1. Create the Railway project and environment (if they don't exist)
2. Deploy PostgreSQL and Redis with persistent volumes
3. Deploy n8n Primary with a public domain
4. Deploy n8n Workers (2 replicas)
5. Wait for all deployments to reach `SUCCESS`

### 3. Verify

```bash
# List services
railctl get services -p my-n8n -e production

# Check logs
railctl logs n8n-primary -p my-n8n -e production

# Describe a service
railctl describe service n8n-primary -p my-n8n -e production
```

### 4. Clean up

```bash
chmod +x cleanup.sh
./cleanup.sh
```

## Configuration Reference

### Environment Variables (`.envrc`)

| Variable | Required | Description |
|----------|----------|-------------|
| `RAILWAY_TOKEN` | ✅ | Railway API token (any type; a **project token** is recommended — least privilege) |
| `RAILCTL_PROJECT` | with workspace/account tokens | Project name — **optional with a project token** (scope derived via `railctl whoami`) |
| `RAILCTL_ENVIRONMENT` | with workspace/account tokens | Environment name — **optional with a project token** (e.g., `production`) |
| `N8N_POSTGRES_PASSWORD` | ✅ | PostgreSQL password |
| `N8N_REDIS_PASSWORD` | ✅ | Redis password |
| `N8N_ENCRYPTION_KEY` | ✅ | n8n encryption key for credentials |

### Config Files

All service definitions live in `configs/` and are deployed in alphabetical order:

| File | Service | Notes |
|------|---------|-------|
| `01-n8n-postgres.yaml` | PostgreSQL 16 | Volume, TCP proxy |
| `02-n8n-redis.yaml` | Redis 7 | Volume |
| `03-n8n-primary.yaml` | n8n web UI | Public domain on port 5678 |
| `04-n8n-worker.yaml` | n8n workers | 2 replicas, no public endpoint |

### Variable Syntax

Config files support two variable syntaxes:

- **`$env(VAR)`** — Expanded at deploy time from your local environment (secrets)
- **`${{service.VAR}}`** — Railway service references, resolved at runtime

Example:
```yaml
variables:
  POSTGRES_PASSWORD: "$env(N8N_POSTGRES_PASSWORD)"     # your secret
  DB_HOST: "${{n8n-postgres.RAILWAY_PRIVATE_DOMAIN}}"   # resolved by Railway
```

## How `deploy.sh` Works

The deploy script delegates to [`../shared/deploy.sh`](../shared/deploy.sh),
which is a generic, reusable deployment engine:

1. **Dependency check** — verifies `railctl`, `yq`, and env vars are available
2. **Project/environment setup** — creates them if missing
3. **Config iteration** — reads each `configs/*.yaml` in sort order
4. **Idempotent service management** — creates or updates each service
5. **Volume provisioning** — attaches persistent storage where configured
6. **Variable injection** — expands `$env()` refs, preserves `${{}}` refs
7. **Deployment await** — polls Railway until all services reach terminal status

### Deploy a single service

```bash
./deploy.sh --config 01-n8n-postgres.yaml
```

### Skip waiting

```bash
./deploy.sh --skip-wait
```

## Customization

### Scale workers

Edit `configs/04-n8n-worker.yaml`:

```yaml
deploy:
  numReplicas: 4  # scale to 4 workers
```

### Pin n8n version

Replace `latest` with a specific tag in both primary and worker configs:

```yaml
service:
  image: n8nio/n8n:1.70.3
```

### Use a private n8n image

Add registry credentials to the config:

```yaml
service:
  image: ghcr.io/your-org/custom-n8n:latest
registry:
  username: "$env(GH_USERNAME)"
  password: "$env(GH_PASSWORD)"
```

Then add the credentials to `.envrc`.

## Directory Structure

```
examples/n8n/
├── configs/
│   ├── 01-n8n-postgres.yaml   # PostgreSQL database
│   ├── 02-n8n-redis.yaml      # Redis job queue
│   ├── 03-n8n-primary.yaml    # n8n web UI & API
│   └── 04-n8n-worker.yaml     # n8n worker (2 replicas)
├── .envrc.example              # Template for secrets
├── deploy.sh                   # Deployment entrypoint
├── cleanup.sh                  # Teardown entrypoint
└── README.md                   # This file
```

## Features Demonstrated

- ✅ Config-driven declarative deployment
- ✅ Persistent volumes (PostgreSQL, Redis)
- ✅ Railway service-to-service references
- ✅ Secret injection via `$env()` syntax
- ✅ Public domain generation
- ✅ TCP proxy for database access
- ✅ Multi-replica workers
- ✅ Deployment status polling
- ✅ Idempotent create-or-update
