## What

<!-- Brief description of what this PR does -->

## Why

<!-- Why is this change needed? Link to issue if applicable -->

Closes #

## How

<!-- How does this work? Key implementation details -->

## Testing

- [ ] Unit tests pass (`make test`) and lint passes (`golangci-lint run`)
- [ ] Tests cover the meaningful scenarios (incl. not-found / ambiguous / empty where relevant)
- [ ] E2E tests pass locally (if applicable)
- [ ] Tested manually with the `railctl` binary

## Checklist

- [ ] Code follows existing patterns — reused `cmdutil` / `output` / `resolver` instead of hand-rolling
- [ ] `--help` output updated for new/changed commands
- [ ] Documentation updated — README env-var table & examples updated for any new flag or `RAILCTL_*` env var (the `docs-guard` check enforces this)
- [ ] One focused change — one feature or fix per PR
- [ ] No secrets or credentials in the diff
