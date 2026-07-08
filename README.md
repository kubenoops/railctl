# railctl

[![PR Tests](https://github.com/kubenoops/railctl/actions/workflows/pr.yml/badge.svg?event=pull_request)](https://github.com/kubenoops/railctl/actions/workflows/pr.yml)
[![Go Version](https://img.shields.io/badge/go-1.22+-blue.svg)](https://golang.org)
[![Test Coverage](https://img.shields.io/badge/coverage-80%25-brightgreen.svg)](docs/testing-architecture.md)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A kubectl-style CLI for [Railway.app](https://railway.app) - manage your Railway infrastructure with familiar, powerful commands.

## Quick Start

### Installation

#### From GitHub Releases

Download the latest binary from the [Releases](https://github.com/kubenoops/railctl/releases) page:

```bash
# Linux (amd64)
curl -Lo railctl https://github.com/kubenoops/railctl/releases/latest/download/railctl-linux-amd64
chmod +x railctl
sudo mv railctl /usr/local/bin/

# Linux (arm64)
curl -Lo railctl https://github.com/kubenoops/railctl/releases/latest/download/railctl-linux-arm64
chmod +x railctl
sudo mv railctl /usr/local/bin/

# macOS (Apple Silicon)
curl -Lo railctl https://github.com/kubenoops/railctl/releases/latest/download/railctl-darwin-arm64
chmod +x railctl
sudo mv railctl /usr/local/bin/

# macOS (Intel)
curl -Lo railctl https://github.com/kubenoops/railctl/releases/latest/download/railctl-darwin-amd64
chmod +x railctl
sudo mv railctl /usr/local/bin/
```

#### Using Go Install

```bash
go install github.com/kubenoops/railctl/cmd/railctl@latest
```

#### From Source

```bash
git clone https://github.com/kubenoops/railctl.git
cd railctl
make build
sudo mv railctl /usr/local/bin/
```

### Authentication

```bash
# Get your API token from https://railway.app/account/tokens
export RAILWAY_TOKEN=your-api-token-here
```

railctl automatically detects the token type on first use — no extra configuration needed:

| Token type | What it can do |
|------------|----------------|
| **Account** (personal) | Access all workspaces and projects |
| **Workspace-scoped** | Access all projects in one workspace |
| **Project-scoped** | Access one project and environment |

When using a workspace or project token, flags like `-w`, `-p`, and `-e` (or their `RAILCTL_*` equivalents) are ignored with a warning — the scope is already baked into the token.

### Basic Usage

```bash
# List all projects
railctl get projects

# Create a new project
railctl create project my-app

# List services in production
railctl get services -p my-app -e production

# Deploy a new service
railctl create service api --image node:20-alpine -p my-app

# Update service image (triggers deployment)
railctl update service api --image node:20 -p my-app -e production
```

### Built-in usage guide (`railctl skill`)

railctl ships a self-contained usage guide **embedded in the binary** — the
command surface, declarative `apply`/`diff`, volume backups, and the token
model (account/workspace vs project-scoped tokens and their limitations). It is
written to be consumed by AI agents as well as humans, needs no network access,
and always matches the version you are running:

```bash
railctl skill                 # print the guide
railctl skill | less
railctl skill > railctl.skill.md   # save it as a portable skill
```

The source of truth is [`docs/railctl-skill.md`](docs/railctl-skill.md). A
byte-identical copy at `internal/skill/railctl-skill.md` is compiled into the
binary via `//go:embed` (the directive cannot reach outside its package, so the
copy lives beside the code). Regenerate the copy with `make gen`; a CI check
(`skill-sync`) fails if the two drift.

## Documentation

- **[CI/CD & Build Setup](docs/ci-build-setup.md)** - Build system, GitHub Actions workflows, release lifecycle, and dependency management
- **[Testing Architecture](docs/testing-architecture.md)** - Three-tier testing strategy, mock patterns, E2E test harness, and coverage matrix
- **[Railway Service Creation Behavior](docs/railway-service-creation-behavior.md)** - Understanding Railway's automatic service instance creation and our workaround

## Declarative Configuration

Manage your Railway infrastructure as code with YAML config files:

```bash
# Define your stack
cat > stack.yaml <<EOF
services:
  - name: api
    image: node:20-alpine
    deploy:
      startCommand: "npm start"
      replicas: 2
    networking:
      domain:
        port: 3000
    variables:
      PORT: "3000"
      DATABASE_URL: "${{postgres.DATABASE_URL}}"
EOF

# Preview changes
railctl diff -f stack.yaml -p my-app -e production

# Apply changes
railctl apply -f stack.yaml -p my-app -e production

# Apply with deployment wait
railctl apply -f stack.yaml -p my-app -e production --await
```

See **[Declarative Configuration Reference](docs/declarative-config.md)** for the full schema.

## Usage Guide

### Projects

```bash
# List all projects
railctl get projects
railctl get projects -o wide    # Show more details
railctl get projects -o json    # JSON output
railctl get projects -o yaml    # YAML output

# Describe a specific project
railctl describe project my-app

# Create a new project
railctl create project my-new-app

# Delete a project (with confirmation)
railctl delete project old-app
railctl delete project old-app --yes  # Skip confirmation
```

### Environments

```bash
# List environments in a project
railctl get environments -p my-app
railctl get envs -p my-app  # Short alias

# Create an environment
railctl create environment staging -p my-app

# Delete an environment
railctl delete environment staging -p my-app --yes
```

### Services

```bash
# List services in an environment
railctl get services -p my-app -e production
railctl get svc -p my-app -e production  # Short alias

# Describe a service
railctl describe service api -p my-app -e production

# Create a service from Docker image
railctl create service api --image node:20-alpine -p my-app

# Create with deployment configuration
railctl create service api --image node:20 \
  --start-command "npm start" \
  --restart-policy ON_FAILURE \
  --max-retries 3 \
  --replicas 2 \
  -p my-app

# Create with health checks for zero-downtime deployments
railctl create service api --image node:20 \
  --healthcheck-path /health \
  --healthcheck-timeout 60 \
  -p my-app

# Update service image (triggers new deployment)
railctl update service api --image node:20 -p my-app -e production

# Update deployment configuration
railctl update service api \
  --restart-policy ON_FAILURE \
  --max-retries 5 \
  --replicas 3 \
  -p my-app -e production

# Delete a service
railctl delete service api -p my-app -e production --yes
```

#### Deployment Configuration Flags

Control how your services run and scale:

| Flag                    | Description                                            | Valid Values                    |
| ----------------------- | ------------------------------------------------------ | ------------------------------- |
| `--start-command`       | Override container's default start command             | Any string                      |
| `--restart-policy`      | Control restart behavior                               | `ON_FAILURE`, `ALWAYS`, `NEVER` |
| `--max-retries`         | Maximum restart attempts (requires `--restart-policy`) | Integer >= 0                    |
| `--replicas`            | Number of instances (horizontal scaling)               | Integer >= 1                    |
| `--healthcheck-path`    | HTTP endpoint for health checks                        | Path (e.g., `/health`)          |
| `--healthcheck-timeout` | Max seconds to wait for health check                   | Integer (default: 300)          |

**Note:** These flags are available for both `create service` and `update service` commands.

### Logs

```bash
# View recent deployment logs
railctl logs service api -p my-app -e production

# View last 50 log lines
railctl logs service api --tail 50 -p my-app -e production

# Follow logs in real-time (like tail -f)
railctl logs service api -f -p my-app -e production

# View logs from specific deployment
railctl logs service api --deployment abc123 -p my-app -e production
```

### Volumes

```bash
# List volumes in an environment
railctl get volumes -p my-app -e production
railctl get volumes -o wide  # Show volume IDs and detailed sizes
railctl get volumes -o json  # JSON output

# Create a volume attached to a service
railctl create volume --mount-path /app/data -s backend -p my-app -e production
railctl create volume my-data --mount-path /app/uploads -s api -p my-app -e production

# Update volume properties
railctl update volume my-data --name uploads -p my-app -e production
railctl update volume my-data --mount-path /app/uploads -p my-app -e production
railctl update volume my-data --attach -s backend -p my-app -e production
railctl update volume my-data --detach -p my-app -e production

# Delete a volume
railctl delete volume my-data -p my-app -e production
railctl delete volume my-data --yes -p my-app -e production  # Skip confirmation
```

### Volume Backups

Railway can back up a volume manually or on an automated schedule (daily,
weekly, or monthly — retention is fixed by Railway per kind). Backup schedules
are best managed declaratively via the `backupSchedules` field (see
[Declarative Configuration](docs/declarative-config.md)); the commands below
cover manual operations.

```bash
# List a volume's backups
railctl get backups my-data -p my-app -e production
railctl get backups my-data -o wide          # include IDs and referenced size
railctl get backups my-data --schedules      # list automated schedules instead

# Create a manual backup
railctl create backup my-data -p my-app -e production
railctl create backup my-data --name pre-migration -p my-app -e production

# Delete a backup (scoped to its volume)
railctl delete backup <backup-id> --volume my-data -p my-app -e production
railctl delete backup <backup-id> --volume my-data --yes -p my-app -e production

# Restore a volume from a backup
# Note: Railway stages a new volume — deploy the service to finalize the
# restore. Backups created after the restored point in time are removed.
railctl restore backup <backup-id> --volume my-data -p my-app -e production
```

### Environment Variables

```bash
# List variables for a service
railctl get variables -p my-app -e production -s api
railctl get vars -p my-app -e production -s api  # Short alias

# Set a single variable (triggers deployment)
railctl set variable DATABASE_URL=postgres://... -p my-app -e production -s api

# Set multiple variables at once
railctl set variable API_KEY=abc123 DEBUG=true -p my-app -e production -s api

# Set variables without triggering deployment
railctl set variable FEATURE_FLAG=enabled --skip-deployment -p my-app -e production -s api

# Delete a variable
railctl delete variable OLD_KEY -p my-app -e production -s api --yes
```

### Using Environment Variables for Context

Avoid repeating flags by setting context variables:

```bash
# Set your working context
export RAILCTL_WORKSPACE=my-team
export RAILCTL_PROJECT=my-app
export RAILCTL_ENVIRONMENT=production
export RAILCTL_SERVICE=api

# Now commands are much shorter
railctl get projects
railctl get variables
railctl set variable NEW_VAR=value
railctl delete variable OLD_VAR --yes
```

### Private Docker Registries

Deploy from private registries (requires Railway Pro plan):

```bash
# Using flags
railctl create service app \
  --image registry.example.com/myapp:v1 \
  --registry-username user \
  --registry-password token \
  -p my-project

# Using environment variables (recommended)
export RAILCTL_REGISTRY_USERNAME=user
export RAILCTL_REGISTRY_PASSWORD=token
railctl create service app --image registry.example.com/myapp:v1 -p my-project

# Update with new image from private registry
railctl update service app \
  --image registry.example.com/myapp:v2 \
  --registry-username user \
  --registry-password token \
  -p my-project -e production
```

## Configuration

### Global Flags

These flags are available on every command:

| Flag | Short | Description |
|------|-------|-------------|
| `--token` | | Railway API token (default: `RAILWAY_TOKEN` env var) |
| `--workspace` | `-w` | Workspace name (default: `RAILCTL_WORKSPACE` env var) |
| `--project` | `-p` | Project name (default: `RAILCTL_PROJECT` env var) |
| `--environment` | `-e` | Environment name (default: `RAILCTL_ENVIRONMENT` env var) |
| `--service` | `-s` | Service name (default: `RAILCTL_SERVICE` env var) |
| `--output` | `-o` | Output format: `table`, `wide`, `json`, `yaml` (default: `table`) |

### Environment Variables

| Variable                    | Description                  | Example         |
| --------------------------- | ---------------------------- | --------------- |
| `RAILWAY_TOKEN`             | Railway API token (required) | `frp_xxxxxxxxx` |
| `RAILCTL_PROJECT`           | Default project name/ID      | `my-app`        |
| `RAILCTL_ENVIRONMENT`       | Default environment name/ID  | `production`    |
| `RAILCTL_SERVICE`           | Default service name/ID      | `api`           |
| `RAILCTL_REGISTRY_USERNAME` | Docker registry username     | `myuser`        |
| `RAILCTL_REGISTRY_PASSWORD` | Docker registry password     | `mytoken`       |
| Variable | Description | Example |
|----------|-------------|---------|
| `RAILWAY_TOKEN` | Railway API token (required) | `frp_xxxxxxxxx` |
| `RAILCTL_WORKSPACE` | Default workspace name (required when multiple workspaces exist) | `my-team` |
| `RAILCTL_PROJECT` | Default project name | `my-app` |
| `RAILCTL_ENVIRONMENT` | Default environment name | `production` |
| `RAILCTL_SERVICE` | Default service name | `api` |
| `RAILCTL_REGISTRY_USERNAME` | Docker registry username | `myuser` |
| `RAILCTL_REGISTRY_PASSWORD` | Docker registry password | `mytoken` |

### Output Formats

All `get` and `describe` commands support multiple output formats:

- **Table** (default) - Human-readable tabular output
- **Wide** (`-o wide`) - Table with additional columns
- **JSON** (`-o json`) - Machine-readable JSON
- **YAML** (`-o yaml`) - YAML format

## Examples

The [`examples/`](examples/) directory contains production-ready deployment templates
that show how to deploy real-world stacks on Railway using `railctl`:

| Example                            | Description                           | Services                                              |
| ---------------------------------- | ------------------------------------- | ----------------------------------------------------- |
| **[n8n](examples/n8n/)**           | n8n workflow automation in queue mode | PostgreSQL, Redis, n8n Primary, n8n Worker (×2)       |
| **[Temporal](examples/temporal/)** | Temporal durable workflow engine      | PostgreSQL, Temporal Server, Temporal UI, Worker (×2) |

Each example includes:

- Declarative YAML config files for every service
- A shared [`deploy.sh`](examples/shared/deploy.sh) script that handles idempotent create-or-update
- Cleanup scripts for full teardown
- `.envrc.example` templates for secrets
- Detailed README with architecture diagrams

```bash
# Quick start with the n8n example
cd examples/n8n
cp .envrc.example .envrc
# Edit .envrc with your Railway token and secrets
source .envrc
./deploy.sh
```

## Development

### Prerequisites

- Go 1.22 or higher
- Railway API token

### Building

```bash
# Build (version auto-detected from git tags)
make build

# Build with specific version
make build VERSION=v1.0.0

# Build for all platforms (linux/darwin, amd64/arm64)
make build-all

# Install to GOPATH/bin
make install

# Run directly
go run ./cmd/railctl --help
```

### Testing

```bash
# Run unit + integration tests
make test

# Run E2E tests (requires RAILWAY_TOKEN; builds binary first)
make test-e2e

# E2E with verbose output
E2E_VERBOSE=1 make test-e2e

# E2E keeping resources for debugging
E2E_KEEP=1 make test-e2e

# Run with coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

See **[Testing Architecture](docs/testing-architecture.md)** for the full testing strategy.

### Code Quality

```bash
# Format code
make fmt

# Run linter
make lint
```

### Project Structure

```
railway-cli/
├── .github/
│   ├── CODEOWNERS            # Pipeline change protection
│   ├── dependabot.yml        # Dependency update config
│   └── workflows/
│       ├── pr.yml             # PR unit test workflow
│       ├── release.yml        # Auto pre-release + build workflow
│       └── manual-release.yml # Manual release dispatch
├── cmd/railctl/              # CLI entry point
│   └── main.go               # Main entrypoint
├── internal/                 # Internal packages
│   ├── api/                  # Railway GraphQL API client
│   │   ├── client.go         # HTTP/GraphQL client
│   │   ├── projects.go       # Project operations
│   │   ├── environments.go   # Environment operations
│   │   ├── services.go       # Service operations
│   │   ├── variables.go      # Variable operations
│   │   ├── deployments.go    # Deployment operations
│   │   ├── interface.go      # API interface definition
│   │   └── mock.go           # Mock client for testing
│   ├── cmd/                  # Cobra command implementations
│   │   ├── root.go           # Root command
│   │   ├── get_*.go          # Get commands
│   │   ├── create_*.go       # Create commands
│   │   ├── update_*.go       # Update commands
│   │   ├── delete_*.go       # Delete commands
│   │   └── describe_*.go     # Describe commands
│   ├── output/               # Output formatting
│   │   ├── table.go          # Table formatter
│   │   ├── json.go           # JSON formatter
│   │   └── yaml.go           # YAML formatter
│   ├── resolver/             # Name/ID resolution
│   │   └── resolver.go       # Resolve names to IDs
│   └── types/                # Domain models
│       └── types.go          # Shared type definitions
├── tests/e2e/                # End-to-end tests
│   ├── run.sh                # E2E test suite (~100 tests)
│   └── README.md             # E2E documentation
├── docs/                     # Documentation
│   ├── testing-architecture.md
│   ├── ci-build-setup.md
│   └── railway-service-creation-behavior.md
├── Makefile                  # Build automation
├── SKILL.md                  # Development guidelines
├── go.mod                    # Go module definition
└── README.md                 # This file
```

### Registry Credentials Note

When updating a service from a private Docker registry, you must re-provide registry credentials due to Railway API limitations. The API replaces configuration rather than merging, and credentials are encrypted.

**Workaround:** Set environment variables once:

```bash
export RAILCTL_REGISTRY_USERNAME=user
export RAILCTL_REGISTRY_PASSWORD=token
```

## Documentation

- **[README.md](README.md)** - This file
- **[SKILL.md](SKILL.md)** - Development guidelines and patterns
- **[Declarative Configuration](docs/declarative-config.md)** - Config file schema, variable expansion, and examples

## License

MIT License - see LICENSE file for details

## Credits

Built with:

- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Go](https://golang.org) - Programming language
- [Railway API](https://railway.app) - Infrastructure platform

---

**Note:** This is an unofficial CLI tool. For the official Railway CLI, see [railway.app/cli](https://docs.railway.app/develop/cli).

# E2E trigger test
