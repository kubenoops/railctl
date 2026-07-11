# Test Workflow

## Unit Tests

Run the Go unit test suite with:

```bash
make test
```

Tests live beside the code they cover. Prefer table-driven tests and include
meaningful edge cases such as empty input, not-found results, and ambiguous
matches when adding validation or resolver behavior.

## Lint

The canonical lint command is:

```bash
golangci-lint run
```

Let `gofmt` and `golangci-lint` own style feedback.

## E2E Tests

E2E tests hit the live Railway API and require explicit credentials. Do not run
them unless the caller has provided the required tokens.

```bash
make test-e2e-account
make test-e2e-workspace
make test-e2e-project
make test-e2e
```

See `tests/e2e/README.md` and `docs/testing-architecture.md` before changing E2E
coverage, token handling, or test harness behavior.
