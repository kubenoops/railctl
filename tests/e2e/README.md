# railctl E2E Test Suite

Comprehensive end-to-end test suite that exercises all `railctl` commands and flags against the live Railway API.

## Overview

This test suite is built with `go test -tags e2e`. It includes:

- **Smoke test** (`TestSmoke`): A fast ~1min linear walk through the full lifecycle (project → env → service → variable → volume → deployments). Use `make test-smoke` for a quick sanity check.
- **Full suite**: Individual test files for each resource type with exhaustive flag/format coverage (~10min). Use `make test-e2e` to run everything.

## Test Coverage

### Commands Tested

| Category | Commands |
|----------|----------|
| **Project** | create, get (table/json/yaml/wide), describe (all formats), delete |
| **Environment** | create, get (all formats), describe (all formats), delete |
| **Service** | create (with all deploy config flags), get (all formats), describe (--show-values), update (image, deploy config, healthcheck), delete |
| **Variable** | set (single/multiple, --skip-deployment), get (all formats), delete |
| **Volume** | create (--mount-path), get (all formats), describe, update (--name, --attach, --detach), delete |
| **Deployment** | get (all formats, --limit), logs (--tail, --deployment, --follow), delete (rollback), update --set-active (reactivate) |

### Test Phases

1. **Pre-flight Checks** — Validate binary, token, and dependencies
2. **Project Operations** — Create, list, describe, error handling
3. **Environment Operations** — Create, list, describe, error handling
4. **Service Operations** — Create with flags, list, describe, substring matching
5. **Variable Operations** — Set, get, delete, skip-deployment flag
6. **Volume Operations** — Create, attach, describe, update, error handling
7. **Deployment Operations** — List, logs with various flags, error handling
8. **Update Service Operations** — Image updates, deploy config, healthcheck, combined updates
9. **Deployment Lifecycle** — Rollback (delete deployment), reactivate (update deployment --set-active)
10. **Error Handling & Edge Cases** — Invalid inputs, missing flags, env var overrides, substring resolution
11. **Cleanup** — Delete resources in reverse order

### Total Test Cases

**~140+ test assertions** covering:
- ✅ Success paths for all commands
- ✅ All output formats (table, wide, json, yaml)
- ✅ All command flags and combinations
- ✅ Error handling (missing flags, nonexistent resources, invalid inputs)
- ✅ Environment variable overrides
- ✅ Substring matching for project/environment/service resolution
- ✅ Deployment lifecycle (rollback, reactivate)

## Prerequisites

1. **Railway Account & Token**
   ```bash
   export RAILWAY_TOKEN="your-token-here"
   
   ```

2. **railctl Binary**
   ```bash
   cd /path/to/railway-cli
   go build -o railctl ./cmd/railctl
   ```

3. **Dependencies**
   - `jq` — JSON parsing (required)
   - `yq` — YAML parsing (optional, for YAML validation)

## Usage

### Basic Run

```bash
cd /path/to/railway-cli
./tests/e2e/run.sh
```

### Using direnv (recommended)

```bash
cd tests/e2e
# Copy .envrc.example to .envrc and fill in your tokens
./run.sh
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `RAILWAY_TOKEN` | Railway API token | **Required** |
| `RAILCTL` | Path to railctl binary | `../../railctl` |
| `E2E_PROJECT` | Custom project name | `e2e-test-<timestamp>-<random>` |
| `E2E_KEEP` | Skip cleanup (for debugging) | `0` |
| `E2E_VERBOSE` | Show command stdout | `0` |

### Examples

```bash
# Basic run
RAILWAY_TOKEN=xxx ./run.sh

# Verbose output (show command stdout)
E2E_VERBOSE=1 ./run.sh

# Keep resources for debugging
E2E_KEEP=1 ./run.sh

# Custom project name
E2E_PROJECT=my-test ./run.sh

# Use specific railctl binary
RAILCTL=/usr/local/bin/railctl ./run.sh
```

## Output Format

```
  ┌─────────────────────────────────────────────────┐
  │         railctl E2E Test Suite                   │
  └─────────────────────────────────────────────────┘

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Pre-flight Checks
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  [001] railctl binary exists                                    PASS
  [002] RAILWAY_TOKEN is set                                     PASS
  [003] jq is installed                                          PASS
  [004] railctl --version                                        PASS

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Phase 1: Project Operations
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  [005] create project                                           PASS
  [006] get projects (table)                                     PASS
  [007] get projects -o json                                     PASS
  ...

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Test Results
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  Passed:  140
  Failed:  0
  Skipped: 2
  Total:   142

  ✓ All tests passed!
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

## Test Philosophy

### Sequential Flow
Unlike traditional unit tests that isolate each test, this E2E suite uses a **single sequential flow** that:
- Creates resources once (project → environment → service)
- Tests all operations in dependency order
- Cleans up in reverse order

This approach:
- ✅ Mimics real-world usage patterns
- ✅ Reduces API calls and test time
- ✅ Avoids Railway rate limits (project/env creation has 30s cooldown)
- ✅ Tests the full lifecycle of resources

### Error Recovery
- **Emergency cleanup** on script failure/interrupt (Ctrl+C)
- **Graceful degradation** — Skips tests when resources aren't available (e.g., no deployment IDs yet)
- **Clear failure messages** — Shows exactly what failed and why

## Debugging

### Keep Resources After Run

```bash
E2E_KEEP=1 ./run.sh
```

This skips cleanup so you can inspect resources in Railway's dashboard.

**⚠️ Remember to manually delete the project when done!**

### Verbose Mode

```bash
E2E_VERBOSE=1 ./run.sh
```

Shows stdout/stderr for every command, helpful for diagnosing failures.

### Inspect Specific Test

```bash
# Run the script, it will fail/skip at the first error
# Then use railctl directly to debug:
./railctl get projects -p e2e-test-1707600000-abc123 -o json
```

## CI/CD Integration

This test suite is designed for CI/CD pipelines:

```yaml
# Example GitHub Actions workflow
- name: Run E2E Tests
  env:
    RAILWAY_TOKEN: ${{ secrets.RAILWAY_TOKEN }}
  run: |
    go build -o railctl ./cmd/railctl
    ./tests/e2e/run.sh
```

Exit code: **0** if all tests pass, **non-zero** otherwise.

## Known Limitations

1. **Rate Limits** — Railway limits project/environment creation to 1 per 30 seconds. This suite respects that but tests may take 5-10 minutes to complete.

2. **Deployment Timing** — Some tests wait for deployments to register. If Railway is slow, some deployment-related tests may be skipped (but won't fail).

3. **Logs Availability** — Build logs may not be immediately available. The suite accepts "no logs" as a valid state for recent deployments.

4. **Sequential Dependencies** — If an early test fails (e.g., project creation), subsequent tests will cascade fail or skip.

## Contributing

When adding new commands or flags to `railctl`:

1. Add test cases to the appropriate phase function
2. Test both success and error paths
3. Test all output formats if applicable
4. Add edge cases to `test_error_handling`
5. Update the "Test Coverage" section in this README

## See Also

- [Railway CLI Documentation](https://docs.railway.app/reference/cli)
- [Project README](../../README.md)
- [Example Deployments](../../examples/)
