# Release and CI Workflow

Release and CI behavior is documented in `docs/ci-build-setup.md`.

## CI Expectations

Pull requests must keep the required checks green, including unit tests and the
documentation guard. If a change affects command behavior, flags, environment
variables, or Railway API semantics, update the corresponding docs in the same
PR.

## Documentation Guard

Behavior-bearing changes commonly need updates to:

- `docs/railctl-skill.md`
- `README.md`
- `docs/declarative-config.md`
- `docs/token-capability-matrix.md`
- `docs/testing-architecture.md`

Run `make gen` after changing the skill source and verify with `make gen-check`.

## Workflow Hygiene

GitHub Actions in this repository use pinned actions. Preserve that convention
when editing workflow files. Changes to CI, the `Makefile`, or E2E
infrastructure require extra care because they affect repository-wide validation.
