# n8n Stack Deployment Example

This directory contains example configurations for deploying an n8n stack to Railway using `railctl`.

## Architecture

The n8n stack consists of 4 services:

1. **Postgres** - PostgreSQL database with persistent volume
2. **Redis** - Redis cache for queue management with persistent volume
3. **Primary** - Main n8n application server (web UI)
4. **Worker** - Worker nodes for executing workflows

## Prerequisites

- `railctl` CLI installed and built
- `yq` YAML processor ([installation instructions](https://github.com/mikefarah/yq))
  ```bash
  # Install via snap (recommended)
  sudo snap install yq
  
  # Or via apt (older version)
  sudo apt install yq
  ```
- Railway API token
- Environment variables configured (see `.envrc` example)

## Configuration

This deployment uses **declarative configuration files** as the single source of truth. Each service has a `config.yaml` file in its directory that defines:

- Service metadata (name, image)
- Deploy configuration (start command, restart policy, replicas)
- Volume mounts
- Environment variables

The `deploy.sh` script reads these config files using `yq` and creates services accordingly.

### Service Configuration Files

- `n8n-postgres/config.yaml` - PostgreSQL 16 with SSL support
- `n8n-redis/config.yaml` - Redis cache configuration
- `n8n-primary/config.yaml` - n8n main server (web UI + API)
- `n8n-worker/config.yaml` - n8n worker for background job execution

## Quick Start

### 1. Set Up Environment Variables

Create or edit `.envrc` with your configuration:

```bash
export RAILWAY_TOKEN="your-railway-token"
export RAILCTL_PROJECT="your-project-name"
export RAILCTL_ENVIRONMENT="test"  # or production

# n8n deployment variables
export RAILCTL_POSTGRES_PASSWORD="secure-postgres-password"
export RAILCTL_REDIS_PASSWORD="secure-redis-password"
export RAILCTL_N8N_ENCRYPTION_KEY="32-character-encryption-key"
export RAILCTL_N8N_EDITOR_BASE_URL="https://n8n.example.com"
```

Load the environment:
```bash
direnv allow  # if using direnv
# or
source .envrc
```

### 2. Deploy the Stack

```bash
./deploy.sh
```

The script will:
1. Validate dependencies (yq, railctl, environment variables)
2. Deploy infrastructure services (Postgres, Redis) with volumes
3. Deploy application services (Primary, Worker)
4. Set all environment variables with proper Railway service references

### 3. Verify Deployment

```bash
# List all services
railctl get services -p $RAILCTL_PROJECT -e $RAILCTL_ENVIRONMENT

# Check volumes
railctl get volumes -p $RAILCTL_PROJECT -e $RAILCTL_ENVIRONMENT

# View service details
railctl describe service Primary -p $RAILCTL_PROJECT -e $RAILCTL_ENVIRONMENT

# View logs
railctl logs service Primary -p $RAILCTL_PROJECT -e $RAILCTL_ENVIRONMENT
```

### 4. Clean Up

To delete all services and volumes:

```bash
./cleanup.sh
```

The cleanup script will:
1. Delete all services (Worker, Primary, Redis, Postgres)
2. Delete all orphaned volumes in the environment

## Service References

The deployment uses Railway's service reference syntax to connect services:

```yaml
# In Primary and Worker configs
DB_POSTGRESDB_HOST: "${{Postgres.RAILWAY_PRIVATE_DOMAIN}}"
DB_POSTGRESDB_PASSWORD: "${{Postgres.PGPASSWORD}}"
QUEUE_BULL_REDIS_HOST: "${{Redis.RAILWAY_PRIVATE_DOMAIN}}"
QUEUE_BULL_REDIS_PASSWORD: "${{Redis.REDIS_PASSWORD}}"
```

These references are automatically resolved by Railway at runtime, ensuring services can communicate securely over the private network.

## Features Demonstrated

- ✅ **Config-driven deployment** - All configuration in YAML files
- ✅ **Volume management** - Persistent data for Postgres and Redis
- ✅ **Service-to-service references** - Automatic service discovery
- ✅ **Deploy configuration** - Restart policies, replicas, start commands
- ✅ **Environment variable expansion** - Local vars expanded, Railway refs preserved
- ✅ **Dependency checking** - Validates all prerequisites before deployment
- ✅ **Clean teardown** - Removes all services and volumes

## Deployment Order

The script deploys services in dependency order:

1. **Phase 1: Infrastructure**
   - Postgres (database with volume)
   - Redis (cache with volume)

2. **Phase 2: Application**
   - Primary (web UI, depends on Postgres + Redis)
   - Worker (background jobs, depends on Postgres + Redis)

## Customization

Edit the `config.yaml` files to customize:

- **Image versions**: Change the `image` field
- **Resources**: Adjust `numReplicas` for scaling
- **Environment variables**: Add/modify in the `variables` section
- **Start commands**: Customize `startCommand` for different modes
- **Restart policies**: Change `restartPolicyType` and `restartPolicyMaxRetries`

### Example: Using a Custom n8n Image

Edit `n8n-primary/config.yaml` and `n8n-worker/config.yaml`:

```yaml
service:
  name: Primary
  image: ghcr.io/your-org/custom-n8n:latest

registry:
  username: "${GH_USERNAME}"
  password: "${GH_PASSWORD}"
```

Then add the credentials to `.envrc`:
```bash
export GH_USERNAME="your-github-username"
export GH_PASSWORD="your-github-token"
```

## Troubleshooting

### Deployment Issues

If deployment fails, check:

1. **Environment variables**: Ensure all required vars are set
   ```bash
   env | grep RAILCTL
   ```

2. **Service logs**: Check for errors
   ```bash
   railctl logs service Primary -p $RAILCTL_PROJECT -e $RAILCTL_ENVIRONMENT
   ```

3. **Service status**: Verify deployment status
   ```bash
   railctl describe service Primary -p $RAILCTL_PROJECT -e $RAILCTL_ENVIRONMENT
   ```

### Volume Issues

If volumes aren't being created or deleted properly:

```bash
# List all volumes
railctl get volumes -p $RAILCTL_PROJECT -e $RAILCTL_ENVIRONMENT

# Manually delete a volume
railctl delete volume <volume-name> -p $RAILCTL_PROJECT -e $RAILCTL_ENVIRONMENT --yes
```

### Debug Mode

For detailed API request/response logging, use the `--debug` flag:

```bash
railctl --debug get services -p $RAILCTL_PROJECT -e $RAILCTL_ENVIRONMENT
```

## Notes

- **Service names**: This example uses Railway template naming (`Postgres`, `Redis`, `Primary`, `Worker`)
- **Official images**: Uses official `n8nio/n8n` and `railwayapp/redis` images
- **Environment isolation**: Services are created only in the specified environment
- **Volume cleanup**: The cleanup script removes ALL orphaned volumes in the environment
