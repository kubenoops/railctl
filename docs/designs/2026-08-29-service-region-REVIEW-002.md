# SPEC Review 002 — service region placement

**Reviews:** `docs/designs/2026-08-29-service-region.md` (Status: Reviewable, iteration 3)
**Predecessor:** `docs/designs/2026-08-29-service-region-REVIEW-001.md` (verdict: Needs material revision)
**Reviewer role:** IEEE 29148 specification reviewer (second pass)
**Date:** 2026-08-29
**Method:** (A) Verify every REVIEW-001 finding is genuinely closed by iteration 3, citing the closing DEC/REQ. (B) Hunt for issues introduced by the iteration-3 edits, cross-checked against the live code (`internal/api/services.go`, `internal/apply/apply.go`, `internal/diff/diff.go`, `internal/cmd/apply.go`, `internal/cmd/update_service.go`, `internal/config/config.go`).

Stable IDs: `NEW-CRIT-n`, `NEW-MAJ-n`, `NEW-MIN-n` for new findings; closure table for REVIEW-001 items.

---

## Summary of verdict

Iteration 3 is a strong, disciplined sweep: the write-plumbing contract (DEC-017), the diff→apply tuple resolution (DEC-018), and the multi-region collapse guard (DEC-015) all land, and 18 of the 19 REVIEW-001 findings are genuinely closed. However, the edits added surface, and one new hole is a genuine blocker: **the declarative diff still derives a region-placed service's replica count from the flat `numReplicas` field, not from `MultiRegion[region]`, so `compareDeployConfig` will report a perpetual `deploy.replicas` drift and the promised idempotency/no-op (REQ-APL-005, DEC-022) fails for exactly the services this feature creates.** REVIEW-001 Minor-2 was therefore only *partially* closed. Two new Major items (apply-side collapse-guard data source; undefined N for a forced collapse) and three Minor items round out the pass. **Verdict: Needs material revision** (one focused, well-bounded revision — the design is otherwise sound).

---

## Part A — Closure of REVIEW-001 findings

| REVIEW-001 ID | Closing DEC/REQ in iteration 3 | Status | Notes |
|---|---|---|---|
| **Critical-1** (write-plumbing contract) | DEC-017, REQ-API-004 | **CLOSED** | `UpdateServiceInstanceConfig` gains `region *string`; interface, `MockClient`, and all four call sites named. Signature branch (region!=nil ⇒ map path) is explicit. |
| **Critical-2** (entanglement through diff→apply) | DEC-018, REQ-APL-007 | **CLOSED** (write side) | apply resolves the full `(region, numReplicas)` tuple from `configMap[name].Deploy` + live state and forces the map path even for a one-field ChangeSet. See **NEW-CRIT-1**: the *compare* side (idempotency) is not equally fixed. |
| **Critical-3** (silent destruction of other regions) | DEC-015, REQ-CMD-009, REQ-APL-008, REQ-API-005 | **CLOSED** (conceptually) | Hard-refuse collapse unless `--force`, reading the full live map. Imperative path is realizable (reads `ctx.Service.MultiRegion`); apply-path realizability inherits **NEW-MAJ-1** (data source at the pre-flight location). |
| **Major-1** (default-placement no-op unreliable) | DEC-022 | **CLOSED** | Empty live map is always treated as a change; sidesteps needing the default region's canonical name. Sound. |
| **Major-2** (no-op vs replicas rule) | DEC-022, REQ-CMD-006 | **CLOSED** | No-op now requires region **and** effective `numReplicas` equal; verification row exercises the region-equal-but-replicas-differ case. |
| **Major-3** (`--region` alone scales to 1) | DEC-016, REQ-CMD-002 | **CLOSED** | Update preserves the live count; default-1 only on create/unknown. |
| **Major-4** (bare `--replicas` wrong path) | DEC-019, REQ-CMD-008, REQ-API-003 | **CLOSED** | Bare replica change on a region-placed service routes through `multiRegionConfig`; verification asserts `{X:{4}}`, region preserved. |
| **Major-5** (declarative region, no replicas) | DEC-020, REQ-CFG-005 | **CLOSED** | Manifest region + omitted replicas defaults to live-if-known else 1, never 0; verification row present. |
| **Major-6** (atomic pre-flight infeasible) | DEC-021, REQ-APL-004 | **CLOSED** (volume) | Volume pre-flight relocated to `cmd/apply.go` after `fetchLiveState`, before `apply.Apply` — matches the real `ListVolumes` call at `cmd/apply.go:300`. The *collapse* half stapled to it is not (see **NEW-MAJ-1**). |
| **Gap-1** (region in delete diff) | REQ-APL-009 | **CLOSED** (single-region) | Delete diff renders `deploy.region`. Multi-region caveat in **NEW-MIN-2**. |
| **Gap-2** (create-path write acceptance) | REQ-CMD-001 + verification, REQ-API-002 | **CLOSED** | Imperative create acceptance present; apply-create map-shape assertion is implicit via REQ-API-002/REQ-APL-001 (slightly soft but adequate). |
| **Gap-3** (`RAILCTL_REGION` validated?) | REQ-CMD-003 | **CLOSED** | Resolved value from flag **or** env is validated identically; verification tests "unknown via flag OR env". |
| **Gap-4** (no-op vs volume order) | DEC-023, REQ-CMD-006 | **CLOSED** | No-op evaluated before the volume block; REQ-VOL-001 verification scopes to "not already-satisfied". |
| **XC-1** (live-state fields in Appendix E) | Appendix E | **CLOSED** | `ServiceDetail.MultiRegion`, `ServiceDetail.Region`, `LiveDeployConfig.Region`, and the `fetchLiveState` copy are listed. But `LiveService`/`LiveDeployConfig` still lack the *map* — root of **NEW-MAJ-1**/**NEW-CRIT-1**. |
| **XC-2** (queries must fetch `multiRegionConfig`) | REQ-API-005 | **CLOSED** | Both queries add the selection; `toServiceDetail` parses it into `MultiRegion` + derived `Region`. |
| **Minor-1** (dangling Q-002/Q-009) | Appendix B | **CLOSED** | Q-002 → DEC-002/REQ-CFG-001; Q-009 → REQ-DOC-001/002/003. No gaps remain. |
| **Minor-2** (weak acceptances: REQ-CMD-006, REQ-APL-005) | DEC-022 / REQ-CMD-006 (strengthened); REQ-APL-005 | **PARTIALLY CLOSED** | REQ-CMD-006 acceptance is now strong. **REQ-APL-005 "idempotent diff empty" is still not achievable** because the compare path reads flat replicas — see **NEW-CRIT-1**. The per-region replica read was specified for the *write* (REQ-APL-007) but not for the *diff/compare*. |
| **Minor-3** (DEC-002/007 precedence prose terse) | DEC-007 (rewritten normative) | **CLOSED** | DEC-007 is now first-class normative text with an explicit "region present" definition and refinement pointers. |

**Score: 18 CLOSED, 1 PARTIALLY CLOSED (Minor-2 / REQ-APL-005).**

---

## Part B — New issues introduced by iteration 3

### NEW-CRIT-1 — the declarative diff derives replicas from flat `numReplicas`, so region-placed services never reach a no-op

**Symptom.** REQ-APL-005 promises idempotency ("re-apply → no-op … idempotent diff empty") and DEC-022 defines the no-op as region **and** effective `numReplicas` equal. Iteration 3 fixed the *write* to resolve replicas from the per-region map (REQ-APL-007, DEC-018), but left the *compare* path untouched:

- `cmd/apply.go:fetchLiveState` populates `ls.Deploy.Replicas` from `svc.Replicas` (`cmd/apply.go:316`), i.e. the flat top-level `numReplicas` (`api/services.go:248`, `serviceInstanceNode.NumReplicas`).
- `diff.compareDeployConfig` compares desired `deploy.replicas` against that scalar `LiveDeployConfig.Replicas` (`diff/diff.go:524-530`).
- For a **region-placed** service the replica count lives in `multiRegionConfig[region]`, and the top-level `numReplicas` is typically 0/stale/default. So `LiveDeployConfig.Replicas` ≠ the real count.

Consequence: a manifest declaring `deploy.region: X` + `deploy.replicas: 3` against a live service correctly running `{X:{3}}` yields `live Replicas = 0` (or 1) → `compareDeployConfig` emits a `deploy.replicas` diff **on every apply**. The ChangeSet is never empty, `apply` always writes and redeploys, and REQ-APL-005's acceptance fails. This defeats the central idempotency guarantee for precisely the services this feature introduces. Appendix E leaves `LiveDeployConfig` with a scalar `Replicas` and never says it must be derived from `MultiRegion[Region]`.

Note this is asymmetric with the imperative path, which is fine: REQ-CMD-006's no-op reads `ServiceDetail.MultiRegion` (the map) directly. The hole is specific to the diff/compare layer.

**Fix.** Specify that for a region-placed live service, `fetchLiveState`/`LiveDeployConfig` MUST populate the effective `Replicas` from `MultiRegion[Region]` (not the flat `numReplicas`), or that `compareDeployConfig` MUST compare desired replicas against the per-region count when the service is region-placed. Add a verification: "manifest `{region:X, replicas:3}` over live `{X:{3}}` → empty diff (no `deploy.replicas`, no `deploy.region`)." This is what finally closes REVIEW-001 Minor-2 for REQ-APL-005.

---

### NEW-MAJ-1 — the apply-side collapse guard has no data source at the pre-flight location

**Symptom.** REQ-APL-008 says the multi-region collapse guard is "part of the atomic pre-flight, REQ-APL-004," which DEC-021/REQ-APL-004 place in `cmd/apply.go` after `fetchLiveState` and before `apply.Apply`. The guard needs the **full live `multiRegionConfig`** to detect `>1` region. But:

- REQ-APL-004 names only two data sources for the pre-flight: the ChangeSet fields and "the live volume set."
- `fetchLiveState` returns `[]diff.LiveService`, and per Appendix E `LiveDeployConfig` carries only `Region string` (the *derived single* region), **not** the map. For a multi-region service `Region` is deliberately `""`.

So at the exact location the spec mandates for the pre-flight, the live map cardinality is unavailable. The guard cannot be computed from what REQ-APL-004 says it has. (This is the REVIEW-001 Major-6 failure mode — asserting a guarantee whose data isn't present at the chosen site — reintroduced by the collapse-guard addition.)

**Fix.** State where the live map reaches the pre-flight: either (a) extend `fetchLiveState`/`LiveService`/`LiveDeployConfig` to carry `MultiRegion map[string]int`, and add it to Appendix E; or (b) have the pre-flight read `ServiceDetail.MultiRegion` from the already-listed services. Name the source in REQ-APL-004/REQ-APL-008 as was done for the volume set, so "refuse before mutating any service" is realizable. (Resolving this also supplies the data NEW-CRIT-1 needs.)

---

### NEW-MAJ-2 — effective replica count is undefined for a `--force` multi-region collapse without `--replicas`

**Symptom.** REQ-CMD-002 and DEC-016 define the fallback replica count as "**the** live replica count … from live `multiRegionConfig` if region-placed" — singular. But a genuinely multi-region service has one count *per region* (e.g. `{A:{2}, B:{5}}`). When `--force --region A` collapses it with no `--replicas`, "the live replica count" is undefined: is N = 2 (target region's), 5, 7 (sum), or 1? REQ-CFG-005/DEC-020 have the same singular phrasing on the manifest path. The REQ-CMD-009 / REQ-APL-008 verification rows only assert "write proceeds" under `--force` — they never pin N, so the acceptance is untestable on this axis.

**Fix.** Define N for a forced collapse (recommend: the target region's live count if the target is among the live regions, else 1 — matching the "preserve what you can" instinct of DEC-016), and add the asserted N to the `--force` verification rows.

---

### NEW-MIN-1 — REQ-API-003 conditions the flat path on live state the write method cannot see

**Symptom.** REQ-API-003 says the flat `numReplicas` path is taken "when no region is set **and the live service is not region-placed**." But the write method's contract (REQ-API-004) branches solely on `region *string` — `UpdateServiceInstanceConfig` has no visibility into live `multiRegionConfig`. The "not region-placed" half is enforceable only in the callers (REQ-CMD-008/REQ-APL-007), which resolve and pass the region. As written, REQ-API-003 assigns the API layer a decision it structurally cannot make; a reader implementing REQ-API-003/004 literally at the API layer could re-open the Major-4 failure (a nil-region write flattening a region-placed service). No REQ makes the write method itself guard against this.

**Fix.** Reword REQ-API-003 to state the condition is *caller-enforced* (the caller resolves live placement per REQ-CMD-008/REQ-APL-007 and passes `region` accordingly), keeping REQ-API-004 as the sole method-level contract. Editorial, but worth doing so the layering is unambiguous.

---

### NEW-MIN-2 — delete diff omits region for multi-region services

**Symptom.** REQ-APL-009 renders `deploy.region` in the delete diff, sourced from `ls.Deploy.Region` (`buildDeleteChange`, `diff/diff.go:315-332` pattern). Since `Region` is the *derived single* value (`""` when `len(MultiRegion) > 1`, per Appendix E), a pruned multi-region service shows **no** region line — the delete diff silently under-reports placement for exactly the services the collapse guard cares about.

**Fix.** Either accept and document the single-region scope of REQ-APL-009, or have `buildDeleteChange` enumerate each live region (e.g. `deploy.region[A]`, `deploy.region[B]`) from the map. Low stakes; a one-line scope note suffices.

---

### NEW-MIN-3 — `--force` × volume-bound × multi-region interaction is unspecified/untested

**Symptom.** Appendix E and DEC-015 correctly scope `--force` to the collapse guard "only. No other effect" — so a service that is *both* multi-region and volume-bound should still be refused by REQ-VOL-001/REQ-APL-004 even with `--force`. This precedence (force overrides collapse but not the volume block) is implied but never stated as a combined case, and no verification exercises it.

**Fix.** Add one sentence to DEC-015 (or REQ-VOL-001) stating `--force` does not bypass the volume block, and a verification row: "`--force --region X` on a multi-region *and* volume-bound service → still refused by the volume constraint."

---

## Consistency matrix — write path & N (task item, resolved reading)

| Scenario | Write path | N written | Consistent? |
|---|---|---|---|
| Imperative `--region X` (create) | map (REQ-API-002/004) | 1 (REQ-CMD-002) | ✅ |
| Imperative `--region X` (update, live `{X:{3}}`) | map | 3, preserve (DEC-016) | ✅ |
| Imperative `--region X --replicas 5` | map | 5 | ✅ |
| Imperative bare `--replicas 4`, live `{X:{2}}` | map for X (DEC-019/REQ-CMD-008) | 4 | ✅ |
| Manifest `region:X`, replicas omitted | map (REQ-APL-007) | live-if-known else 1 (DEC-020) | ✅ |
| Manifest replicas-only, live region-placed | map (REQ-APL-007) | resolved tuple | ✅ write / ❌ **diff never idempotent (NEW-CRIT-1)** |
| `--force` collapse `{A:2,B:5}`→A, no `--replicas` | map | **undefined (NEW-MAJ-2)** | ❌ |
| API method, `region==nil` | flat (REQ-API-004) | flat | ⚠️ REQ-API-003 wording (NEW-MIN-1) |

DEC-007 / DEC-016 / DEC-019 / DEC-022 do **not** contradict each other on the write path or on write-side N — the tuple-resolution model is coherent. The breakages are (a) the compare/diff path lagging the write path (NEW-CRIT-1), (b) the singular-count assumption under a forced collapse (NEW-MAJ-2), and (c) the pre-flight data source (NEW-MAJ-1).

---

## Quality-attribute scorecard (delta vs REVIEW-001)

| Criterion | REVIEW-001 | Now | Note |
|---|---|---|---|
| Unambiguous | Fails | **Mostly OK** | Residual: NEW-MAJ-2 (forced-collapse N), NEW-MIN-1 (REQ-API-003 layering). |
| Complete | Fails | **At risk** | NEW-CRIT-1 (compare-path replica derivation) and NEW-MAJ-1 (collapse data source) are genuine holes. |
| Consistent | Fails | **Mostly OK** | Write-path DECs are coherent; the diff/write asymmetry (NEW-CRIT-1) is the exception. |
| Verifiable | Mostly OK | **Mostly OK** | REQ-APL-005 acceptance still not achievable (NEW-CRIT-1); `--force` rows don't pin N (NEW-MAJ-2). |
| Feasible | At risk | **At risk** | Volume pre-flight now feasible; collapse guard at that site is not (NEW-MAJ-1). |
| Necessary | OK | OK | No gold-plating added. |
| Traceable | Minor gap | **OK** | Q-002/Q-009 closed; every REQ has a verification row; REQ→DEC links sound. |
| Modifiable | Minor gap | **OK** | Appendix E now lists the live-state fields (though not the map — see NEW-MAJ-1). |
| Bounded | Mostly OK | **OK** | Pre-existing multi-region services now explicitly in scope via the collapse guard. |

---

## Verdict

**Needs material revision.**

Rationale: one Critical-level correctness hole remains — **NEW-CRIT-1**, the declarative diff deriving region-placed replicas from the flat `numReplicas` field, which makes REQ-APL-005 idempotency and DEC-022 no-op unattainable for region-placed services (and leaves REVIEW-001 Minor-2 only partially closed). Two Major items (**NEW-MAJ-1** apply-side collapse-guard data source; **NEW-MAJ-2** undefined N on forced collapse) also need real DEC/REQ text, not editing. These share a single root cause — the live `multiRegionConfig` map is resolved on the *write* path but not carried into the *diff/compare/pre-flight* structures — so a single focused revision (thread `MultiRegion` through `LiveService`/`fetchLiveState`, use `MultiRegion[Region]` for replica compare, name it as the collapse-guard source, and pin forced-collapse N) should close all three plus the three Minors. The design is otherwise sound and ready; a follow-up REVIEW-003 scoped to that one revision should reach **Ready**.
