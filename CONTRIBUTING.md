# Contributing to railctl

Thank you for your interest in contributing to railctl! This guide will help you get started.

## Code of Conduct

By participating in this project, you agree to abide by our [Code of Conduct](CODE_OF_CONDUCT.md).

## Getting Started

### Prerequisites

- Go 1.22+
- A [Railway](https://railway.app) account and API token
- (Optional) Docker for container builds

### Clone and Build

```bash
git clone https://github.com/kubenoops/railctl.git
cd railctl
make build
```

### Run Tests

```bash
# Unit tests
make test

# E2E tests (requires your own Railway API token)
export RAILWAY_TOKEN_1="your-railway-api-token"
RAILCTL=$(pwd)/railctl go test -tags e2e -v -timeout 10m ./tests/e2e/...
```

## Development Workflow

### Branch Strategy

- `main` — stable, released code
- Feature branches — `feature/<description>` or `fix/<description>`

### Pull Request Process

1. Fork the repository and create your branch from `main`
2. Write or update tests as needed
3. Ensure all tests pass: `make test`
4. Update documentation if you're changing behavior
5. Submit a pull request

### PR Requirements

- All PRs must pass the CI checks (build + unit tests)
- PRs that change CI/CD workflows or test infrastructure require maintainer approval
- Keep PRs focused — one feature or fix per PR

## Project Structure

```
railctl/
├── cmd/railctl/           # CLI entry point
├── internal/
│   ├── api/               # Railway GraphQL API client
│   ├── cmd/               # Cobra command implementations
│   ├── cmdutil/           # Command utilities
│   ├── output/            # Output formatting (table, JSON, YAML)
│   ├── resolver/          # Name/ID resolution logic
│   └── types/             # Data structures
├── tests/e2e/             # End-to-end tests
├── examples/              # Example configurations
├── experiments/           # Experimental tools and prototypes
└── docs/                  # Documentation
```

## Coding Standards

- Follow standard Go conventions (`gofmt`, `golint`)
- Write table-driven tests where appropriate
- Keep the API client (`internal/api/`) decoupled from commands (`internal/cmd/`)
- Use the mock client (`internal/api/mock.go`) for unit tests
- Never log sensitive values — use `security.go` utilities for masking

## E2E Testing

E2E tests run against the live Railway API. To run them locally:

1. Create a Railway account and generate an API token
2. Set environment variables:
   ```bash
   export RAILWAY_TOKEN_1="your-token"
   # Optionally set RAILWAY_TOKEN_2 and RAILWAY_TOKEN_3 for load balancing
   ```
3. Build and run:
   ```bash
   make build
   RAILCTL=$(pwd)/railctl go test -tags e2e -v -timeout 10m ./tests/e2e/...
   ```

> **Note:** E2E tests create and delete Railway projects. Use a dedicated account or team to avoid impacting production resources.

## Reporting Issues

- Use [GitHub Issues](https://github.com/kubenoops/railctl/issues)
- Include steps to reproduce, expected vs. actual behavior
- Include `railctl` version (`railctl --version`)

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
