# railctl AI Agent Instructions

Use these instructions as the canonical project context for AI coding agents.
Tool-specific files such as `CLAUDE.md`, `.cursorrules`, `.windsurfrules`, and
`.github/copilot-instructions.md` point here so every agent reads the same rules.

## Project Snapshot

`railctl` is a Go CLI for managing Railway.app infrastructure with kubectl-style
commands. The command surface is implemented with Cobra under `internal/cmd`,
uses `internal/api` for Railway GraphQL calls, and renders structured output via
the shared `internal/output` package.

The embedded usage guide is a first-class artifact for both humans and agents:

- Source: `docs/railctl-skill.md`
- Embedded copy: `internal/skill/railctl-skill.md`
- Regenerate/check: `make gen` and `make gen-check`

## Workflows

Read the focused workflow docs before changing behavior:

- Build and generated assets: `.ai/workflows/build.md`
- Tests and validation: `.ai/workflows/test.md`
- Release and CI expectations: `.ai/workflows/release.md`

Read the context docs before touching architecture-sensitive code:

- Architecture map: `.ai/context/architecture.md`
- API and CLI patterns: `.ai/context/api-patterns.md`

## Non-Negotiable Conventions

- Keep PRs focused on one feature, fix, or documentation update.
- Format Go with `gofmt` and keep `golangci-lint run` clean.
- Use `cmdutil.ResolveContext` and `cmdutil.PrintResult` for command scaffolding
  instead of reimplementing context resolution or output behavior.
- Render JSON, YAML, table, and wide output through `internal/output`.
- Resolve names through `internal/resolver` and preserve its exact-match,
  substring, not-found, and ambiguous-name contract.
- Wrap returned errors with context using `%w`.
- Do not log, print, commit, or echo Railway tokens or secret values. Use the
  existing masking utilities when displaying potentially sensitive variables.
- For behavior changes, update the embedded skill doc and any affected README or
  `docs/` page in the same PR.

## Validation Baseline

For documentation-only changes, run:

```bash
git diff --check
```

For Go behavior changes, run at least:

```bash
make gen-check
make test
golangci-lint run
```

E2E tests require live Railway credentials and must not be run unless the caller
explicitly provides the needed tokens. See `tests/e2e/README.md` for token scope
requirements and group-specific commands.

## PR Checklist for Agents

- Confirm the issue is still open, unassigned, and not already covered by an open
  pull request before starting work.
- Inspect `CONTRIBUTING.md`, `.github/PULL_REQUEST_TEMPLATE.md`, and `SKILL.md`
  before editing code.
- Keep generated files in sync when touching generated sources.
- Do not commit local binaries, coverage output, credentials, or `.env` files.
- Fill out the PR template with the checks actually run and note when E2E tests
  were skipped because credentials were unavailable.
