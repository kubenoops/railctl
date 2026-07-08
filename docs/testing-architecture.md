# Testing Architecture

## Overview

railctl uses a three-tier testing strategy:

```text
┌──────────────────────────────────────────────────────────┐
│                    E2E Tests (Tier 3)                   │
│     Live Railway API, real binary, real resources       │
│    tests/e2e/{account,workspace,project} + harness/     │
├──────────────────────────────────────────────────────────┤
│              Integration Tests (Tier 2)                 │
│   Cobra commands + mocked API client + output checks    │
│          Go test files in internal/cmd/                 │
├──────────────────────────────────────────────────────────┤
│                   Unit Tests (Tier 1)                   │
│          Pure logic tests (resolver, formatter)         │
│          Go test files alongside source files           │
└──────────────────────────────────────────────────────────┘
```

| Tier            | Scope                 | Speed            | Dependencies      | Location        |
| --------------- | --------------------- | ---------------- | ----------------- | --------------- |
| **Unit**        | Functions/methods     | Fast (~ms)       | None              | `internal/*/`   |
| **Integration** | Commands + mock API   | Medium (~s)      | Mock client       | `internal/cmd/` |
| **E2E**         | Full CLI binary + API | Slow (~5-10 min) | Railway tokens (per scope) | `tests/e2e/`    |

---

## Tier 1: Unit Tests

### Purpose

Test individual functions and methods in isolation, with no external dependencies.

### Location

Unit test files live alongside the code they test:

```text
internal/
├── resolver/
│   ├── resolver.go
│   └── resolver_test.go
├── output/
│   ├── table.go
│   ├── table_test.go
│   ├── format.go
│   └── format_test.go
├── types/
│   ├── project.go
│   ├── project_test.go
│   ├── time.go
│   └── time_test.go
└── api/
    ├── client_test.go
    ├── domains_test.go
    ├── tcp_proxy_test.go
    └── ...
```

### Run

```bash
go test ./...
go test -cover ./...
```

---

## Tier 2: Integration Tests (Mock-based)

### Purpose

Test Cobra command implementations end-to-end within Go, using a mocked API client. These verify:

- Flag parsing and validation
- Error handling paths
- Output formatting
- Command argument processing

### Location

```text
internal/cmd/
├── cmd_test.go
├── root_test.go
├── services_test.go
├── update_service_test.go
├── variables_test.go
└── ...
```

### Run

```bash
go test ./internal/cmd/...
go test -race ./internal/cmd/
```

---

## Tier 3: End-to-End (E2E) Tests

### Purpose

Validate the entire CLI binary against the live Railway API. These tests exercise the real network calls, authentication, and API behavior.

### Location

The suite is split into **three groups keyed to Railway token scope** — each
group runs under exactly its own token type and tests only what is exclusive
to that layer (see `tests/e2e/README.md` and `docs/token-capability-matrix.md`):

```text
tests/e2e/
├── harness/      # shared package: Env/runner/assertions + token-type preflight
├── account/      # L1 — workspace enumeration & -w disambiguation (RAILWAY_ACCOUNT_TOKEN)
├── workspace/    # L2 — project/env lifecycle, minting, smoke (RAILWAY_WORKSPACE_TOKEN)
└── project/      # L3 — the bulk: all in-scope mechanics + boundary fail-fasts
                  #      (TestMain mints its own project token from RAILWAY_WORKSPACE_TOKEN)
```

Each group's `TestMain` classifies its token with the same detection railctl
uses and refuses to run under a mismatched type.

### Test Levels

| Group / test       | What it covers                                    | Duration | Command                  |
| ------------------ | ------------------------------------------------- | -------- | ------------------------ |
| **TestSmoke**      | Full lifecycle, one assertion per command          | ~1 min   | `make test-smoke`        |
| **account**        | Workspace enumeration, `-w` disambiguation         | ~10 s    | `make test-e2e-account`  |
| **workspace**      | Project/env lifecycle, token minting, smoke        | ~4 min   | `make test-e2e-workspace`|
| **project**        | Bulk mechanics + fail-fast boundaries (bulk group) | ~7 min   | `make test-e2e-project`  |
| **Full suite**     | All three groups, top-down                         | ~12 min  | `make test-e2e`          |

### Running

```bash
# Fast smoke test (~1min, workspace group)
make test-smoke

# Full E2E suite (~12min; needs RAILWAY_ACCOUNT_TOKEN + RAILWAY_WORKSPACE_TOKEN)
make test-e2e

# One group / one test directly
go build -o railctl ./cmd/railctl
RAILCTL=$(pwd)/railctl RAILWAY_WORKSPACE_TOKEN=... \
  go test -tags e2e -v -run TestBoundaries ./tests/e2e/project/...
```

---

## Test Coverage Matrix

### Commands vs Test Tiers

| Command                            | Unit | Integration | E2E |
| ---------------------------------- | ---- | ----------- | --- |
| `get projects`                     | —    | ✅          | ✅  |
| `describe project`                 | —    | ✅          | ✅  |
| `create project`                   | —    | ✅          | ✅  |
| `delete project`                   | —    | ✅          | ✅  |
| `get environments`                 | —    | ✅          | ✅  |
| `describe environment`             | —    | ✅          | ✅  |
| `create environment`               | —    | ✅          | ✅  |
| `delete environment`               | —    | ✅          | ✅  |
| `get services`                     | —    | ✅          | ✅  |
| `describe service`                 | —    | ✅          | ✅  |
| `create service`                   | —    | ✅          | ✅  |
| `create service --generate-domain` | ✅   | ✅          | ✅  |
| `create service --generate-tcp`    | ✅   | ✅          | ✅  |
| `update service`                   | —    | ✅          | ✅  |
| `update service --generate-domain` | ✅   | ✅          | ✅  |
| `update service --remove-domain`   | ✅   | ✅          | ✅  |
| `update service --generate-tcp`    | ✅   | ✅          | ✅  |
| `update service --remove-tcp`      | ✅   | ✅          | ✅  |
| `delete service`                   | —    | ✅          | ✅  |
| `set variable`                     | —    | ✅          | ✅  |
| `get variables`                    | —    | ✅          | ✅  |
| `delete variable`                  | —    | ✅          | ✅  |
| `get volumes`                      | —    | ✅          | ✅  |
| `describe volume`                  | —    | ✅          | ✅  |
| `create volume`                    | —    | ✅          | ✅  |
| `update volume`                    | —    | ✅          | ✅  |
| `delete volume`                    | —    | ✅          | ✅  |
| `get deployments`                  | —    | ✅          | ✅  |
| `create deployment`                | —    | ✅          | ✅  |
| `delete deployment`                | —    | ✅          | ✅  |
| `update deployment`                | —    | ✅          | ✅  |
| `logs`                             | —    | ✅          | ✅  |
| Resolver (substring)               | ✅   | —           | ✅  |
| Output formatting                  | ✅   | ✅          | ✅  |

### Output Formats Tested (E2E)

Every `get` and `describe` command is tested with all four output formats:

- `table` (default)
- `wide`
- `json` (`-o json`)
- `yaml` (`-o yaml`)

### Error Scenarios Tested (E2E)

| Category       | Examples                                                                  |
| -------------- | ------------------------------------------------------------------------- |
| Missing flags  | `-p`, `-e`, `-s` omitted                                                  |
| Invalid inputs | Bad token, nonexistent resources, invalid formats                         |
| Validation     | Empty keys, bad mount paths, invalid restart policies                     |
| Flag conflicts | `--max-retries` without `--restart-policy`; generate/remove conflicts     |
| Idempotency    | `--generate-domain` skips if domain exists; remove is a no-op when absent |
| Edge cases     | Substring resolution, env var overrides                                   |

---

## CI/CD Integration

### Workflows

The project uses two GitHub Actions workflows:

- **`pr.yml`** — Runs on every PR. Executes unit + integration tests (`make test`), linting, and build verification.
- **E2E tests are run locally** by contributors using their own Railway API tokens. See `tests/e2e/README.md` for setup.

### When to Run Each Tier

| Trigger              | Unit           | Integration    | E2E                   |
| -------------------- | -------------- | -------------- | --------------------- |
| Every commit (local) | ✅ `make test` | ✅ `make test` | —                     |
| PR to main (CI)      | ✅ auto        | ✅ auto        | — (run locally)       |
| Merge to main (CI)   | —              | —              | — (run locally)       |
| Quick sanity check   | —              | —              | ✅ `make test-smoke`  |

---

## Best Practices

1. Run `go test ./...` before committing.
2. Use `MockClient` for command tests; do not make real API calls in integration tests.
3. Test both success and error paths.
4. Keep E2E tests sequential.
5. Update E2E coverage when adding new commands or flags.

---

## Quick Reference

```bash
# All unit + integration tests
go test ./...

# Command tests only
go test ./internal/cmd/...

# API tests only
go test ./internal/api/...

# Smoke E2E
make test-smoke

# Full E2E
make test-e2e
```
