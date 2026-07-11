# Architecture Context

The primary implementation path is:

```text
cmd/railctl main -> internal/cmd -> internal/cmdutil -> internal/api/resolver/output/types
```

## Key Areas

- `cmd/railctl/`: CLI entry point.
- `internal/cmd/`: Cobra commands and user-facing command behavior.
- `internal/cmdutil/`: shared command scaffolding, context resolution, guards,
  protection helpers, and result printing.
- `internal/api/`: Railway GraphQL API client, mock client, token/security
  helpers, and resource operations.
- `internal/resolver/`: name and ID resolution contract.
- `internal/output/`: JSON, YAML, table, and wide output formatting.
- `internal/apply/`, `internal/config/`, `internal/diff/`: declarative config,
  diff, and apply behavior.
- `tests/e2e/`: live Railway integration tests behind the `e2e` build tag.
- `examples/`: deployment examples that should stay aligned with user-facing
  CLI behavior.

## Layering Rules

- Commands should delegate shared lookup and output behavior to `cmdutil`.
- API code should stay decoupled from command presentation.
- Shared data structures belong in `internal/types`.
- New resolver behavior should extend `internal/resolver` rather than creating
  ad hoc matching in commands.
- Keep generated skill content synchronized when behavior changes.
