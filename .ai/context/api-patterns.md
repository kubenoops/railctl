# API and CLI Patterns

## Command Patterns

- Use Cobra `RunE`.
- Provide `Use`, `Short`, `Long`, and `Example` for user-facing commands.
- Return errors instead of exiting from command handlers.
- Resolve flags and environment variables in the existing order:
  flag value, then `RAILCTL_*` environment variable, then default.
- Keep validation errors actionable by naming the invalid value and the accepted
  options when possible.

## Output Patterns

- Use `cmdutil.PrintResult` for command output.
- Use `internal/output` for table, wide, JSON, and YAML rendering.
- Avoid freeform printing for commands that return structured resources.

## Resolver Patterns

`internal/resolver` follows this contract:

1. exact match
2. case-insensitive substring match
3. zero matches returns `ErrNotFound`
4. multiple matches returns `ErrAmbiguous`

Preserve this behavior for new resource lookups.

## API and Security Patterns

- Wrap API errors with useful operation context and `%w`.
- Keep token resolution centralized through the existing token path.
- Never expose token or secret values in logs, examples, tests, or error output.
- Use `api.IsSensitiveKey` and `api.MaskValue` when showing variable values that
  may be sensitive.

## Documentation Pattern

When changing command behavior, update `docs/railctl-skill.md` first, regenerate
the embedded copy with `make gen`, and then update README or focused docs where
the changed behavior is user-facing.
