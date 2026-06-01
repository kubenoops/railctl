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

# Lint (the canonical lint gate)
golangci-lint run

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
2. Write or update tests as needed (see **Coding Standards** below)
3. Ensure unit tests and lint pass: `make test` and `golangci-lint run`
4. **Update documentation whenever you change behavior** — if you add or change a
   CLI flag or `RAILCTL_*` environment variable you must update `README.md`
   (the environment-variable table and usage examples). A required `docs-guard`
   CI check enforces this and will fail the PR otherwise (maintainers can apply
   the `docs-not-needed` label for genuine exceptions).
5. Submit a pull request and fill out the template checklist

### PR Requirements

- All PRs must pass the required CI checks (`test` and `docs-guard`)
- All review conversations must be resolved before merging
- PRs that change CI/CD workflows, the `Makefile`, or test infrastructure require
  code-owner approval (see `.github/CODEOWNERS`)
- Keep PRs focused — one feature or fix per PR

## Project Structure

```
railctl/
├── cmd/railctl/           # CLI entry point
├── internal/
│   ├── api/               # Railway GraphQL API client (+ mock client)
│   ├── cmd/               # Cobra command implementations
│   ├── cmdutil/           # Shared command scaffolding (ResolveContext, PrintResult)
│   ├── output/            # Centralized output formatting (table, JSON, YAML, wide)
│   ├── resolver/          # Name/ID resolution logic (exact → substring → ambiguous)
│   └── types/             # Data structures
├── tests/e2e/             # End-to-end tests (build tag: e2e)
├── examples/              # Deployment examples
│   ├── shared/            # Reusable deploy/cleanup scripts
│   ├── n8n/               # n8n queue-mode deployment
│   └── temporal/          # Temporal workflow engine deployment
├── experiments/           # Experimental tools and prototypes
└── docs/                  # Documentation
```

## Coding Standards

These mirror the conventions the codebase already follows. The automated reviewer
(`.gemini/styleguide.md`) checks against the same rules.

### General Go

- Format with `gofmt`; the canonical lint gate is **`golangci-lint`** (not the
  deprecated `golint`). Let the linter own style nits.
- Wrap errors with context: `fmt.Errorf("doing X: %w", err)`.
- This codebase deliberately does **not** thread `context.Context` through the
  API client — don't introduce it unless a change specifically calls for it.

### Architecture & layering

Respect the established layering — don't re-implement what a shared package
already provides:

- Use **`cmdutil.ResolveContext`** and **`cmdutil.PrintResult`** for command
  scaffolding (used across the command set) rather than hand-rolling resolution
  and output.
- Render output through the **`output.Printer`** layer — don't hand-roll
  JSON/YAML/table formatting.
- Resolve names/IDs through the **`resolver`** package, which follows an
  exact-match → case-insensitive substring → ambiguous contract and returns the
  `ErrNotFound` / `ErrAmbiguous` sentinel errors. New resolution logic should
  follow the same contract.
- For long-running operations, follow the existing `--await` polling pattern
  (terminal-status detection with backoff).
- Keep the API client (`internal/api/`) decoupled from commands (`internal/cmd/`).

### CLI conventions

- Every command sets Cobra `Use`/`Short`/`Long`/`Example` and uses `RunE`.
- Flag values resolve in the order **flag → environment variable → default**.
- Provide actionable validation errors (e.g. list the valid options when input
  is invalid).
- Never log sensitive values — use the masking utilities in `internal/api/security.go`.

### Testing

- Write **table-driven tests** placed beside the code they cover.
- Cover the meaningful scenarios exhaustively — including **not-found**,
  **ambiguous**, and **empty** cases, not just the happy path.
- Use the mock client (`internal/api/mock.go`) for unit tests.
- E2E tests live behind the `e2e` build tag.

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
