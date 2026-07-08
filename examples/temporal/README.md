# Temporal Workflow Engine Deployment

Deploy [Temporal](https://temporal.io) on [Railway](https://railway.app) using `railctl`.

Temporal is a durable execution platform for running reliable, long-running
workflows. This example deploys the full Temporal stack — server, UI, and a
template for your own worker.

## Declarative alternative (one manifest)

The four configs collapse into a single declarative manifest,
[`stack.yaml`](stack.yaml), reconciled by `railctl apply`:

```bash
source .envrc                      # token + secrets (see above)
railctl diff  -f stack.yaml        # ALWAYS diff first — exit != 0 while drift exists
railctl apply -f stack.yaml --await
railctl delete -f stack.yaml --yes # declarative teardown
```

With a project token no `-p`/`-e` flags are needed — the token carries its
scope. The manifest also manages the postgres volume's daily backup schedule
declaratively.

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│  Railway Project                                             │
│                                                              │
│  ┌──────────────┐       ┌──────────────────┐                 │
│  │  PostgreSQL   │◄──────│  Temporal Server  │  (gRPC :7233)  │
│  │  (persistent) │       │  (auto-setup)     │                │
│  └──────────────┘       └────────┬─────────┘                 │
│                                  │                           │
│                    ┌─────────────┼─────────────┐             │
│                    │             │             │              │
│             ┌──────┴──────┐  ┌──┴───────┐                    │
│             │ Temporal UI  │  │  Worker   │  (×2 replicas)    │
│             │ (web :8080)  │  │ (your app)│                   │
│             └─────────────┘  └──────────┘                    │
│                    ▲                                         │
│                    │                                         │
│              public URL                                      │
└──────────────────────────────────────────────────────────────┘
```

| Service | Image | Purpose |
|---------|-------|---------|
| `temporal-postgres` | `postgres:15` | Temporal persistence (history, visibility) |
| `temporal-server` | `temporalio/auto-setup:1.29.5` | Temporal gRPC server with auto-schema setup |
| `temporal-ui` | `temporalio/ui:2.48.1` | Web dashboard for workflows |
| `temporal-worker` | *Your image* | Your application worker (template) |

## Prerequisites

- **railctl** — [installation instructions](../../README.md#installation)
- **yq** — YAML processor ([install](https://github.com/mikefarah/yq#install))
- **Railway account** — [sign up](https://railway.app) and create an API token

## Quick Start

### 1. Configure secrets

```bash
cp .envrc.example .envrc
```

Edit `.envrc`:

```bash
export RAILWAY_TOKEN="your-railway-api-token"
export RAILCTL_PROJECT="my-temporal"
export RAILCTL_ENVIRONMENT="production"

# Generate: openssl rand -hex 16
export TEMPORAL_POSTGRES_PASSWORD="<generated>"

# Only needed if deploying a custom worker (04-temporal-worker.yaml)
export TEMPORAL_WORKER_IMAGE="your-registry/your-worker:latest"
export REGISTRY_USERNAME="your-username"
export REGISTRY_PASSWORD="your-token"
export TEMPORAL_TASK_QUEUE="my-task-queue"
```

Load the environment:

```bash
source .envrc
```

### 2. Deploy the infrastructure (without worker)

If you don't have a worker image yet, deploy just the core services:

```bash
chmod +x deploy.sh
./deploy.sh --config 01-temporal-postgres.yaml \
            --config 02-temporal-server.yaml \
            --config 03-temporal-ui.yaml
```

### 3. Deploy everything (including worker)

Once you have a worker image:

```bash
./deploy.sh
```

The script will:
1. Create the Railway project and environment (if missing)
2. Deploy PostgreSQL with a persistent volume
3. Deploy Temporal Server (auto-creates the schema)
4. Deploy Temporal UI with a public domain
5. Deploy your worker (2 replicas)
6. Wait for all deployments to reach `SUCCESS`

### 4. Verify

```bash
# List services
railctl get services -p my-temporal -e production

# Check Temporal Server logs
railctl logs service temporal-server -p my-temporal -e production

# Check UI is accessible
railctl describe service temporal-ui -p my-temporal -e production
```

### 5. Clean up

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
| `RAILCTL_ENVIRONMENT` | with workspace/account tokens | Environment name — **optional with a project token** |
| `TEMPORAL_POSTGRES_PASSWORD` | ✅ | PostgreSQL password |
| `TEMPORAL_WORKER_IMAGE` | Worker only | Your worker container image |
| `REGISTRY_USERNAME` | Worker only | Container registry username |
| `REGISTRY_PASSWORD` | Worker only | Container registry password/token |
| `TEMPORAL_TASK_QUEUE` | Worker only | Temporal task queue name |

### Config Files

| File | Service | Notes |
|------|---------|-------|
| `01-temporal-postgres.yaml` | PostgreSQL 15 | Volume, TCP proxy |
| `02-temporal-server.yaml` | Temporal Server | gRPC on 7233, auto-schema |
| `03-temporal-ui.yaml` | Temporal UI | Public domain on port 8080 |
| `04-temporal-worker.yaml` | Your worker | 2 replicas, private registry |

### Variable Syntax

Config files support two syntaxes:

- **`$env(VAR)`** — Expanded at deploy time from your local environment
- **`${{service.VAR}}`** — Railway service references, resolved at runtime

Example:
```yaml
variables:
  POSTGRES_PWD: "$env(TEMPORAL_POSTGRES_PASSWORD)"             # your secret
  POSTGRES_SEEDS: "${{temporal-postgres.RAILWAY_PRIVATE_DOMAIN}}"  # Railway resolves
```

## How `deploy.sh` Works

The deploy script delegates to [`../shared/deploy.sh`](../shared/deploy.sh),
a generic deployment engine shared across examples:

1. **Dependency check** — `railctl`, `yq`, and env vars
2. **Project/environment setup** — creates if missing
3. **Config iteration** — processes `configs/*.yaml` in sort order
4. **Idempotent service management** — creates or updates
5. **Volume provisioning** — persistent storage where configured
6. **Variable injection** — expands `$env()`, preserves `${{}}`
7. **Deployment await** — polls until all services are terminal

### Deploy individual services

```bash
# Just the database
./deploy.sh --config 01-temporal-postgres.yaml

# Server + UI
./deploy.sh --config 02-temporal-server.yaml --config 03-temporal-ui.yaml
```

## Building a Temporal Worker

The `04-temporal-worker.yaml` config is a **template**. You need to provide
your own worker image built with a [Temporal SDK](https://docs.temporal.io/develop).

Minimal Go worker example:

```go
package main

import (
    "log"
    "os"

    "go.temporal.io/sdk/client"
    "go.temporal.io/sdk/worker"
)

func main() {
    c, err := client.Dial(client.Options{
        HostPort: os.Getenv("TEMPORAL_ADDRESS"),
    })
    if err != nil {
        log.Fatal(err)
    }
    defer c.Close()

    w := worker.New(c, os.Getenv("TEMPORAL_TASK_QUEUE"), worker.Options{})
    // w.RegisterWorkflow(YourWorkflow)
    // w.RegisterActivity(YourActivity)
    if err := w.Run(worker.InterruptCh()); err != nil {
        log.Fatal(err)
    }
}
```

Build, push to your registry, and set `TEMPORAL_WORKER_IMAGE` in `.envrc`.

## Customization

### Pin Temporal versions

Update image tags in the config files:

```yaml
# 02-temporal-server.yaml
service:
  image: temporalio/auto-setup:1.25.2

# 03-temporal-ui.yaml
service:
  image: temporalio/ui:2.30.3
```

### Scale workers

Edit `04-temporal-worker.yaml`:

```yaml
deploy:
  numReplicas: 4
```

### Add Temporal namespaces

The `auto-setup` image creates the `default` namespace automatically.
Additional namespaces can be created via the Temporal UI or `tctl`.

## Directory Structure

```
examples/temporal/
├── configs/
│   ├── 01-temporal-postgres.yaml   # PostgreSQL database
│   ├── 02-temporal-server.yaml     # Temporal server (auto-setup)
│   ├── 03-temporal-ui.yaml         # Web UI dashboard
│   └── 04-temporal-worker.yaml     # Worker template (your image)
├── .envrc.example                   # Template for secrets
├── deploy.sh                        # Deployment entrypoint
├── cleanup.sh                       # Teardown entrypoint
└── README.md                        # This file
```

## Features Demonstrated

- ✅ Config-driven declarative deployment
- ✅ Persistent volume (PostgreSQL)
- ✅ Railway service-to-service references
- ✅ Secret injection via `$env()` syntax
- ✅ Public domain generation (Temporal UI)
- ✅ TCP proxy (PostgreSQL, gRPC)
- ✅ Private container registry support
- ✅ Multi-replica workers
- ✅ Selective deployment (`--config` flags)
- ✅ Deployment status polling
