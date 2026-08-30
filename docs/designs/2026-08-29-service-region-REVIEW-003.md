# SPEC Review 003 — service region placement

**Reviews:** `docs/designs/2026-08-29-service-region.md` (Status: Reviewable, iteration 4)
**Predecessor:** `docs/designs/2026-08-29-service-region-REVIEW-002.md` (verdict: Needs material revision)
**Reviewer role:** IEEE 29148 specification reviewer (third pass — confirmation only)
**Date:** 2026-08-29
**Scope:** Confirm the six REVIEW-002 findings are genuinely closed by iteration 4, and that
the iteration-4 edits introduced no new contradiction. REVIEW-001 items (confirmed closed in
REVIEW-002) and settled product decisions are **not** re-litigated.

**Feasibility cross-check:** `internal/diff/diff.go` (`compareDeployConfig`, lines ~505–543),
`internal/cmd/apply.go` (`fetchLiveState`, lines ~283–320). Both confirm the pre-iteration-4
state (`ls.Deploy.Replicas = svc.Replicas`; `compareDeployConfig` compares `d.Replicas != l.Replicas`)
and that the specified fix is realizable: `svc` is a `ServiceDetail` in scope at the copy site,
and the volume list is already fetched at the pre-flight location.

---

## Closure table

| Finding | Closing DEC/REQ in iteration 4 | Status |
|---|---|---|
| **NEW-CRIT-1** (diff derives replicas from flat `numReplicas`) | DEC-024 + REQ-APL-010 (+ REQ-APL-005 verification) | **CLOSED** |
| **NEW-MAJ-1** (collapse-guard data source at pre-flight) | REQ-APL-004 + REQ-APL-008 + Appendix E | **CLOSED** |
| **NEW-MAJ-2** (undefined N on forced collapse) | DEC-025 + REQ-CMD-002/CFG-005 + REQ-CMD-009 verification | **CLOSED** |
| **NEW-MIN-1** (REQ-API-003 layering) | REQ-API-003 (reworded caller-enforced) | **CLOSED** |
| **NEW-MIN-2** (delete diff multi-region scope) | REQ-APL-009 (scope note) | **CLOSED** |
| **NEW-MIN-3** (`--force` × volume) | REQ-VOL-001 + verification | **CLOSED** |

**Score: 6 / 6 CLOSED.**

---

## Per-finding confirmation

**NEW-CRIT-1 — CLOSED.** DEC-024 threads the live `multiRegionConfig` map through the
diff/compare/pre-flight path. REQ-APL-010 mandates `LiveService`/`LiveDeployConfig` carry
`MultiRegion map[string]int` and that the effective replica count for a region-placed live
service used by `compareDeployConfig` be `MultiRegion[Region]`, not flat `numReplicas`. Appendix E
adds `diff.LiveDeployConfig.MultiRegion` populated in `fetchLiveState` (`ls.Deploy.MultiRegion =
svc.MultiRegion`). Idempotency is now **achievable and verifiable**: REQ-APL-010's row asserts
`compareDeployConfig` on live `{X:3}` vs desired `{region:X, replicas:3}` → no `deploy.replicas`
diff; vs `replicas:5` → one diff, and REQ-APL-005's row now asserts an empty ChangeSet on
re-apply. Feasible against the confirmed code.

**NEW-MAJ-1 — CLOSED.** REQ-APL-004 now names the source explicitly: `fetchLiveState` "now
carries `LiveDeployConfig.MultiRegion`, REQ-APL-010." REQ-APL-008 states the collapse guard reads
`LiveDeployConfig.MultiRegion` per REQ-APL-010 inside the REQ-APL-004 pre-flight. Appendix E
confirms the field is used "by the apply pre-flight/collapse guard (REQ-APL-004/008) to read map
cardinality before any mutation." The map is now present at the exact pre-flight site — the
REVIEW-002 hole (data absent where the guarantee is asserted) is filled.

**NEW-MAJ-2 — CLOSED.** DEC-025 pins N for a forced collapse: the target region's live count if
the target is among the live regions, else 1 — never the sum or another region's count. REQ-CMD-002
folds this in (target-region count, else flat count if default-placed, else 1) and REQ-CFG-005
applies the same rule to the manifest path. Verifiable: REQ-CMD-009's row asserts `--force` (no
`--replicas`) on live `{A:{2},B:{5}}`→A writes `{A:{2}}`, and `--force --region C` (C not live)
writes `{C:{1}}`. N is pinned and tested on both axes.

**NEW-MIN-1 — CLOSED.** REQ-API-003 is reworded: the flat path is used exactly when the caller
passes `region == nil`; choosing flat vs map based on live placement is "**caller-enforced**
(REQ-CMD-008/REQ-APL-007 …); the write method itself branches only on the `region` parameter and
MUST NOT be assumed to inspect live state." Layering is now unambiguous; REQ-API-004 remains the
sole method-level contract.

**NEW-MIN-2 — CLOSED.** REQ-APL-009 documents the single-region scope: it renders the derived
single region, and "a pruned service with >1 live region shows no region line (documented
single-region scope, acceptable because delete removes the whole service regardless)." The
verification row matches.

**NEW-MIN-3 — CLOSED.** REQ-VOL-001 states `--force` "which only overrides the collapse guard,
DEC-015 … MUST NOT bypass this volume block: a service that is both multi-region and volume-bound
is still refused." The verification row exercises exactly `--force --region X` on a
multi-region+volume-bound service → still refused.

---

## New-contradiction check (iteration-4 edits only)

- **DEC-024 vs DEC-022 / DEC-016 / DEC-019:** the map-based compare (DEC-024) and the imperative
  no-op reading `ServiceDetail.MultiRegion` (DEC-022) are two readers of the same live map — no
  conflict. Write-path DECs are untouched and remain coherent.
- **DEC-025 vs DEC-016:** DEC-025 is a strict extension of DEC-016's "preserve what you can" to the
  collapse case (target region's count, else 1). No contradiction; REQ-CMD-002 unifies them without
  ambiguity.
- **REQ-APL-004 / REQ-APL-008 vs REQ-APL-010 vs Appendix E:** consistent chain — REQ-API-005
  surfaces `ServiceDetail.MultiRegion`; `fetchLiveState` copies it into
  `LiveDeployConfig.MultiRegion`; pre-flight/compare read it. Matches feasible code.
- **REQ-APL-005 / REQ-APL-010 verification rows:** self-consistent with Appendix E's
  `MultiRegion map[string]int` (region→numReplicas) shape.

No new contradiction introduced by iteration 4.

---

## Residual findings

**RES-1 (Minor, editorial / scope) — REQ-APL-010 leaves the replica-compare undefined for a
live service with >1 region.** REQ-APL-010 defines the effective compare count as
`MultiRegion[Region]`, but per Appendix E `Region` is `""` when `len(MultiRegion) > 1`. A literal
implementation (`effective = l.MultiRegion[l.Region]`) then yields `MultiRegion[""] == 0` for a
multi-region live service, so a manifest that sets `deploy.replicas` **without** `deploy.region`
against such a service would show a perpetual `deploy.replicas: 0 → N` drift — the same shape as
NEW-CRIT-1, but for the >1-region case. This is narrow: multi-region fan-out is out of scope
(DEC-002), the manifest cannot express >1 region, and a single-region manifest that names the
region is caught by the collapse guard (REQ-APL-008) before compare matters. It is not worse than
the pre-iteration-4 behavior (flat `numReplicas`, also stale). Recommend one clarifying clause in
REQ-APL-010 — e.g. "for a live service with `len(MultiRegion) > 1`, `compareDeployConfig` MUST NOT
emit a `deploy.replicas` diff from an omitted-region manifest" (or note the case as out of scope
per DEC-002). Non-blocking.

---

## Verdict

**Ready.**

All six REVIEW-002 findings (NEW-CRIT-1, NEW-MAJ-1/2, NEW-MIN-1/2/3) are genuinely closed by
iteration 4 with concrete DEC/REQ text and matching verification rows, and the fixes are feasible
against the live `diff.go` / `cmd/apply.go`. The iteration-4 edits introduce no new contradiction;
DEC-024 and DEC-025 sit cleanly alongside the existing write-path decisions. The single residual
(RES-1) is a narrow, out-of-scope (DEC-002) editorial clarification on REQ-APL-010's edge behavior
for pre-existing multi-region services — worth a one-clause note but not a blocker to
implementation. The design is Ready; folding RES-1 in during implementation is sufficient.
