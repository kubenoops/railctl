# Docker Compose → Railway Importer

Import a `docker-compose.yml` into an existing Railway project, creating services, variables, and volumes idempotently.

## Prerequisites

- Python 3.10+
- `pyyaml` (`pip install pyyaml`)
- `curl`
- A Railway API token ([get one here](https://railway.com/account/tokens))

## Usage

```bash
# Dry run — preview without making changes
python compose_importer.py docker-compose.yml \
  --project-id <id> \
  --environment-id <id> \
  --dry-run

# Execute import
python compose_importer.py docker-compose.yml \
  --project-id <id> \
  --environment-id <id> \
  --token <token>

# Or use RAILWAY_TOKEN env var
export RAILWAY_TOKEN=<token>
python compose_importer.py docker-compose.yml \
  --project-id <id> \
  --environment-id <id>
```

## What Gets Imported

| Compose Feature | Railway Mapping |
|---|---|
| `image:` | Service source image |
| `environment:` | Railway variables |
| `env_file:` | Parsed → Railway variables |
| `command:` / `entrypoint:` | `startCommand` |
| `restart:` | `restartPolicyType` |
| `deploy.replicas` | `numReplicas` |
| `deploy.restart_policy` | `restartPolicyType` + `restartPolicyMaxRetries` |
| `healthcheck` (HTTP) | `healthcheckPath` + `healthcheckTimeout` |
| `volumes:` | Railway volume at mount path |
| `ports:` | Internal port used for networking context |

## What Gets Skipped

| Feature | Reason |
|---|---|
| `build:` | Ignored when `image:` is present; error without `image:` |
| `networks:` | Railway provides automatic private networking |
| `depends_on:` | Railway handles startup automatically |
| `links:` | Services use `{service}.railway.internal` |
| `secrets:` / `configs:` | Docker Swarm features — not supported |
| `deploy.resources` | Managed by Railway plan tier |
| `deploy.placement` | Use Railway region config instead |
| `privileged` / `cap_add` | Not supported on Railway |

## Variable Handling

### Sources (priority order)
1. `env_file:` referenced files
2. Inline `environment:` values (override env_file)
3. Shell environment (for `${VAR}` resolution)

### Transformations
- **Cross-service references** — `db:5432` → `db.railway.internal:5432`
- **`RAILWAY_*` variables** — Dropped (Railway injects its own)
- **Railway references** — `${{service.VAR}}` passed through unchanged

### Secret Convention

Files matching `*.secrets` (e.g. `.env.secrets`, `.env.db.secrets`) are treated as secret sources. Values from these files:
- Are **masked** in the import summary (`DB_PASSWORD=my***rd`)
- Are pushed to Railway the same way as other variables

> **Note:** Railway's API does not support marking variables as secrets. To seal sensitive values, use the Railway dashboard after import.

## Idempotency

The script is fully idempotent:
- Creates services that don't exist
- Updates configuration of existing services
- Skips volume creation if already attached at the same mount path
- Variables are upserted (create or update)

## Running Tests

```bash
pip install pyyaml pytest
python -m pytest test_compose_importer.py -v
```
