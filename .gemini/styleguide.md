# railctl Code Review Style Guide

This guide tells Gemini Code Assist how to review pull requests for `railctl`, a
Go CLI for managing Railway infrastructure. The expectations below are derived
from the patterns **already established** in this codebase — reviews should hold
contributions to the same bar, not to aspirational rules the project does not yet
follow.

Keep feedback constructive and specific. Cite the file and line. Prefer a concrete
suggestion over a vague complaint. Do not nitpick style that `gofmt`/`golangci-lint`
already enforce.

---

## 1. Security — Highest Priority

This is a CLI that handles API tokens and environment variables. Security issues
are always worth a comment.

- **Never log or print secret values.** Variable values that could be sensitive
  must go through `api.MaskValue()`. Output of secret values is only allowed when
  the user explicitly passes `--show-values`.
- **Sensitive key detection** uses `api.IsSensitiveKey()` (regex on key names like
  `KEY`, `SECRET`, `PASSWORD`, `TOKEN`, `CREDENTIALS`, `AUTH`, `APIKEY`). If a PR
  adds handling of variable values, confirm it respects this masking path.
- **No hardcoded credentials, tokens, or passwords** — not in source, tests,
  examples, or fixtures. Test/example secrets must be obvious placeholders
  (e.g. `your-token`, empty strings in `.envrc.example`).
- **Token resolution** must follow the established chain: `--token` flag →
  `RAILWAY_TOKEN` env var. Never introduce alternate token sources that bypass
  `getToken()`.
- Flag a masking regression if `MaskValue` is changed in a way that leaks value
  length or suffix (it intentionally returns a fixed 14-char output).

## 2. CLI Command Conventions (Cobra)

Every command in `internal/cmd/` follows a consistent shape. New commands should
match it:

- Define `Use`, `Short`, `Long`, and `Example` on the `cobra.Command`. **A missing
  `Example` or `Long` on a user-facing command is worth a comment** — help-text
  completeness is an explicit project goal (AI-agent-friendly CLI).
- Use `RunE` (not `Run`) and return errors rather than calling `os.Exit`.
- The root command sets `SilenceUsage: true` — commands should return errors, not
  print usage on operational failures.
- Respect the flag → env → default resolution for `project` (`RAILCTL_PROJECT`),
  `environment` (`RAILCTL_ENVIRONMENT`), and `service` (`RAILCTL_SERVICE`).
- Support the standard `-o/--output` formats (`table`, `json`, `yaml`, `wide`)
  where the command lists or describes resources. Don't add a command that only
  prints freeform text if it returns structured data.

## 3. Error Handling

- Wrap errors with context using `fmt.Errorf("...: %w", err)`. The `%w` verb is
  required so callers can unwrap — flag plain `%v` wrapping on error returns.
- Validation errors should be **actionable**: state what was wrong and the valid
  options. The codebase does this well, e.g.
  `invalid restart policy '%s'. Must be one of: ON_FAILURE, ALWAYS, NEVER` and
  `--%s must be between 1 and 65535, got %d`. Hold new validation to that standard.
- Don't swallow errors. Every returned `error` should be checked.
- For resource-not-found and ambiguous-name conditions, use the resolver's
  sentinel errors rather than ad-hoc strings (see §9).

## 4. Testing

- Unit tests live next to the code (`*_test.go`) and cover **exhaustive scenarios,
  not just the happy path**. The established bar (see `internal/resolver/resolver_test.go`)
  is one test per scenario including edge cases: exact match, substring match,
  case-insensitivity, **not-found, ambiguous, and empty-input**. New
  resolution/validation logic without not-found/ambiguous/empty coverage is a gap.
- Table-driven style is preferred where scenarios share a shape.
- Pure formatting/transformation helpers (e.g. `projectsToTable`, the `output`
  package) must have direct unit tests asserting output shape.
- E2E tests live in `tests/e2e/` behind the `e2e` build tag and hit the live
  Railway API. Don't require new mandatory E2E coverage for small changes, but a
  new top-level command or lifecycle operation should have an E2E path.
- Tests must not depend on real credentials baked into the repo — they read
  `RAILWAY_TOKEN` from the environment.

## 5. Scope & Hygiene

- **One feature or fix per PR** (per CONTRIBUTING.md). If a PR mixes unrelated
  changes, suggest splitting it.
- Changes to behavior should update the relevant docs (see §7 for the
  behavior→doc map). Flag behavior changes that leave docs stale.
- Changes to CI/CD workflows or `tests/e2e/` infrastructure require maintainer
  attention — call these out explicitly in the review summary (they are
  CODEOWNER-protected).
- Keep the dependency footprint small. The project has only two direct
  dependencies (`spf13/cobra`, `yaml.v3`). A new third-party dependency for
  something the stdlib or an existing dep already does is worth questioning.
- GitHub Actions must stay **SHA-pinned** (full-length commit SHA with a version
  comment). Flag any workflow change that introduces a floating tag like
  `uses: actions/checkout@v4`.

## 6. Go Practices

- Code must be `gofmt`-clean and pass `golangci-lint` (`make lint`); don't comment
  on formatting/lint nits the tooling already handles.
- Use the project's module path `github.com/kubenoops/railctl/...` for internal
  imports — never a stale `NuevaNext` path.
- Prefer the standard library; follow idiomatic Go (early returns, no needless
  abstraction, UTF-8/rune-safe string handling as `MaskValue` does).
- Exported functions and types should have doc comments. The existing code
  documents non-obvious decisions well (see the `sensitivePattern` rationale) —
  match that level of explanation for tricky logic.

## 7. Completeness Pass — Run This on Every PR

This review must not only react to the lines in the diff — it must check for what
the PR **failed to change**. Diff-only review misses omissions (e.g. a flag added
but never documented). For **every PR that adds or changes a CLI flag, environment
variable, command, or user-facing behavior**, explicitly run through this checklist
and report each item in the review summary — even when the answer is "present":

- **The embedded skill (highest-priority doc)** — `docs/railctl-skill.md`
  (printed by `railctl skill`, mirrored into `internal/skill/` via `make gen`)
  is the single source of truth for agents operating railctl. **Any PR that
  changes behavior-bearing code (`internal/cmd`, `internal/api`,
  `internal/apply`, `internal/config`, `internal/diff`) must update it** —
  new/changed commands, flags, semantics, error messages, token-scope
  behavior, manifest fields all belong there. CI enforces this (docs-guard
  skill check + `make gen-check`); the reviewer must verify the skill edit is
  *accurate*, not merely present. A missing or stale skill update is a HIGH
  finding.
- **Documentation** — Was `README.md` updated? In particular, a new `RAILCTL_*`
  env var must be added to the env-var table, and a new flag should appear in the
  usage/examples. If the PR adds a surface but touches no `.md` file, call it out
  as a gap.
- **Behavior docs** — Behavior changes should update the matching file under
  `docs/`: token-scope semantics → `docs/token-capability-matrix.md`;
  manifest schema → `docs/declarative-config.md`; test approach
  → `docs/testing-architecture.md`.
- **Help text** — Does the new flag/command have a clear description, and is the
  command's `Long`/`Example` updated to mention the new capability?
- **Tests** — Is there a unit test covering the new logic, including not-found /
  ambiguous / empty edge cases where applicable (see §4)?
- **Examples** — If the change affects how the `examples/` deployments work, were
  they updated?
- **Consistency** — Does the new surface follow existing patterns (architecture
  in §8, resolution in §9, output via `output.Printer`, error wrapping)?

Report the checklist as a short ✅/❌ list in the summary. An unchecked **Documentation**
or **Tests** box on a behavior-changing PR should be raised as a MEDIUM (or higher)
comment, not left silent. The goal is that "you added a flag but didn't document or
test it" is never missed again.

## 8. Architecture & Layering

The codebase has a deliberate layering. New code should fit it rather than
re-implement existing primitives:

```
cmd  →  cmdutil  →  api / resolver / output  →  types
```

- **Command scaffolding** — Commands resolve project/environment/service context
  via `cmdutil.ResolveContext(client, cmdutil.ResolveOpts{…})` and render results
  via `cmdutil.PrintResult(…)`. This is used across ~15 commands. A new command
  that hand-rolls context resolution or output instead of using `cmdutil` is
  diverging from the established pattern — flag it.
- **Output** — Rendering goes through the `output` package (`output.NewPrinter`,
  `Printer.PrintJSON/PrintYAML/PrintTable`, `output.NewTable`). Do **not**
  hand-roll JSON/YAML/table formatting with ad-hoc `fmt.Println` when a structured
  result is being returned.
- **Types** — Shared structs live in `internal/types`. Don't redefine equivalent
  shapes locally.

## 9. Resolution Contract

Name-to-resource lookup is centralized in `internal/resolver` and follows a fixed
contract. New resolution logic should reuse it rather than re-implement matching:

- **Order:** exact match first → case-insensitive substring → decide by count.
- **Outcome:** 0 matches → `resolver.ErrNotFound{Resource, Name}`; exactly 1 →
  that match; N > 1 → `resolver.ErrAmbiguous{Resource, Name, Matches}`.
- **Error messages** are standardized by those sentinel types
  (`"<resource> '<name>' not found"`, `"ambiguous <resource> name '<name>'. Matches: …"`).
  Don't invent new not-found/ambiguous message formats.

A PR that adds a new resource type or resolves by a new field (as the workspace
feature did) should extend `resolver` and use these error types.

## 10. Long-Running Operations (`--await`)

Operations that wait for Railway to converge follow the `awaitDeployment` pattern
(`internal/cmd/await.go`):

- Poll until a **terminal status** (`SUCCESS`, `FAILED`, `CRASHED`, `REMOVED`,
  `SKIPPED`).
- Use **exponential backoff** (start 5s, cap 30s, factor 2.0).
- Print status transitions as they occur and respect an explicit timeout, returning
  a wrapped error on timeout.

New long-running/polling commands should follow this rather than inventing their
own loop or a fixed-interval busy-wait.

## 11. API Client Conventions

In `internal/api`, error surfaces are consistent — match them in new client methods:

- **Internal failures** are wrapped: `fmt.Errorf("failed to <do X>: %w", err)`.
- **API-level errors** use the `API error: %s` / `API %s (HTTP %d). %s` forms, and
  response bodies are truncated (`truncateBody(body, 200)`) so we never dump a huge
  payload into a user-facing error.

## 12. What This Project Deliberately Does NOT Do

To avoid false-positive review comments, do **not** ask for these — they run
counter to the codebase's actual conventions:

- **No `context.Context` threading.** No file in `internal/` uses `context.Context`;
  do not request adding context parameters to functions or API calls.
- **Lint nits belong to `golangci-lint`.** `make lint` runs `golangci-lint`; do not
  duplicate formatting/lint-level comments the linter already enforces.

---

### Review tone

- Lead with what matters: security > correctness > tests > docs > style.
- It's fine to approve a small, clean PR with a short positive note.
- Use severity sparingly: reserve **high/critical** for security leaks, broken
  error handling, or removal of test/masking safeguards.
