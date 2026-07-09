# CI/CD & Build Setup

This document describes the CI/CD pipeline, build system, and release workflow for `railctl`.

## Build System

### Makefile

All build operations are driven by the [`Makefile`](../Makefile) at the project root.

| Target | Description |
|--------|-------------|
| `make build` | Build the `railctl` binary for the current platform |
| `make build-all` | Cross-compile for all 4 release platforms |
| `make install` | Install to `$GOPATH/bin` |
| `make test` | Run all Go unit tests with verbose output |
| `make fmt` | Format Go source files |
| `make lint` | Run `golangci-lint` |
| `make clean` | Remove `railctl`, `dist/`, and `coverage.out` |
| `make help` | Show available targets and examples |

### Version Embedding

The build injects version metadata into the binary via Go **ldflags**. Three variables in `internal/cmd/root.go` are set at build time:

```go
var (
    version = "dev"     // git tag or "local-build"
    commit  = "unknown" // short commit hash
    date    = "unknown" // ISO-8601 UTC timestamp
)
```

The Makefile computes these from git automatically:

```makefile
VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT  ?= $(shell git rev-parse --short HEAD)
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
```

You can override the version at build time:

```bash
make build VERSION=v1.0.0
```

The `--version` flag output:

```
railctl version v1.0.0 (commit: abc1234, built: 2026-02-10T19:15:27Z)
```

### Cross-Compilation

`make build-all` produces 4 binaries in the `dist/` directory:

| Binary | OS | Architecture |
|--------|----|--------------|
| `dist/railctl-linux-amd64` | Linux | x86_64 |
| `dist/railctl-linux-arm64` | Linux | ARM64 |
| `dist/railctl-darwin-amd64` | macOS | Intel |
| `dist/railctl-darwin-arm64` | macOS | Apple Silicon |

---

## GitHub Actions Workflows

All workflow files live in `.github/workflows/`.

### 1. PR Tests (`pr.yml`)

**Purpose:** Validate every pull request before merge.

**Trigger:** Pull request opened or updated against `main`.

**What it does:**

1. Checks out the code
2. Sets up Go 1.22 with module caching
3. Runs `make build` to verify compilation
4. Runs `go test -v ./...` and captures output
5. Posts (or updates) a comment on the PR with:
   - A summary table showing Passed / Failed / Total counts
   - Full test output in a collapsible `<details>` block

**Permissions:** `contents: read`, `pull-requests: write` (for posting comments).

**Key behaviors:**
- The bot comment is **updated in-place** on subsequent pushes (no spam)
- Test output is truncated to 60KB if too long
- The step uses `continue-on-error: true` for the comment so a GitHub API failure doesn't mark the whole run as failed

```
## Test Results ✅ All tests passed!

| Metric | Count |
|--------|-------|
| Total  | 42    |
| Passed | 42    |
| Failed | 0     |
```

---

### 1b. Docs Guard (`docs-guard.yml`)

Two deterministic completeness checks on every PR:
- **CLI surface → docs**: a PR adding a flag or `RAILCTL_*` env var must touch
  documentation (bypass label: `docs-not-needed`).
- **Behavior code → embedded skill**: a PR changing `internal/{cmd,api,apply,
  config,diff}` (tests excluded) must update `docs/railctl-skill.md` — the
  agent-facing source of truth printed by `railctl skill` (bypass label:
  `skill-not-affected`).

PR Tests additionally run `make gen-check` to guarantee the embedded copy
(`internal/skill/railctl-skill.md`) is regenerated and committed alongside the
source.

### 2. Release (`release.yml`)

**Purpose:** Automate the release lifecycle with a two-phase approach.

**Triggers:**
- `push` to `main` → creates a **pre-release**
- `release` event (type: `released`) → builds **binaries**

#### Phase 1: Auto Pre-Release (on push to main)

> Version computation uses the **semver-highest existing tag** (`git tag |
> sort -V`), not `git describe` — describe returns the nearest tag on the
> commit graph, which can resurrect an older lineage after a manual
> minor/major release (issue #64).

Every merge to `main` automatically:

1. Finds the latest git tag (or defaults to `v0.0.0`)
2. Increments the **patch** version (e.g., `v1.2.3` → `v1.2.4`)
3. Generates a changelog from commit messages since the last tag
4. Creates a GitHub **pre-release** with the new tag

This means every merge to main produces a versioned, tagged pre-release that's ready for promotion.

#### Phase 2: Binary Build (on release promotion)

When a pre-release is **promoted to a full release** (via the GitHub UI or the manual release workflow):

1. Sets up Go 1.22
2. Runs `make build-all` with the release tag as the version
3. Uploads the 4 platform binaries as release assets

**Permissions:** `contents: write`, `pull-requests: read`.

**Actions used:**
- `actions/checkout@v6`
- `actions/setup-go@v5`
- `softprops/action-gh-release@v2`

---

### 3. Manual Release (`manual-release.yml`)

**Purpose:** Create deliberate releases with version control (patch, minor, major).

**Trigger:** Manual `workflow_dispatch` with a required `release_type` input.

| Release Type | Behavior | Example |
|-------------|----------|---------|
| **patch** | Promotes the latest pre-release to a full release | `v1.2.4 (pre-release)` → `v1.2.4` |
| **minor** | Creates a new minor version tag and release | `v1.2.x` → `v1.3.0` |
| **major** | Creates a new major version tag and release | `v1.x.x` → `v2.0.0` |

#### Patch Flow

1. Finds the most recent pre-release
2. Removes "(pre-release)" from the title
3. Flips the `prerelease` flag to `false`
4. Sets it as the `latest` release
5. Builds and uploads binaries

#### Minor / Major Flow

1. Calculates the new version number
2. Finds the appropriate baseline tag for the changelog
3. Generates release notes with:
   - Merged pull requests (with titles from the GitHub API)
   - Direct commits
   - A full changelog comparison link
4. Creates a new git tag and pushes it
5. Creates the GitHub release
6. Builds and uploads binaries

**Permissions:** `contents: write`.

**Key detail:** Binary uploads use `--clobber` to replace any existing assets (safe for re-runs).

---

## Dependency Management

### Dependabot (`dependabot.yml`)

Dependabot is configured to keep two ecosystems up to date:

| Ecosystem | Schedule | Labels | Commit Prefix |
|-----------|----------|--------|---------------|
| Go modules (`gomod`) | Weekly (Mondays) | `dependencies`, `go` | `deps(go)` |
| GitHub Actions (`github-actions`) | Weekly (Mondays) | `dependencies`, `github-actions` | `deps(actions)` |

Both are limited to **5 open PRs** to avoid noise. Dependabot PRs trigger the PR Tests workflow automatically.

---

## Release Lifecycle

Here's how a typical change flows through the pipeline:

```
  Developer pushes to feature branch
         │
         ▼
  ┌──────────────┐
  │  PR Created  │──── PR Tests run ──── Bot posts results
  └──────┬───────┘
         │ Merge
         ▼
  ┌──────────────┐
  │  Push to     │──── Release workflow creates pre-release
  │  main        │     (auto-increments patch version)
  └──────┬───────┘
         │
         ▼
  ┌──────────────────────────────────────────────┐
  │  Option A: Manual Release (patch)            │
  │  → Promotes pre-release, builds binaries     │
  │                                              │
  │  Option B: Manual Release (minor/major)      │
  │  → New version, changelog, builds binaries   │
  │                                              │
  │  Option C: Promote in GitHub UI              │
  │  → Triggers build-release job                │
  └──────────────────────────────────────────────┘
         │
         ▼
  ┌──────────────┐
  │  Release     │──── 4 platform binaries available
  │  Published   │     for download
  └──────────────┘
```

---

## Security & Permissions

All workflows follow the **principle of least privilege**:

- **PR Tests** — Read-only code access, write only to PR comments
- **Release / Manual Release** — Write access to contents (required for creating tags and releases)
- **No external secrets** — All workflows use the automatic `GITHUB_TOKEN`
- **No third-party services** — Builds are self-contained, no uploads to external registries

---

## Local Development Workflow

```bash
# Build and test locally
make build          # Compile with version from git
make test           # Run all tests
make fmt            # Format code
make lint           # Lint (requires golangci-lint installed)

# Before committing
make clean          # Remove artifacts
make build          # Verify clean build
make test           # Verify tests pass
```

---

## Related Files

| File | Purpose |
|------|---------|
| [`Makefile`](../Makefile) | Build automation and targets |
| [`.github/workflows/pr.yml`](../.github/workflows/pr.yml) | PR test workflow |
| [`.github/workflows/release.yml`](../.github/workflows/release.yml) | Auto pre-release + binary build |
| [`.github/workflows/manual-release.yml`](../.github/workflows/manual-release.yml) | Manual release dispatch |
| [`.github/dependabot.yml`](../.github/dependabot.yml) | Dependency update config |
| [`internal/cmd/root.go`](../internal/cmd/root.go) | Version variables (ldflags targets) |
| [`.gitignore`](../.gitignore) | Build artifact exclusions |
