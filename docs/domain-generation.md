# Domain and TCP Networking

## Overview

railctl supports lifecycle management for service networking from the CLI:

- `--generate-domain <port>`
- `--remove-domain`
- `--generate-tcp <port>`
- `--remove-tcp`

`--generate-domain` is available on `create service` and `update service`.
Removal flags are available on `update service` only.

## Usage

```bash
# Create a service and generate a Railway domain for app port 80
railctl create service web --image nginx:latest --generate-domain 80 \
  -p my-project -e production

# Add a domain to an existing service
railctl update service web --generate-domain 80 \
  -p my-project -e production

# Remove the current domain from a service
railctl update service web --remove-domain \
  -p my-project -e production

# Add a TCP proxy to an existing service
railctl update service db --generate-tcp 5432 \
  -p my-project -e production

# Remove the current TCP proxy from a service
railctl update service db --remove-tcp \
  -p my-project -e production
```

## Behavior

### Domain generation

- Creates a Railway domain (`*.up.railway.app`) for the service
- Expects an application port argument, e.g. `--generate-domain 80`
- Idempotent
- If a domain already exists and the target port differs, it updates the port
- Custom domains take priority over Railway-generated domains when checking existing domains

### Domain removal

- Available on `update service` only
- Removes the first existing domain
- Prefers removing a custom domain before a Railway-generated domain
- Idempotent: if no domain exists, it prints a no-op message and succeeds

### TCP generation

- Creates a TCP proxy for the given application port
- Expects an application port argument, e.g. `--generate-tcp 5432`
- Idempotent for an existing proxy on the same application port

### TCP removal

- Available on `update service` only
- Removes the first existing TCP proxy
- Idempotent: if no TCP proxy exists, it prints a no-op message and succeeds

### Conflicting flags

These combinations are rejected:

```bash
railctl update service web --generate-domain 80 --remove-domain
railctl update service db --generate-tcp 5432 --remove-tcp
```

## Structured output

Networking metadata is exposed in structured output for service detail and service list commands.

Examples:

```bash
railctl describe service web -p my-project -e production -o json
railctl get services -p my-project -e production -o wide
railctl get services -p my-project -e production -o json
```

Structured fields:

- `serviceDomains`
- `customDomains`
- `tcpProxies`

## Not supported

The following is not implemented:

- config YAML support like `domain.generate: true`
- selecting a specific domain or TCP proxy to remove by ID or port

## Files

| File                               | Change                                               |
| ---------------------------------- | ---------------------------------------------------- |
| `internal/api/interface.go`        | Domain/TCP list, create, update, and delete methods  |
| `internal/api/domains.go`          | Domain queries and create/update/delete mutations    |
| `internal/api/tcp_proxy.go`        | TCP proxy queries and create/delete mutations        |
| `internal/api/mock.go`             | Mock stubs for domain/TCP methods                    |
| `internal/cmd/create_service.go`   | `--generate-domain` and `--generate-tcp` helpers     |
| `internal/cmd/update_service.go`   | generate/remove networking flags and command flow    |
| `internal/cmd/describe_service.go` | structured networking output for `describe service`  |
| `internal/cmd/get_services.go`     | structured/wide networking output for `get services` |

## Testing

```bash
# Unit and command tests
go test ./internal/api/...
go test ./internal/cmd/...

# Focused networking tests
go test ./internal/cmd -run 'TestGenerateServiceDomain|TestRemoveServiceDomain|TestRemoveTCPProxy|TestRunUpdateService_WithGenerateDomain|TestRunUpdateService_WithRemoveDomain|TestRunUpdateService_WithGenerateTCP|TestRunUpdateService_WithRemoveTCP'

# E2E tests
go build -o railctl ./cmd/railctl
RAILCTL=$(pwd)/railctl go test -tags e2e -v -timeout 10m ./tests/e2e/...
```

## Imperative domain commands

Alongside the declarative `customDomains` block, domains are manageable
imperatively:

```bash
railctl get domains -s api            # railway + custom domains, verification status
railctl create domain app.example.com -s api [--port N]   # prints the DNS records
railctl delete domain app.example.com -s api --yes
```

Custom-domain **removal is imperative-only by design** — `apply` never removes
a live domain, so a manifest edit cannot cause an accidental outage.
