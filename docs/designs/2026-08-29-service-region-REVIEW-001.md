# SPEC Review 001 — service region placement

**Reviews:** `docs/designs/2026-08-29-service-region.md` (Status: Reviewable, iteration 2)
**Reviewer role:** IEEE 29148 specification reviewer
**Date:** 2026-08-29
**Method:** Audit against IEEE 29148 quality criteria (Unambiguous, Complete, Consistent, Verifiable, Feasible, Necessary, Traceable, Modifiable, Bounded), cross-checked against the live `railctl` codebase.

Each finding has a stable ID, a classification, the symptom (quoted REQ/DEC), and the shape of the fix.

Classifications: **Critical** (blocks implementation) · **Major** (substantive ambiguity or hole) · **Minor** (mechanical fix) · **Coverage Gap** (behaviour that should be specified but isn't) · **Cross-Cutting** (touches multiple REQs/DECs).

---

## Summary of verdict

The spec is well-structured and its safety instincts (DEC-008 leave-alone, DEC-009 atomic pre-flight, DEC-003 volume block) are sound. But the **region/replicas entanglement is specified only at the level of the low-level API method (REQ-API-002/003) and never carried through the diff/apply pipeline or the write-plumbing signature**, and the spec assumes every live service is single-region when the Railway model it targets is inherently multi-region. Three Critical holes and several Major ones must close before implementation. **Verdict: Needs material revision.**

---

## Critical

### Critical-1 — The write-plumbing contract for region is never specified

**Symptom.** REQ-API-002 mandates the *wire shape* —
> "the service-instance update MUST send `multiRegionConfig: { <region>: { numReplicas: N } }` and MUST NOT also send a top-level `numReplicas`"

— but no REQ/DEC states *which function* carries region or *what its signature becomes*. In the codebase every deploy-config write (imperative create, imperative update, `apply` create, `apply` update) funnels through a single method:

```
api.UpdateServiceInstanceConfig(serviceID, envID, startCommand, restartPolicy, maxRetries, replicas *int, healthcheckPath, healthcheckTimeout)   // internal/api/services.go:422
```

It has no region parameter, and `replicas` is a standalone `*int` that today always maps to top-level `numReplicas` (services.go:444). REQ-API-002 requires that when region is set, `replicas` must instead be folded *into* `multiRegionConfig` and the top-level field suppressed — a decision this method must make internally, yet the spec never gives it the region input nor the branching rule. Appendix E documents the `multiRegionConfig` *shape* but not the Go contract that produces it.

**Fix.** Add a DEC + Appendix E row fixing the new signature and its internal branch, e.g. `UpdateServiceInstanceConfig(..., replicas *int, ..., region *string)` with the rule: `region != nil` → emit `multiRegionConfig{ *region: { numReplicas: <resolved N> } }` and omit top-level `numReplicas`; `region == nil` → today's flat path. State that all four call sites (`cmd/create_service.go:231`, `cmd/update_service.go` via `applyUpdateDeployConfig`, `apply.applyCreate`, `apply.applyUpdate`) and the hand-written `MockClient` move to the new signature.

### Critical-2 — region/replicas entanglement is not carried through diff → apply

**Symptom.** DEC-007 resolves the entanglement *server-side* only:
> "When region is set, `multiRegionConfig` carries the replica count and the call omits top-level `numReplicas`… Resolves the region/replicas entanglement."

But `apply` does not assemble the write from a single desired object — it assembles it from **independent per-field diffs**. `diff.compareDeployConfig` emits `deploy.region` and `deploy.replicas` as separate `FieldDiff`s, and `apply.buildDeployConfigUpdate` (apply.go:591) reconstructs each pointer independently from whichever field drifted. Two unhandled cases fall out:

1. **Only region drifts** (replicas unchanged): `buildDeployConfigUpdate` produces `region` but `replicas == nil`. REQ-API-002 requires a `numReplicas` *inside* `multiRegionConfig` — but the count was never diffed, so where does N come from? Unspecified.
2. **Only replicas drift on a region-placed service** (region unchanged): the diff yields `deploy.replicas` alone, so the write takes the *flat* `numReplicas` path (REQ-API-003) against a service whose placement lives in `multiRegionConfig` — the wrong path (see Major-4).

REQ-API-002/003 are written as if the caller always knows the region at write time; through the declarative pipeline it frequently does not, because "unchanged" fields are dropped from the ChangeSet by design (the same zero-value-means-unmanaged model in compareDeployConfig, diff.go:508).

**Fix.** Specify that `apply` resolves the **full `(region, numReplicas)` tuple** for a service from the *desired config* (falling back to the *live* value for whichever half didn't drift) before writing, and that **any managed/desired region forces the `multiRegionConfig` path regardless of which sub-field drifted**. This likely means the apply layer must read from `configMap[name].Deploy` and live state rather than from `rc.Fields` for the region write — call that out, since it breaks the current field-diff-driven `buildDeployConfigUpdate` pattern.

### Critical-3 — a single-region write silently destroys other regions of an existing multi-region service

**Symptom.** DEC-002:
> "One region per invocation; setting it replaces prior placement."

combined with REQ-API-004, which only reads the *singular* region:
> "MUST surface the service's current region so apply/diff can read live placement."

Railway's `multiRegionConfig` is a map; a user (or the dashboard) may have placed a service in *two or more* regions. Because `railctl` reads only "the current region" (singular) and writes a one-entry map that *replaces* the whole config, a routine `railctl update service --region X` or an `apply` will **silently drop every other region** — a replica/availability loss with no diff, warning, or guard. No REQ detects a live map with >1 entry, and the read model (`ServiceDetail.Region string`, Appendix E) cannot even represent it. This is the "does a single-region write silently drop other regions?" question from the review brief, and the spec currently answers it "yes, silently, unspecified."

**Fix.** Pick one and specify it with a verification row:
- (a) Read the *full* live `multiRegionConfig`; if it has >1 entry and the operation would collapse it, **hard-refuse** (like the volume block) unless an explicit override flag is given; or
- (b) Explicitly document replace-all as accepted behaviour *and* add a pre-write warning naming the regions being dropped, plus a verification that the warning fires.
Either way REQ-API-004 must read enough to know the live map size, not just one name.

---

## Major

### Major-1 — no-op detection is unreliable for implicit default placement

**Symptom.** REQ-CMD-006 / REQ-APL-005:
> "if the service is already in the target region it MUST be a no-op (no write, no deploy)."

A service that has never had `multiRegionConfig` set runs in Railway's *default* region and its `multiRegionConfig` read is empty. So `--region <defaultRegionName>` compares desired `"us-west1"` against live `""` → *drift* → needless write + redeploy; and there is no way to confirm equality for a legitimately-in-default service. REQ-APL-005's acceptance ("re-apply → no-op / idempotent diff empty") will fail for exactly these services.

**Fix.** Define how default placement is represented and compared (e.g., resolve the account/project default region name so `""`-live can be equated to the default's canonical name), and make the no-op/idempotency acceptances explicit about the default-region case.

### Major-2 — no-op is defined on region alone, contradicting the replicas rule

**Symptom.** REQ-CMD-006 defines the no-op purely by region equality, but REQ-CMD-002 / DEC-010 make replicas part of the same write:
> REQ-CMD-002: "When `--replicas` accompanies `--region`, the region entry MUST carry that count".

So `--region X --replicas 5` against a service already in X with 2 replicas is **not** a no-op, yet REQ-CMD-006 as written ("already in the target region → no-op") would suppress it and drop the replica change.

**Fix.** Redefine the no-op as *both* region **and** effective `numReplicas` equal to the target; only then skip the write/deploy. Add the replica-differs case to REQ-CMD-006's verification row.

### Major-3 — `--region` alone silently scales an update down to 1 replica

**Symptom.** DEC-010 / REQ-CMD-002:
> "`--region` alone MUST default to 1 replica."

On *create* this is harmless. On *update* of a service currently running N>1 replicas in its current region, `railctl update service --region X` (no `--replicas`) writes `multiRegionConfig{ X: { numReplicas: 1 } }` — a silent scale-down from N to 1 bundled into a region move. Nothing in the spec flags this.

**Fix.** Specify that on *update*, `--region` without `--replicas` **preserves the live replica count** (read via REQ-API-004), defaulting to 1 only when live is unknown; or at minimum warn. Keep the "default 1" only for *create* (no live count exists).

### Major-4 — `--replicas` alone against a region-placed service takes the wrong write path

**Symptom.** REQ-API-003:
> "When no region is set, the update path MUST retain the existing flat `numReplicas` behaviour unchanged (backward compatible)."

This assumes the service's placement is the implicit default. For a service already placed via `multiRegionConfig` (multi-region, or single-region-but-non-default), a flat top-level `numReplicas` write is at best ignored and at worst resets/conflicts with the existing placement. The spec treats the flat path as universally safe; it is only safe for default-placed services. This is the review brief's item 6.

**Fix.** Specify that a bare `--replicas` (no `--region`) must first read live placement (REQ-API-004); if the service is region-placed, route the replica change *through* `multiRegionConfig` for the existing region rather than the flat field. Add a verification for "replicas-only change on a region-placed service keeps its region and updates the count."

### Major-5 — declarative region with no declared replicas has an undefined replica count

**Symptom.** DEC-010 fixes the default (1) only for the *flag* surface. The manifest path has no equivalent. Appendix E requires `multiRegionConfig` `{ "<name>": { "numReplicas": <N≥1> } }`, but `config.DeployConfig.Replicas` defaults to 0 = unmanaged (config.go:44; validated `>= 1` only when non-zero, config.go:375). So a manifest with `deploy.region` set and `deploy.replicas` omitted has **no N** to write, and 0 violates the `N≥1` invariant.

**Fix.** Add a REQ mirroring DEC-010 for the declarative path: when `deploy.region` is set and `deploy.replicas` is omitted, N defaults to the live count if known, else 1. Add a config/apply verification row.

### Major-6 — REQ-APL-004 atomic pre-flight: location and data source unspecified, and not feasible as currently framed

**Symptom.** REQ-APL-004:
> "If **any** service in the apply set requires a region change AND has an attached volume, `apply` MUST fail the entire operation before mutating any service (atomic pre-flight)…"

`apply.Apply` (apply.go:42) does not receive live volume state at all — it takes only the `ChangeSet` and `configMap`, then mutates services in a per-service loop, fetching volumes *lazily inside* `applyUpdate`/`findServiceVolumeInstanceID` (apply.go:511). By the time any volume is inspected, earlier services in the loop have already been mutated. The spec asserts the atomic guarantee but never says *where* the pre-flight runs or *how* it obtains live volume state for the whole set before the loop.

**Fix.** Specify the pre-flight explicitly: either (a) perform it in `cmd/apply.go` after `fetchLiveState` (which already lists all volumes once, apply.go:300) and before `apply.Apply`, scanning every `ResourceChange` whose fields include `deploy.region` against the live volume set; or (b) pass `[]diff.LiveService` (or a volume map) into `apply.Apply` and gate before the create/update loops. Name the data source (the existing single `ListVolumes` call) so the "before mutating any service" guarantee is realisable.

---

## Coverage Gaps

### Gap-1 — region in the prune/delete diff is unspecified

The Touch list names `diff.buildDeleteChange`, but no REQ or verification says whether a pruned service's live `deploy.region` should render in the delete diff. `buildDeleteChange` (diff.go:307) currently enumerates every live deploy field. **Fix:** state whether `deploy.region` appears in delete diffs and add/withhold it deliberately.

### Gap-2 — create-path region write acceptance is implicit

REQ-APL-001 ("on create, set placement") and REQ-CMD-001 (create `--region`) both depend on the create-time deploy-config write (`applyCreate` apply.go:150-155; `cmd/create_service.go:231`) learning to emit `multiRegionConfig`. This is entangled with Critical-1 but has no *acceptance* of its own. **Fix:** add a verification that create writes `multiRegionConfig["<region>"].numReplicas` (not flat) for both imperative create and apply-create.

### Gap-3 — is the `RAILCTL_REGION`-derived value validated?

REQ-CMD-005 makes `RAILCTL_REGION` a create-time fallback; REQ-CMD-003 says "a `--region` value MUST be validated (case-insensitive)". It is unclear whether the env-derived value is a "`--region` value" for validation purposes. **Fix:** state that the resolved region (flag or env) is validated identically, and add it to REQ-CMD-005's acceptance.

### Gap-4 — ordering of the no-op check vs the volume block on update

On `update --region` against a volume-attached service that is *already* in the target region, REQ-VOL-001 (hard error) and REQ-CMD-006 (no-op) both apply. Which wins? A no-op should presumably short-circuit before the volume error (nothing is being moved). **Fix:** specify the order (read live region → if equal, no-op and return; else volume check).

---

## Cross-Cutting

### XC-1 — live-state region fields missing from Appendix E

REQ-API-004 and REQ-APL-003 require region to flow through the read/diff path, which needs `diff.LiveDeployConfig.Region` and `types.ServiceDetail.Region`. Appendix E's field reference lists only `types.Region` and `config.DeployConfig.Region` — the two *live* fields (and their exact names) are absent, though the Touch list gestures at `diff.LiveDeployConfig`. This weakens Modifiability (an implementer can't see the full field set). **Fix:** add `ServiceDetail.Region string` and `LiveDeployConfig.Region string` (and the `fetchLiveState` copy `ls.Deploy.Region = svc.Region`, cmd/apply.go:311) to Appendix E.

### XC-2 — the GraphQL read queries must fetch `multiRegionConfig`, which is never stated

REQ-API-004 depends on the read surfacing placement, but `listServicesQuery` and `getServiceQuery` (services.go:11, 56) select only the flat `numReplicas` — no `multiRegionConfig`. The spec says only "surface the current region" via `serviceInstanceNode`, never that the *queries* gain a `multiRegionConfig` selection and that `toServiceDetail` parses it. This touches the read path, no-op detection (Major-1/2), diff rendering (REQ-APL-003), and Critical-3 (map size). **Fix:** state that both queries add `multiRegionConfig` and that `toServiceDetail` derives `Region` (and, for Critical-3, map cardinality / per-region replicas) from it.

---

## Minor

### Minor-1 — dangling open-question numbers

Appendix B claims "all initial questions closed → DEC-008..014" and lists only Q-007, but the DEC log references Q-001, Q-003, Q-004, Q-005, Q-006, Q-008, Q-010. **Q-002 and Q-009 appear nowhere** — neither asked nor mapped to a DEC. **Fix:** either restore/close Q-002 and Q-009 explicitly or renumber so the sequence has no gaps.

### Minor-2 — weak verification acceptances

Two acceptance rows under-test their REQ:
- REQ-CMD-006: "already-in-region → not called" never exercises the region-equal-but-replicas-differ path (see Major-2).
- REQ-APL-005: "re-apply → no-op (idempotent diff empty)" is untestable until per-region replica read and default-region equality (Major-1) are specified.

**Fix:** strengthen both acceptances once Major-1/-2 land. (Note: every REQ *does* have a verification row — no REQ is missing one — so this is about acceptance quality, not coverage.)

### Minor-3 — DEC-002/DEC-007 precedence prose is terse

DEC-002 ("API takes a map internally") and DEC-007's trailing "*Trace of the map's precedence note.*" gesture at the create-path precedence without restating it. Given Critical-1/-2, the precedence rule deserves a first-class sentence in the DEC rather than a marginal note. **Fix:** fold the resolved precedence rule (region present ⇒ map path carries replicas; region absent ⇒ flat path) into DEC-007 as normative text.

---

## Traceability / quality-attribute scorecard

| Criterion | Assessment |
|---|---|
| Unambiguous | **Fails** at the entanglement boundary (Critical-1/-2, Major-2/-4). |
| Complete | **Fails** — no multi-region live handling (Critical-3), no declarative replica default (Major-5), no query change stated (XC-2). |
| Consistent | **Fails** — REQ-CMD-006 no-op vs DEC-010 replicas (Major-2); DEC-008 leave-alone is otherwise consistent with the diff zero-value model. |
| Verifiable | Mostly OK — all REQs have rows; two acceptances weak (Minor-2). |
| Feasible | **At risk** — REQ-APL-004 atomic pre-flight not realisable as framed (Major-6). |
| Necessary | OK — no gold-plating; non-goals are disciplined. |
| Traceable | Minor gap — dangling Q-002/Q-009 (Minor-1); REQ→DEC links otherwise sound. |
| Modifiable | Minor gap — Appendix E omits live-state fields (XC-1). |
| Bounded | Mostly OK — scope is clear, but the boundary "what happens to pre-existing multi-region services" is inside the blast radius yet unaddressed (Critical-3). |

### Note on DEC-008 vs the diff model (brief item 2)
The "omit leaves alone" rule (DEC-008) **is** consistent with how `diff` actually works: `compareDeployConfig` (diff.go:508) treats a zero-value field as unmanaged, so an empty `deploy.region` string never diffs — exactly matching `startCommand`/`healthcheckPath` and the `deleteProtection` pointer pattern. No consistency gap there for a *managed* region. The real inconsistency is downstream (Critical-2): dropping *unchanged* fields from the ChangeSet is precisely what makes the entangled `multiRegionConfig` write hard to assemble.

---

## Verdict

**Needs material revision.**

Rationale: three Critical findings block implementation — the write-plumbing contract (Critical-1), the diff→apply entanglement (Critical-2), and silent destruction of pre-existing multi-region placements (Critical-3). None can be resolved by an editorial pass; each needs a new/expanded DEC and matching verification. Once those and the Major items (especially Major-5 declarative replica default and Major-6 pre-flight feasibility) are addressed, a follow-up REVIEW-002 should be able to reach Ready.
