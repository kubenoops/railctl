# Code Review: service region placement (`feat/service-region`)

**Reviewer:** independent senior Go reviewer (skeptical, code-focused)
**Date:** 2026-08-29
**Base:** `e89a3ad` (merge-base with `main`) → `HEAD` (`f58c7ab`)
**SPEC:** `docs/designs/2026-08-29-service-region.md` (Ready, iteration 5)
**Scope reviewed:** full diff `e89a3ad..HEAD` — `internal/{api,cmd,config,diff,apply,types}`, tests
(`internal/**/*_test.go`, `tests/e2e/project/`), docs.

## Summary

This is a careful, high-quality implementation that closely tracks the SPEC — including the
subtle behaviours the three prior SPEC reviews were written to protect (region write
precedence, no-op-before-volume, per-region replica comparison for idempotency, atomic
apply pre-flight, `--force`-guarded collapse, and volume-block-not-bypassable-by-force). I
traced every load-bearing path against its REQ and found **no correctness defects** in the
mutation/diff logic. `go build ./...` and `go vet ./internal/...` are clean, `gofmt -l`
reports nothing, and `internal/skill/railctl-skill.md` is byte-identical to
`docs/railctl-skill.md` (so `make gen-check` passes).

The findings below are one repo-convention gap (README), a few real test-coverage gaps
against the verification table, and one cosmetic diff-rendering nit. None touch the
correctness of a write.

**Verdict: Approve with nits** — contingent on M1 (the README rows required by
`CONTRIBUTING.md §4`), which is a mechanical doc fix, not a design change.

---

## A/B. Spec conformance & correctness (per-REQ)

Traced by reading the implementation and the captured-write / diff tests. `file:line` is the
primary implementation site.

| REQ | Satisfied | Where / note |
|---|---|---|
| API-001 | ✅ | `internal/api/regions.go:33` `ListRegions` executes `regions` query, maps name/country/location/`railwayMetal`→Metal. Nil `railwayMetal` → false. |
| API-002 | ✅ | `internal/api/services.go:474-483` — `region != nil` emits `multiRegionConfig{region:{numReplicas:N}}`, **no** top-level `numReplicas`. Body-capture test `regions_test.go:72`. |
| API-003 | ✅ | `services.go:484-485` flat path only when `region == nil`; caller-enforced. Test `regions_test.go:104`. |
| API-004 | ✅ | Signature gained `region *string` on iface (`interface.go:31`), mock field+dispatch (`mock.go:23,185`), and all 4 call sites (create/update cmd, apply create/update). Build confirms migration. |
| API-005 | ✅ | `services.go:257-268` parses full `multiRegionConfig`→`MultiRegion`, derives `Region` only when `len==1`. Query selections added at `services.go:30,73`. Test `regions_test.go:116` (single/multi/none). |
| CMD-001 | ✅ | `--region` on create (`create_service.go:95`) and update (`update_service.go:148`). |
| CMD-002 | ✅ | `effectiveRegionReplicas` (`region.go:` ~93) implements the full precedence ladder; create defaults 1 via API layer. Unit `region_cmd_test.go:29`; integration `:223`. |
| CMD-003 | ✅ | `resolveRegion` (`region.go:15`) case-insensitive match, error lists sorted valid names. Unit `region_cmd_test.go:13`; e2e unknown-region fail. |
| CMD-004 | ✅ | Create resolves/validates region **before** `CreateService` (`create_service.go:157-168`). Fail-fast test `region_cmd_test.go:182`. |
| CMD-005 | ✅ | Create falls back to `RAILCTL_REGION` (`create_service.go:159`), flag wins; update never reads env (`resolveUpdateRegion` uses only the flag). |
| CMD-006 | ✅ | No-op = region AND effective replicas match, checked **before** volume block (`update_service.go` `resolveUpdateRegion`, `isRegionNoop` at `region.go`). Default-placed (empty map) always a change. See m1 for a test gap on the "replicas-differ ⇒ not no-op" arm. |
| CMD-007 | ✅ | `get_regions.go` table (NAME,LOCATION) / wide (+COUNTRY,METAL) / json / yaml via `output.Printer`; table via `tabwriter` (an established repo pattern). Tests `get_regions_test.go`. |
| CMD-008 | ✅ | Bare `--replicas` on a region-placed service routes through the map for the existing region; `>1` region errors (`resolveUpdateRegion` Case 2). Integration `region_cmd_test.go:319`. |
| CMD-009 | ✅ | `checkRegionCollapse` (`region.go`) refuses `>1`→1 without `--force`, names dropped regions; `--force` flag added. Integration `region_cmd_test.go:293` incl. forced-collapse writes target-live count. |
| VOL-001 | ✅ | `ensureNoVolumeForRegionChange` (`region.go`) hard error with detach→recreate guidance; runs **after** no-op, **before** collapse; `--force` never reaches it. Integration `region_cmd_test.go:274`, e2e volume block. See m2 (force+volume+multi-region not explicitly asserted). |
| VOL-002 | ✅ | Create never calls `ListVolumes` (create path has no volume check). |
| CFG-001 | ✅ | `config.DeployConfig.Region` (`config.go:47`). Test `config/region_test.go:10`. |
| CFG-002 | ✅ | No API call in `Validate`; unknown name passes. Test `region_test.go:33`. |
| CFG-003 | ✅ | `deploy.region` not `$env()`-expanded — `ExpandServiceConfigEnvRefs` leaves it literal. Test `region_test.go:45`. |
| CFG-004 | ✅ | Legacy config → `Region == ""`. Test `region_test.go:61`. |
| CFG-005 | ✅ | Omitted replicas → live-if-known else 1, never 0, via `effectiveApplyReplicas` (`apply.go`). Unit `apply/region_test.go:13`. |
| APL-001 | ✅ | `applyCreate` sets placement (`apply.go:150`), `applyUpdate` changes on drift. e2e `apply_diff_test.go:382`. |
| APL-002 | ✅ | Omitted `deploy.region` never diffs/writes: `compareDeployConfig` guards `d.Region != ""`. Unit `diff/region_test.go:22`. |
| APL-003 | ✅ | Region drift → `deploy.region` field diff via path-agnostic renderer (`diff.go:562`). Unit `diff/region_test.go:12`. |
| APL-004 | ✅ | `preflightRegionChanges` (`cmd/apply.go:296`) runs after `fetchLiveState`, before `apply.Apply`; scans every change with `deploy.region`; volume-bound → whole apply fails pre-mutation. `ls.Volumes` populated for **all** services (`cmd/apply.go:395`), not just declared ones. Unit `apply_region_test.go:19`. |
| APL-005 | ✅ | Region change deploys; re-apply is a no-op. e2e idempotency step `apply_diff_test.go:417-427`. |
| APL-006 | ✅ (see m3) | apply/diff never read `RAILCTL_REGION`. No test sets the env during apply, but it's structurally guaranteed. |
| APL-007 | ✅ | `resolveApplyRegion` (`apply.go`) keys the map path off `cfg.Deploy.Region` (not the drifted field set), resolves the `(region,replicas)` tuple from desired+live. Unit `apply/region_test.go:37`, `:94`. |
| APL-008 | ✅ | Same collapse guard in the pre-flight, reads `ls.Deploy.MultiRegion`, gated by `apply --force`. Multi-region live always drifts `deploy.region` (derived `Region==""`), so the field is present for the pre-flight to catch. Unit `apply_region_test.go:36`. |
| APL-009 | ✅ | `buildDeleteChange` (`diff.go:337`) emits `deploy.region` for single-region; `>1`-region shows none. Unit `diff/region_test.go:81`. |
| APL-010 | ✅ | `LiveDeployConfig.MultiRegion` carried through `fetchLiveState`; `effectiveLiveReplicas`/`compareDeployConfig` (`diff.go:527,560`) compare per-region count; RES-1 multi-region no-replica-drift honoured. Unit `diff/region_test.go:31,51,59`; e2e idempotency. |
| DOC-001 | ✅ | `docs/declarative-config.md` gains `deploy.region` row + example. |
| DOC-002 | ✅ | `docs/railctl-skill.md` + embedded copy updated & in sync; `gen-check` clean. |
| DOC-003 | ✅ | `docs/region-placement.md` added. |

**RAILCTL_REGION is create-only:** confirmed — only `create_service.go:159` reads it;
`resolveUpdateRegion` and the apply/diff paths never do. ✅

---

## Findings

### M1 — README not updated for `--region` and `RAILCTL_REGION` (repo-convention, Major)

**Where:** `README.md` — deploy-config flags table (`README:230-238`) and environment-variable
reference table (`README:463-473`). Neither lists `--region` nor `RAILCTL_REGION`.

**Problem:** `CONTRIBUTING.md §4` (and `.ai/instructions.md`) state, with "must": *"if you add or
change a CLI flag or `RAILCTL_*` environment variable you must update `README.md` (the
environment-variable table and usage examples)."* This PR adds both a new flag (`--region`,
on `create service` and `update service`) and a new `RAILCTL_REGION` env var, but README is
untouched (`git diff --stat` shows no `README.md`). The SPEC's own DOC REQs target
declarative-config / skill / region-placement docs, not README — so this is a **convention**
gap, not a spec-conformance failure.

**Note on CI:** the `docs-guard` workflow does *not* actually require README specifically — it
passes when **any** `.md`/`docs/` file changes (`.github/workflows/docs-guard.yml:53`), and
several `docs/` files did change. So CI is green; the seatbelt does not catch this omission.
A maintainer reviewing per CONTRIBUTING would still request it.

**Fix:** add a `--region` row to the deploy-config flags table (alongside `--replicas`), a
`RAILCTL_REGION` row to the env-var table, and one usage example. Purely mechanical.

### m1 — No integration test for "region equal, replicas differ ⇒ not a no-op" (Minor, test coverage)

**Where:** verification table row REQ-CMD-006 requires *"region-equal-but-replicas-differ →
NOT a no-op (write happens)"* at the **Integration** tier. Only the unit `TestIsRegionNoop`
(`region_cmd_test.go:57`) asserts `isRegionNoop(..,3) == false`; no command-level test drives
`update service --region X --replicas <different>` against live `{X:n}` and asserts the write
actually fires with the new count.

**Scenario not asserted:** live `{us-west1:2}`, `update service --region us-west1 --replicas 5`
should write `{us-west1:5}` and deploy. Logic is correct (explicit `--replicas` wins in
`effectiveRegionReplicas`, `isRegionNoop` returns false), but the integration guarantee is
untested.

**Fix:** add a case to `region_cmd_test.go` mirroring `TestRunUpdateService_RegionNoop` but
with a differing `--replicas`, asserting `cap.called` and `cap.replicas == 5`.

### m2 — force + volume + multi-region "still refused by volume block" not asserted (Minor, test coverage)

**Where:** REQ-VOL-001 second clause and its verification row: *"`--force --region X` on a
service that is both multi-region and volume-bound → still refused by the volume block."*
`TestRunUpdateService_RegionBlockedByVolume` (`region_cmd_test.go:274`) uses default placement
and no `--force`; `TestPreflightRegionChanges_VolumeBlock` (`apply_region_test.go:19`) does not
combine a `>1`-region live map + volume + `force=true`.

**Why it still holds:** in both paths the volume check precedes the collapse guard and never
consults `force` (`resolveUpdateRegion` order; `preflightRegionChanges` returns the volume
error before the collapse branch). So the behaviour is structurally correct — just not pinned
by a test that would catch a future reordering.

**Fix:** one integration case (`--force --region X`, live `{A:2,B:5}`, volume attached →
error contains `region-bound`, `cap.called == false`) and one `preflightRegionChanges` case
with `force=true` + volume + 2-region map.

### m3 — REQ-APL-006 not exercised with `RAILCTL_REGION` set (Minor, test coverage)

**Where:** verification row REQ-APL-006 (Unit): *"With `RAILCTL_REGION` set and region omitted
in manifest → no region diff/write."* No test sets the env during an apply/diff. The
guarantee is structural (apply/diff never read the env), so this is low priority; a
`t.Setenv("RAILCTL_REGION", …)` around an omitted-region apply would close it explicitly.

### n1 — Collapse to a not-live region renders `deploy.replicas 0 → N` (Nit, cosmetic)

**Where:** `internal/diff/diff.go:527` `effectiveLiveReplicas`, `default` branch returns
`l.MultiRegion[desiredRegion]` which is `0` when the target region isn't among the live set.

**Scenario:** live `{a:2,b:5}`, manifest `deploy.region: c`, `deploy.replicas: 3`. The diff
prints `deploy.replicas  0 → 3` — the "0" is not a real current value (region `c` has no live
count). Harmless: the actual write uses `effectiveApplyReplicas` (→3) correctly, and the
change is refused by the pre-flight unless `--force`. Only the rendered "current" is
misleading.

**Fix (optional):** when the desired region is a collapse target not present in the live map,
suppress the `deploy.replicas` current value (render `—` / omit the line) since the region
line already conveys the move.

---

## C. Convention adherence (other than M1)

- **Error wrapping** — consistent `fmt.Errorf("…: %w", err)` across `regions.go`, `region.go`,
  `get_regions.go`, apply pre-flight. ✅
- **Hand-written MockClient** — new `ListRegionsFunc` field + method and the widened
  `UpdateServiceInstanceConfigFunc` follow the existing nil-guard/dispatch pattern
  (`mock.go:47,185,344`). ✅
- **`output.Printer`** — `get regions` uses `output.NewPrinter`/`IsStructured`/`PrintJSON`/
  `PrintYAML`; the `tabwriter` table matches `get_replicas.go`/`get_deployments.go`/
  `logs_service.go`. ✅ (not a hand-rolled JSON/YAML path)
- **`go:generate` skill embedding** — `docs/railctl-skill.md` and `internal/skill/…` are in
  sync; `gen-check` clean. ✅
- **gofmt / vet / build** — all clean. ✅
- **CLI conventions** — every new command sets `Use/Short/Long/Example`, uses `RunE`; flag →
  env → default order honoured on create. ✅
- **Layering** — API client stays decoupled from `cmd`; region resolution lives in
  `internal/cmd/region.go` helpers, apply logic in `internal/apply`. ✅

---

## D. Test-coverage adequacy

Strong overall — REQs are backed at the tiers `docs/testing-architecture.md` prescribes:

- **Unit (Tier 1):** API body-capture for the multiRegionConfig write shape and flat path
  (`api/regions_test.go`), `toServiceDetail` parse; diff compare/idempotency/RES-1/create+delete
  fields (`diff/region_test.go`); replica-resolution ladders (`cmd/region_cmd_test.go`,
  `apply/region_test.go`); config load/validate/expand/legacy (`config/region_test.go`).
- **Integration (Tier 2):** create/update region flows, no-op, volume block, collapse+force,
  bare-replicas routing (`cmd/region_cmd_test.go`); apply create/update writes
  (`apply/region_test.go`); pre-flight volume/collapse (`cmd/apply_region_test.go`).
- **E2E (Tier 3):** `get regions` in 4 formats, create/move/no-op, volume block
  (`tests/e2e/project/regions_test.go`), and declarative create→idempotent-reapply→region-change
  (`tests/e2e/project/apply_diff_test.go:382`) — this last one directly exercises the
  perpetual-drift class of bug the SPEC reviews were about.

Gaps are the three test items above (m1–m3); all are "assert an already-correct guarantee",
none reveal a logic defect. No REQ is entirely without a test.

---

## Verdict

**Approve with nits.**

The implementation is correct and faithful to the SPEC on every load-bearing behaviour;
build/vet/fmt/gen are clean. Before merge, add the README rows for `--region` and
`RAILCTL_REGION` (M1 — required by `CONTRIBUTING.md §4`; note CI won't flag it). The test gaps
(m1–m3) and the cosmetic diff nit (n1) are worth closing but are not blockers.

---

## Resolution (post-review, 2026-08-29)

All findings closed:

- **M1** — README updated: `--region` row in the deploy-config flags table, `RAILCTL_REGION`
  in the env-var table, `get regions` + create/move usage examples, and an update
  `--force`/volume note.
- **m1** — `TestRunUpdateService_RegionEqualReplicasDifferWrites` (region equal, replicas
  differ ⇒ write fires with the new count and deploys).
- **m2** — `TestRunUpdateService_ForceDoesNotBypassVolume` + `TestPreflightRegionChanges_ForceVolumeMultiRegion`
  (`--force` + volume + multi-region still refused by the volume block).
- **m3** — `TestApply_IgnoresRailctlRegionEnv` (`t.Setenv` around an omitted-region apply ⇒ no
  region write).
- **n1** — `effectiveLiveReplicas` returns not-comparable when collapsing to a region absent
  from the live map (no misleading `deploy.replicas 0 → N`); case added to `TestEffectiveLiveReplicas`.

build / vet (incl. `-tags e2e`) / gofmt / full unit suite / `make gen-check` all clean.
