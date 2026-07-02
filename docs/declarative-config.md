# Declarative Configuration Reference

## Overview

railctl supports declarative resource management via YAML config files.
Define your desired infrastructure state in a config file, then use
`railctl apply` to create/update resources and `railctl diff` to preview changes.

## Commands

### `railctl apply`

```bash
railctl apply -f <file-or-directory> [flags]
```

Apply a declarative config to Railway. Creates, updates, or deletes services
to match the desired state.

**Flags:**

| Flag              | Short | Description                                      |
| ----------------- | ----- | ------------------------------------------------ |
| `--file`          | `-f`  | Path to YAML config file or directory (required) |
| `--dry-run`       |       | Show what would change without applying          |
| `--prune`         |       | Delete services not in the config file           |
| `--await`         |       | Wait for deployments to reach terminal status    |
| `--await-timeout` |       | Timeout in seconds for --await (default: 600)    |
| `--no-color`      |       | Disable colored output                           |
| `--color`         |       | Force colored output even when not a terminal (CI) |
| `--project`       | `-p`  | Project name (overrides config file)             |
| `--environment`   | `-e`  | Environment name (overrides config file)         |

### `railctl diff`

```bash
railctl diff -f <file-or-directory> [flags]
```

Compare config file against current Railway state. Exits with code 0 if
no changes, 1 if differences exist (useful for CI/CD).

**Flags:**

| Flag         | Short | Description                                      |
| ------------ | ----- | ------------------------------------------------ |
| `--file`     | `-f`  | Path to YAML config file or directory (required) |
| `--prune`    |       | Include unmanaged resources in diff              |
| `--no-color` |       | Disable colored output                           |
| `--color`    |       | Force colored output even when not a terminal (CI) |

## Config File Schema

### Full Format

```yaml
# Optional - CLI flags -p/-e override these
project: my-app
environment: production

# One or more services
services:
  - name: api
    image: node:20-alpine

    deploy:
      startCommand: "npm start"
      restartPolicy: ON_FAILURE # ON_FAILURE | ALWAYS | NEVER
      maxRetries: 3
      replicas: 2
      healthcheckPath: /health
      healthcheckTimeout: 300

    networking:
      domain:
        port: 3000 # Generate a Railway domain on this port
      tcpProxy:
        port: 5432 # Application port for TCP proxy

    volume:
      mountPath: /app/data

    variables:
      PORT: "3000"
      DATABASE_URL: "$env(DATABASE_URL)" # Expanded from local env
      REDIS_URL: "${{redis.REDIS_URL}}" # Railway service reference

    registry: # For private Docker registries
      username: "$env(REGISTRY_USER)"
      password: "$env(REGISTRY_PASS)"
```

### Schema Reference

#### `project` (optional, string)

Railway project name. Can be overridden by the `-p` CLI flag or the `RAILCTL_PROJECT` environment variable.

#### `environment` (optional, string)

Railway environment name. Can be overridden by the `-e` CLI flag or the `RAILCTL_ENVIRONMENT` environment variable.

#### `services` (required, array)

List of service definitions. At least one service must be defined. Duplicate service names within a config are not allowed.

---

#### `services[].name` (required, string)

Service name. Used to match against existing Railway services for create-or-update logic.

#### `services[].image` (required, string)

Docker image reference (e.g., `node:20-alpine`, `registry.example.com/myapp:v1`). Supports `$env()` expansion.

#### `services[].deploy` (optional, object)

Deployment configuration for the service.

| Field                | Type   | Default             | Description                                                                                      |
| -------------------- | ------ | ------------------- | ------------------------------------------------------------------------------------------------ |
| `startCommand`       | string | (none)              | Override the container's default start command                                                   |
| `restartPolicy`      | string | (none)              | Restart behavior: `ON_FAILURE`, `ALWAYS`, or `NEVER` (case-insensitive, normalized to uppercase) |
| `maxRetries`         | int    | 0                   | Maximum restart attempts. Requires `restartPolicy` to be set                                     |
| `replicas`           | int    | 0 (Railway default) | Number of instances for horizontal scaling. Must be >= 1 if set                                  |
| `healthcheckPath`    | string | (none)              | HTTP endpoint for health checks (e.g., `/health`)                                                |
| `healthcheckTimeout` | int    | 0 (Railway default) | Maximum seconds to wait for health check response                                                |

#### `services[].networking` (optional, object)

Network configuration for the service.

##### `services[].networking.domain` (optional, object)

| Field  | Type | Default | Description                                                                       |
| ------ | ---- | ------- | --------------------------------------------------------------------------------- |
| `port` | int  | 0       | Application port to expose via a Railway-generated domain. Must be 1-65535 if set |

##### `services[].networking.tcpProxy` (optional, object)

| Field  | Type | Default | Description                                                      |
| ------ | ---- | ------- | ---------------------------------------------------------------- |
| `port` | int  | 0       | Application port to expose via TCP proxy. Must be 1-65535 if set |

#### `services[].volume` (optional, object)

Persistent volume configuration. One volume per service.

| Field       | Type   | Default | Description                                                      |
| ----------- | ------ | ------- | ---------------------------------------------------------------- |
| `mountPath` | string | (none)  | Filesystem path where the volume is mounted inside the container |

#### `services[].variables` (optional, map[string]string)

Environment variables for the service. Keys are variable names, values are strings. Supports `$env()` expansion and `${{}}` Railway service references.

#### `services[].registry` (optional, object)

Private Docker registry credentials. Required when `image` references a private registry.

| Field      | Type   | Default | Description                                    |
| ---------- | ------ | ------- | ---------------------------------------------- |
| `username` | string | (none)  | Registry username. Supports `$env()` expansion |
| `password` | string | (none)  | Registry password. Supports `$env()` expansion |

Both `username` and `password` must be set for credentials to be applied.

Railway never returns stored credentials, so `apply` can't diff them — it
re-applies the declared credentials on any update to the service and shows them
(password masked) in the diff.

**Removal is not supported:** the API can't clear stored credentials, so removing
the `registry` block does not remove them from the service — use the dashboard.

## Variable Expansion

### `$env(VAR_NAME)`

Expands to the value of the local environment variable at load time. Fails with an error if the variable is not set.

Supported in these fields:

- `image`
- `deploy.startCommand`, `deploy.restartPolicy`, `deploy.healthcheckPath`
- `registry.username`, `registry.password`
- All `variables` values

```yaml
variables:
  DATABASE_URL: "$env(DATABASE_URL)"
registry:
  username: "$env(REGISTRY_USER)"
  password: "$env(REGISTRY_PASS)"
```

### `${{service.VAR}}`

Railway service reference. Passed through to the Railway API as-is and resolved at runtime by Railway. Use this to reference variables from other services in your project.

```yaml
variables:
  DATABASE_URL: "${{postgres.DATABASE_URL}}"
  REDIS_URL: "${{redis.REDIS_URL}}"
```

## Legacy Format (Backward Compatible)

The old per-service format is auto-detected and converted. A file is recognized as legacy when it has a top-level `service` key with a `name` field and no top-level `services` key.

```yaml
service:
  name: postgres
  image: postgres:16
deploy:
  startCommand: "..."
  restartPolicyType: ON_FAILURE # maps to restartPolicy
  restartPolicyMaxRetries: 10 # maps to maxRetries
  numReplicas: 2 # maps to replicas
networking:
  tcpProxyPort: 5432 # maps to networking.tcpProxy.port
domain:
  port: 0
volume:
  mountPath: /data
variables:
  KEY: value
```

**Field mapping from legacy to current format:**

| Legacy Field                     | Current Field                         |
| -------------------------------- | ------------------------------------- |
| `service.name`                   | `services[].name`                     |
| `service.image`                  | `services[].image`                    |
| `deploy.restartPolicyType`       | `services[].deploy.restartPolicy`     |
| `deploy.restartPolicyMaxRetries` | `services[].deploy.maxRetries`        |
| `deploy.numReplicas`             | `services[].deploy.replicas`          |
| `networking.tcpProxyPort`        | `services[].networking.tcpProxy.port` |
| `domain.port`                    | `services[].networking.domain.port`   |

## Directory Loading

When `-f` points to a directory, all `*.yaml` and `*.yml` files are loaded
alphabetically and merged into a single config. Use numbered prefixes for
ordering:

```
configs/
├── 01-postgres.yaml
├── 02-redis.yaml
└── 03-api.yaml
```

If multiple files specify `project` or `environment`, they must agree (same value) or loading fails with a conflict error.

## Examples

### Simple Service

```yaml
services:
  - name: api
    image: node:20-alpine
```

### Service with Deployment Config

```yaml
services:
  - name: api
    image: node:20-alpine
    deploy:
      startCommand: "npm start"
      replicas: 2
      healthcheckPath: /health
    networking:
      domain:
        port: 3000
    variables:
      PORT: "3000"
      NODE_ENV: "production"
```

### Full Stack (Postgres + Redis + App)

```yaml
project: my-app
environment: production

services:
  - name: postgres
    image: postgres:16
    deploy:
      startCommand: "docker-entrypoint.sh postgres"
    networking:
      tcpProxy:
        port: 5432
    volume:
      mountPath: /var/lib/postgresql/data
    variables:
      POSTGRES_USER: "app"
      POSTGRES_PASSWORD: "$env(POSTGRES_PASSWORD)"
      POSTGRES_DB: "app"

  - name: redis
    image: redis:7-alpine
    deploy:
      startCommand: "redis-server"
    networking:
      tcpProxy:
        port: 6379
    volume:
      mountPath: /data

  - name: api
    image: node:20-alpine
    deploy:
      startCommand: "npm start"
      restartPolicy: ON_FAILURE
      maxRetries: 3
      replicas: 2
      healthcheckPath: /health
      healthcheckTimeout: 60
    networking:
      domain:
        port: 3000
    variables:
      PORT: "3000"
      DATABASE_URL: "${{postgres.DATABASE_URL}}"
      REDIS_URL: "${{redis.REDIS_URL}}"
```

### CI/CD Integration

```bash
# In CI: check for drift
railctl diff -f config.yaml -p my-app -e production
# Exit code 0 = no changes, 1 = drift detected

# Deploy
railctl apply -f config.yaml -p my-app -e production --await

# Dry run in CI to preview changes
railctl apply -f config.yaml -p my-app -e production --dry-run
```

### Private Registry

```yaml
services:
  - name: app
    image: registry.example.com/myapp:v1
    registry:
      username: "$env(REGISTRY_USER)"
      password: "$env(REGISTRY_PASS)"
    deploy:
      startCommand: "npm start"
    networking:
      domain:
        port: 3000
```
