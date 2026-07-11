# Build Workflow

Use the repository `Makefile` as the source of truth for local build commands.

## Common Commands

```bash
make build
make install
make clean
```

`make build` compiles `./cmd/railctl` into the root `railctl` binary and injects
version metadata from git through linker flags.

## Generated Assets

The embedded `railctl skill` content is generated from `docs/railctl-skill.md`
into `internal/skill/railctl-skill.md`.

```bash
make gen
make gen-check
```

Run `make gen` after changing `docs/railctl-skill.md`. Run `make gen-check`
before opening a PR that touches the embedded skill or behavior-bearing code.

## Build Artifacts

Do not commit local binaries or distribution output:

- `railctl`
- `dist/`
- `coverage.out`
