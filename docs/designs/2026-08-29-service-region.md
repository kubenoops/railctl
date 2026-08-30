# Design/SPEC: service region placement

**Date:** 2026-08-29
**Status:** Reworking (iteration 7 — volume-guard correction (§0.5); iterations 1–5 below are SUPERSEDED)
**Branch:** `feat/service-region`
**Method:** Spec-driven (numbered REQ/DEC/verification per the `spec-driven-development` skill)

---

## 0. Design correction — verified live mechanism (iteration 6) ⚠️ SUPERSEDES §§1–6 below

Iterations 1–5 (and their three reviews) were designed against the **vendored Rust CLI's
stale GraphQL schema**. Live testing against `backboard.railway.com/graphql/v2` (2026-08-29,
with a real project token, cross-checked against railwayapp/cli v5.45.10) proved that core
design **wrong**. The sections below (§§1–6, all DECs/REQs) are retained for history but are
**superseded** by this section. A follow-up editorial pass will renumber them; until then,
**this section is normative** where it conflicts.

### 0.1 What was wrong (all verified)

- `Region.railwayMetal` — **does not exist**. `ServiceInstance.multiRegionConfig` (read) —
  **does not exist** (adding it to the service read queries broke *all* service commands).
- Placement is **not** written via `serviceInstanceUpdate`, and the `ServiceInstance.region`
  string is not the placement store (reads null even after a successful write).
- railctl sends every token as `Authorization: Bearer`; Railway **project tokens** require the
  **`project-access-token`** header for the region operations (and for `serviceDelete` and
  reading deployment `meta`) — Bearer yields "Not Authorized".

### 0.2 Verified mechanism (normative)

- **Auth (REQ-API-100):** when the token is a project token, requests MUST use the
  `project-access-token: <token>` header instead of `Authorization: Bearer`. (Account/workspace
  tokens keep Bearer.) This is foundational and also corrects other project-token operations.
- **Write placement (REQ-API-101):** `environmentPatchCommit(environmentId, patch:
  EnvironmentConfig!, commitMessage)` with
  `patch = {services:{<serviceId>:{deploy:{multiRegionConfig:{<regionId>:{numReplicas:N}|null}}}}}`.
  **Single-region placement** sets the target region and sets every other currently-present
  region to `null` (removal). Region keys are **region IDs**.
- **Read placement (REQ-API-102):** parse the service instance's latest deployment `meta` at
  `serviceManifest.deploy.multiRegionConfig` (region ID → `{numReplicas}`). There is no
  first-class read field.
- **Region list (REQ-API-103 / DEC-100):** the live `regions` query is **not authorized for
  project tokens** (verified even with the correct header + `projectId`). Therefore the region
  list is **hardcoded** in the CLI (no live query). `get regions` and `--region` validation use
  the hardcoded list.
- **Hardcoded regions (REQ-API-104), validated live** (docs.railway.com/reference/regions):
  `us-west2` (California, USA), `us-east4-eqdc4a` (Virginia, USA),
  `europe-west4-drams3a` (Amsterdam, Netherlands), `asia-southeast1-eqsg3a` (Singapore).
  A code update is required if Railway adds regions (accepted trade-off; DEC-101).

### 0.3 Design deltas from §§1–6

- **Keep (now correctly wired):** imperative `--region` on create/update; `get regions`;
  declarative `deploy.region`; per-region read for idempotency (from meta, not a field);
  no-op detection; volume guard (best-effort pre-check, unverified against this mechanism);
  the multi-region collapse guard **remains meaningful** (placement is genuinely a map).
- **Change:** write path → `environmentPatchCommit`; read path → deployment meta; region
  identity → region **IDs** from the hardcoded list; auth → `project-access-token` for project
  tokens.
- **Drop:** `railwayMetal`/`Metal`; `ServiceInstance.multiRegionConfig` read field; the
  `serviceInstanceUpdate`-based `region`/`multiRegionConfig` write; the live `regions(projectId:)`
  query and any "get regions needs a workspace token" messaging (superseded by the hardcoded list).
- **DEC-102:** `environmentPatchCommit` is a new API primitive in railctl; the region write does
  **not** reuse `UpdateServiceInstanceConfig` (which stays for the other deploy-config fields).

### 0.4 Verified evidence (2026-08-29, project token, interu-adapter/production)

- Write: `environmentPatchCommit` with the region patch → commit ref returned; meta read-back
  showed `{us-west2, us-east4-eqdc4a, europe-west4-drams3a, asia-southeast1-eqsg3a}`.
- All four region IDs accepted; `iad:null` removed the default; `serviceDelete` + meta read work
  with the `project-access-token` header; `regions` denied to the project token regardless.

### 0.5 Volume-guard correction (iteration 7 — Railway migrates volumes)

- **DEC-103 — Railway auto-migrates volumes on region change; the hard volume block becomes a
  `--force`-gated guard.** *Supersedes DEC-003's premise and the detach→recreate guidance in
  REQ-VOL-001 / REQ-APL-004.* Verified 2026-08-30: the dashboard shows "migrating volume to
  region…" when a volume-carrying service changes region; Railway's regions doc states a
  mounted volume triggers an automatic migration, with the deployment held until the volume is
  ready in the new region (downtime, duration size-dependent); the 2025-05-02 changelog
  ("Stateful Migrations Now Available") introduced the capability. The guard's purpose is now
  **accidental-downtime protection**, not impossibility. Residual Railway limit (not enforced
  by railctl): metal ↔ non-metal migrations are unsupported.
- **REQ-VOL-100** — `update service --region` on a service with an attached volume MUST refuse
  by default with a warning that Railway will migrate the volume and the service will be down
  for the duration, and MUST proceed when `--force` is passed (the same flag that overrides
  the collapse guard, DEC-015 — one `--force` acknowledges both consequences). The no-op check
  (DEC-023) still runs first. *Replaces REQ-VOL-001's "`--force` MUST NOT bypass".*
  | Trace: DEC-103, DEC-015, DEC-023 | Impl: Cmd
- **REQ-VOL-101** — the `apply` pre-flight (REQ-APL-004 mechanics unchanged: atomic, in
  `cmd/apply.go`, before any mutation) MUST apply the same rule: a volume-bound region change
  refuses without `apply --force` (migration/downtime message) and proceeds with it.
  | Trace: DEC-103, DEC-009 | Impl: Apply
- **Verification:** REQ-VOL-100 — integration: update `--region` on a volume-bound service →
  error contains `migrate` and `--force`, no write; with `--force` → write proceeds. e2e: same
  pair live. REQ-VOL-101 — integration: apply set with a volume-bound region change fails
  pre-mutation without `--force`, proceeds with it.
- **DEC-104 — the declarative path resolves `deploy.region` to the full region ID before any
  write; diff normalizes both sides to short names.** *Augments DEC-006 and REQ-API-104.*
  Found live (2026-08-30, full-suite pass 3): `apply` committed the manifest's literal short
  name (`us-east4`), which Railway accepts as the LEGACY non-metal region — service placement
  works, but a volume migration to it fails ("Reset region due to volume migration failure":
  volumes cannot migrate between metal and non-metal regions), intermittently breaking the
  declarative migration. Now `apply` (create + update) resolves via the shipped list exactly
  like `--region` (unknown name → hard error at apply time), the bare-replicas path keeps
  addressing the live key verbatim (it may legitimately be legacy `iad`), `RegionsToClear`
  already nulls legacy short keys on migration, and `diff` compares/displays short names on
  both sides so a short-name manifest stays idempotent against full-ID live keys.
  Verification — unit: resolveApplyRegion returns the full ID for a short name and errors on
  unknown names; diff: desired short vs live full ID → no drift. e2e: TestDeclarativeMigration
  (the patch history commit message must carry the full ID).

---

**Review history:**
- `REVIEW-001.md` (Needs material revision) — 19 findings closed in iteration 3.
- `REVIEW-002.md` (Needs material revision) — 18/19 confirmed closed; 1 new blocker (NEW-CRIT-1)
  + NEW-MAJ-1/2 + NEW-MIN-1/2/3, all one root cause: the live `multiRegionConfig` map was
  threaded into the *write* path but not the *diff/compare/pre-flight* path. Closed in iteration 4.
- `REVIEW-003.md` (**Ready**) — all 6 REVIEW-002 findings confirmed closed, no new contradiction;
  one editorial residual (RES-1) folded into REQ-APL-010 in iteration 5.

Central resolution: railctl reads the **full live `multiRegionConfig`** and treats `(region,
replicas)` as one resolved tuple (preserving the half the user didn't specify), forcing the
`multiRegionConfig` write path whenever a service is region-placed, comparing replicas against
the **per-region** live count, and hard-refusing a multi-region collapse unless `--force`.

Conformance keywords (MUST, MUST NOT, SHOULD, MAY, MAY NOT) are used per RFC 2119.

Impl tags: `Cmd | API | Config | Apply | Diff | Skill | Doc | Test`.

---

## 1. Scope

Add first-class support for choosing the **region** a Railway service runs in, both
imperatively (`create service` / `update service`) and declaratively (the config manifest
consumed by `apply`/`diff`), plus a `get regions` discovery command. Railway models
placement as `multiRegionConfig` (a JSON map of `region → { numReplicas }`) on the
`serviceInstanceUpdate` mutation; available regions come from the `regions` query.

## 2. Goals / non-goals

**Goals**
- Set a service's region on create and update from the CLI.
- Declare a service's region in the manifest and reconcile it via `apply`/`diff`.
- List valid regions (`get regions`).
- Enforce the region-bound-volume constraint safely.
- Document region in `railctl skill` and the declarative-config reference.

**Non-goals**
- Multi-region replica distribution in one command (single region only; DEC-002).
- Moving/migrating volume data across regions (Railway does not support it).
- Reworking the replica model beyond what region placement requires.

## 3. Out of scope (with rationale)

- **Multi-region fan-out** — larger surface + diff/verification cost; single-region covers
  "override the default region." API shaped for a later additive change (DEC-002).
- **Offline region-name validation** in `config.Validate` — it is API-free; name validity is
  enforced at apply/imperative time (DEC-006).
- **`$env()` expansion for `deploy.region`** — region names are stable identifiers, not
  env-derived secrets; kept literal (DEC-013).
- **Surfacing current region in `describe service` / `get services` human output** — the live
  region is read for diff/apply, but a dedicated display column is deferred (Q-010, DEC-014).

## 4. Glossary

- **Region** — a Railway location a service instance can run in, canonical name e.g.
  `us-west1`; enumerated by the `regions` query.
- **multiRegionConfig** — Railway's per-service-instance JSON map `{ region: { numReplicas } }`
  determining placement and per-region replica count; set via `serviceInstanceUpdate`.
- **Region-bound volume** — a Railway volume physically located in its creation region; it
  cannot move, so a service with an attached volume cannot change region.
- **Single-region placement** — a `multiRegionConfig` with exactly one entry; setting it
  replaces any prior placement.

---

## 5. Requirements

### 5.1 API layer

| ID | Requirement | Trace | Impl |
|---|---|---|---|
| REQ-API-001 | `ListRegions() ([]types.Region, error)` MUST execute the `regions` query and return each region's `name`, `country`, `location`, and metal flag. | DEC-001, DEC-004 | API |
| REQ-API-002 | When a region is set, the service-instance update MUST send `multiRegionConfig: { <region>: { numReplicas: N } }` and MUST NOT also send a top-level `numReplicas` in the same call (multiRegionConfig owns replica count). | DEC-001, DEC-007, DEC-010 | API |
| REQ-API-003 | The flat `numReplicas` path is used exactly when the caller passes `region == nil` (REQ-API-004). Choosing the flat vs map path based on live placement is **caller-enforced** (REQ-CMD-008/REQ-APL-007 resolve live placement and pass `region` accordingly); the write method itself branches only on the `region` parameter and MUST NOT be assumed to inspect live state. | DEC-007, DEC-019 | API |
| REQ-API-004 | The write method `UpdateServiceInstanceConfig` MUST gain a `region *string` parameter. `region != nil` ⇒ emit the `multiRegionConfig` shape of REQ-API-002 and omit top-level `numReplicas`; `region == nil` ⇒ today's flat path. The `APIClient` interface and the hand-written `MockClient` (field + dispatch) MUST move to the new signature, as MUST all call sites (`cmd/create_service.go`, `cmd/update_service.go` via `applyUpdateDeployConfig`, `apply.applyCreate`, `apply.applyUpdate`). | DEC-017 | API |
| REQ-API-005 | The service read path MUST surface the service's **full live `multiRegionConfig`** as `types.ServiceDetail.MultiRegion` (map region→numReplicas), plus a derived single `Region` (the sole key when exactly one entry, else empty). `listServicesQuery` and `getServiceQuery` MUST add a `multiRegionConfig` selection and `toServiceDetail` MUST parse it. | DEC-008, DEC-011, DEC-015, XC-2 of REVIEW-001 | API |

### 5.2 Imperative commands

| ID | Requirement | Trace | Impl |
|---|---|---|---|
| REQ-CMD-001 | `create service` and `update service` MUST accept `--region <name>` setting single-region placement. | DEC-002 | Cmd |
| REQ-CMD-002 | The effective replica count written for `--region X` MUST be: `--replicas` when given; else on **update**, the service's current scale — the target region's live count (`MultiRegion["X"]`) when X is among the live regions, else the single live region's count for a single-region service moving to a new region, else the live flat `numReplicas` when default-placed, else 1; on **create**, 1. `--region` MUST NOT silently scale a running service down. For a `--force` collapse of a *multi-region* service to X, it is X's live count when X is among the live regions, else 1 (DEC-025). | DEC-010, DEC-016, DEC-025 | Cmd |
| REQ-CMD-003 | The resolved region value (from `--region` **or** `RAILCTL_REGION`) MUST be validated against `ListRegions` (case-insensitive on canonical name); an unknown region MUST error and list the valid names. | DEC-006, Gap-3 of REVIEW-001 | Cmd |
| REQ-CMD-004 | On `create service`, region MUST be resolved/validated **before** the service is created (fail fast, no orphaned service). | DEC-006 | Cmd |
| REQ-CMD-005 | `create service --region` MUST fall back to `RAILCTL_REGION` when the flag is omitted (flag wins); `update service` MUST NOT consult `RAILCTL_REGION`. | DEC-005, DEC-012 | Cmd |
| REQ-CMD-006 | A region change on `update service` MUST trigger a deployment unless `--skip-deployment`. It MUST be a no-op (no write, no deploy) **only when the live `multiRegionConfig` already equals exactly `{ <target>: { numReplicas: <effective N> } }`** — i.e. both region and effective replica count match. A service in Railway's default placement (empty live map) is always treated as a change. The no-op check MUST be evaluated **before** the volume block (REQ-VOL-001), so an already-satisfied request on a volume-bound service succeeds silently. | DEC-011, DEC-022, DEC-023 | Cmd |
| REQ-CMD-007 | `get regions` MUST list available regions in `table`/`wide`/`json`/`yaml` (table: NAME, LOCATION; wide adds COUNTRY, METAL). | DEC-004 | Cmd |
| REQ-CMD-008 | A bare `--replicas` (no `--region`) on `update service` MUST first read live placement (REQ-API-005); if the service is region-placed, the replica change MUST be written **through `multiRegionConfig` for the existing region**, not the flat `numReplicas` field. | DEC-019 | Cmd |
| REQ-CMD-009 | If the live service has a `multiRegionConfig` with **>1 region** and a `--region` write would collapse it to one, `update service` MUST hard-refuse with an error naming the regions that would be dropped, unless `--force` is passed. `--force` MUST be a new flag on `update service`. | DEC-015 | Cmd |

### 5.3 Volume constraint

| ID | Requirement | Trace | Impl |
|---|---|---|---|
| REQ-VOL-001 | `update service --region` MUST refuse with a hard error (detach → recreate guidance) when the target service has a volume attached in that environment, and MUST NOT write the region config. `--force` (which only overrides the collapse guard, DEC-015) MUST NOT bypass this volume block: a service that is both multi-region and volume-bound is still refused. | DEC-003, DEC-015 | Cmd |
| REQ-VOL-002 | `create service --region` MUST NOT perform the volume check (a new service has no volume). | DEC-003 | Cmd |

### 5.4 Config / manifest

| ID | Requirement | Trace | Impl |
|---|---|---|---|
| REQ-CFG-001 | `services[].deploy.region` (string) MUST be a supported manifest field. | DEC-002 | Config |
| REQ-CFG-002 | `config.Validate` MUST remain API-free; it MAY format-check region only (e.g. non-empty when the key is present) and MUST NOT validate region names. | DEC-006 | Config |
| REQ-CFG-003 | `deploy.region` MUST NOT be subject to `$env()` expansion. | DEC-013 | Config |
| REQ-CFG-004 | Legacy config MUST load with region unset (no legacy equivalent field). | DEC-002 | Config |
| REQ-CFG-005 | When `deploy.region: X` is set and `deploy.replicas` is omitted (0), the effective replica count MUST default to X's live count (`MultiRegion["X"]`) when X is among the live regions, else 1 — never 0 (which would violate the `N≥1` invariant of Appendix E). | DEC-020, DEC-025 | Config, Apply |

### 5.5 Apply / diff

| ID | Requirement | Trace | Impl |
|---|---|---|---|
| REQ-APL-001 | `apply` MUST reconcile `deploy.region`: on create, set placement; on update, change it when the live region differs. | DEC-002, DEC-011 | Apply |
| REQ-APL-002 | An omitted `deploy.region` MUST leave the live region unchanged (never reset or migrate). | DEC-008 | Apply, Diff |
| REQ-APL-003 | `diff` MUST render region drift as a `deploy.region` field diff (`<current> → <desired>`) via the path-agnostic renderer. | DEC-008 | Diff |
| REQ-APL-004 | The volume/region pre-flight MUST run in `cmd/apply.go` **after `fetchLiveState`** (which already lists all volumes once **and now carries `LiveDeployConfig.MultiRegion`**, REQ-APL-010) and **before** `apply.Apply`'s mutation loop, scanning every `ResourceChange` whose fields include `deploy.region` against the live volume set. If any such service has an attached volume, `apply` MUST fail the whole operation before mutating any service, citing detach → recreate guidance. | DEC-003, DEC-009, DEC-021 | Apply |
| REQ-APL-005 | A region change applied via `apply` MUST trigger a deployment (consistent with apply's deploy/`--await` flow); already-in-region-and-replicas MUST be a no-op (idempotent: a second `apply`/`diff` shows no `deploy.region` change). | DEC-011, DEC-022 | Apply |
| REQ-APL-006 | `RAILCTL_REGION` MUST NOT influence `apply`/`diff` (the manifest is the source of truth). | DEC-012 | Apply |
| REQ-APL-007 | For any service whose desired config sets `deploy.region`, `apply` MUST resolve the full `(region, numReplicas)` tuple from the desired config, falling back to live state (REQ-API-005) for whichever half did not drift, and MUST take the `multiRegionConfig` write path — even when only one of region/replicas appears in the ChangeSet. The region write MUST be driven from `configMap[name].Deploy` + live state, not solely from `rc.Fields`. | DEC-007, DEC-018 | Apply |
| REQ-APL-008 | `apply`/`diff` MUST apply the same multi-region collapse guard as REQ-CMD-009: a desired single `deploy.region` against a live `multiRegionConfig` with >1 region MUST be refused (in the REQ-APL-004 pre-flight, reading `LiveDeployConfig.MultiRegion` per REQ-APL-010) unless `apply --force` is passed. | DEC-015 | Apply |
| REQ-APL-009 | A pruned/deleted service's live `deploy.region` MUST render in the delete diff alongside the other live deploy fields (consistency with `buildDeleteChange`). This renders the **derived single** region; a pruned service with >1 live region shows no region line (documented single-region scope, acceptable because delete removes the whole service regardless). | Gap-1 of REVIEW-001, NEW-MIN-2 | Diff |
| REQ-APL-010 | `LiveService`/`LiveDeployConfig` MUST carry `MultiRegion map[string]int` (populated by `fetchLiveState` from `ServiceDetail.MultiRegion`). For a single-region live service (`len(MultiRegion)==1`), the effective replica count used by `compareDeployConfig` MUST be `MultiRegion[Region]`, **not** the flat top-level `numReplicas` — so a manifest matching the live per-region count produces an empty diff (idempotency). When the live service has **>1 region** (`Region==""`), a manifest that does not set `deploy.region` MUST NOT emit a `deploy.replicas` drift (a single-region manifest cannot manage a multi-region service's replicas; leave it alone — RES-1 of REVIEW-003). | NEW-CRIT-1, NEW-MAJ-1, RES-1 of REVIEW-002/003 | Diff, Apply |

### 5.6 Docs / skill

| ID | Requirement | Trace | Impl |
|---|---|---|---|
| REQ-DOC-001 | `docs/declarative-config.md` MUST document `deploy.region` (deploy table row + example). | DEC-002 | Doc |
| REQ-DOC-002 | `docs/railctl-skill.md` MUST document `--region`, `get regions`, and the volume constraint; the embedded copy MUST be regenerated via `make gen` so `gen-check` passes. | DEC-004 | Skill, Doc |
| REQ-DOC-003 | A `docs/region-placement.md` MUST describe behaviour, the volume constraint, and `RAILCTL_REGION` scope. | DEC-003, DEC-005 | Doc |

---

## 6. Verification

| ID | Method | Acceptance |
|---|---|---|
| REQ-API-001 | Unit (httptest) | Mock `regions` response → returns parsed regions with name/location/metal. |
| REQ-API-002 | Unit (httptest capture) | With region set, captured request body has `input.multiRegionConfig["us-west1"].numReplicas == N` and no top-level `input.numReplicas`. |
| REQ-API-003 | Unit | Region unset + replicas set + live map empty → request body has top-level `input.numReplicas`, no `multiRegionConfig`. |
| REQ-API-004 | Unit | `region != nil` → multiRegionConfig path; `region == nil` → flat path. `MockClient` + all call sites compile against the new signature. |
| REQ-API-005 | Unit | Service query fixture with a 2-entry `multiRegionConfig` → `ServiceDetail.MultiRegion` has both; `Region` empty. Single-entry → `Region` set. |
| REQ-CMD-001 | Integration (mock) | `create`/`update` with `--region us-west1` calls the region write with `us-west1`. |
| REQ-CMD-002 | Integration | update `--region X` on a live-3-replica service (no `--replicas`) → writes numReplicas 3; `--region X --replicas 5` → 5; create `--region X` → 1. |
| REQ-CMD-003 | Integration | Unknown region (via flag OR env) → error listing valid names; `US-West1` resolves to `us-west1`. |
| REQ-CMD-004 | Integration | Unknown region on create → `CreateService` never called. |
| REQ-CMD-005 | Integration | `RAILCTL_REGION` set + no flag on create → region applied; on update → ignored. |
| REQ-CMD-006 | Integration | Region or replica delta → `DeployServiceInstance` called (absent `--skip-deployment`); live map already `{X:{n}}` and target `{X:{n}}` → no write, no deploy; region-equal-but-replicas-differ → NOT a no-op (write happens). |
| REQ-CMD-007 | Integration + e2e | `get regions -o json` valid JSON with ≥1 region; table shows NAME/LOCATION. |
| REQ-CMD-008 | Integration | Bare `--replicas 4` on a service live-placed in `{X:{2}}` → write is `multiRegionConfig{X:{4}}`, region X preserved, not flat numReplicas. |
| REQ-CMD-009 | Integration + e2e | update `--region A` on live `{A:{2},B:{5}}` → error naming A,B; region not written. With `--force` (no `--replicas`) → writes `{A:{2}}` (target region's live count per DEC-025); with `--force --region C` (C not live) → `{C:{1}}`. |
| REQ-VOL-001 | Integration + e2e | Update `--region` on a volume-attached service (not already-satisfied) → error containing `region-bound`; region write not called. `--force --region X` on a service that is both multi-region and volume-bound → still refused by the volume block (force does not bypass it). |
| REQ-VOL-002 | Integration | Create `--region` runs no `ListVolumes` check. |
| REQ-CFG-001 | Unit | Manifest with `deploy.region` loads into `DeployConfig.Region`. |
| REQ-CFG-002 | Unit | `Validate` performs no API call; an unknown-name region passes offline validation. |
| REQ-CFG-003 | Unit | `deploy.region: "$env(X)"` is preserved literally (not expanded). |
| REQ-CFG-004 | Unit | Legacy config loads with `Region == ""`. |
| REQ-CFG-005 | Unit + integration | Manifest `deploy.region` set, `replicas` omitted, live=3 → writes numReplicas 3; live unknown → 1; never 0. |
| REQ-APL-001 | Unit + e2e | apply create/update sets region; live-differs triggers change. |
| REQ-APL-002 | Unit + e2e | Omitted region → no `deploy.region` diff and no region write. |
| REQ-APL-003 | Unit | `diff` output contains `deploy.region` current→desired. |
| REQ-APL-004 | Integration + e2e | apply set with a volume-bound region change → whole apply fails pre-mutation (in cmd/apply.go); no service mutated (assert first service untouched). |
| REQ-APL-005 | Unit + e2e | apply region change → deployment triggered; re-apply → no-op. Unit: manifest `{region:X, replicas:3}` over live `MultiRegion{X:3}` → empty ChangeSet (no `deploy.replicas`, no `deploy.region`). |
| REQ-APL-006 | Unit | With `RAILCTL_REGION` set and region omitted in manifest → no region diff/write. |
| REQ-APL-007 | Unit | Desired `region=X`, replicas undrifted, live `{X:{2}}`+region drift to Y → write `{Y:{2}}` (replicas from live); region-only ChangeSet still takes multiRegionConfig path. |
| REQ-APL-008 | Integration + e2e | apply single region over live `{A,B}` → pre-flight refuses; `apply --force` proceeds. |
| REQ-APL-009 | Unit | Delete diff for a single-region service includes a `deploy.region` line; a >1-region service shows none (documented scope). |
| REQ-APL-010 | Unit | `fetchLiveState` copies `MultiRegion`; `compareDeployConfig` on live `{X:3}` vs desired `{region:X, replicas:3}` → no `deploy.replicas` diff; vs `replicas:5` → one diff. |
| REQ-DOC-001 | Manual/review | `deploy.region` present in declarative-config.md deploy table + example. |
| REQ-DOC-002 | CI (`gen-check`) + review | skill doc mentions `--region`, `get regions`, volume constraint; `make gen` leaves no diff. |
| REQ-DOC-003 | Manual/review | region-placement.md exists and covers volume constraint, `--force` collapse, and env scope. |

---

## Appendix A — Decisions log

- **DEC-001 — Region set via `multiRegionConfig` on `serviceInstanceUpdate`.** Names read from the `regions` query.
- **DEC-002 — Single-region flag surface (`--region <name>`).** One region per invocation; setting it replaces prior placement. API takes a map internally so multi-region is a later additive change.
- **DEC-003 — Region change hard-blocked when a volume is attached.** `update`/`apply` refuse with detach→recreate guidance; `create` never blocked.
- **DEC-004 — `get regions` command included.**
- **DEC-005 — `RAILCTL_REGION` defaults `create service` only.** `update` requires explicit `--region`. *Refined by DEC-012 (no effect on apply).*
- **DEC-006 — Region-name validity enforced at apply/imperative time, not in offline `config.Validate`.**
- **DEC-007 — Region/replicas write precedence (normative).** When a region is present for a service, the write MUST take the `multiRegionConfig` path and that map owns the replica count (top-level `numReplicas` is omitted). When no region is present and the service is not region-placed, the flat `numReplicas` path is used unchanged. "Region present" means: the `--region` flag/`RAILCTL_REGION` (imperative), OR `deploy.region` in the manifest, OR a non-empty live `multiRegionConfig` (a service already region-placed — see DEC-019). *Refined by DEC-017 (signature), DEC-018 (apply tuple), DEC-019 (bare replicas).*
- **DEC-008 — Omitted `deploy.region` leaves live placement alone** (safety principle shared with `deleteProtection`/`customDomains`). Closes Q-003.
- **DEC-009 — `apply` blocks the entire operation (atomic pre-flight) if any targeted region change hits a volume-bound service**, rather than skip-and-continue. Closes Q-004.
- **DEC-010 — `--region` carries `--replicas` when both given; `--region` alone defaults to 1 replica.** Closes Q-001.
- **DEC-011 — A region change triggers a redeploy (update respects `--skip-deployment`; apply uses its deploy flow); already-in-region is a no-op.** Closes Q-008.
- **DEC-012 — `RAILCTL_REGION` does not influence `apply`/`diff`.** *Refines DEC-005.* Closes Q-005.
- **DEC-013 — `deploy.region` is literal; no `$env()` expansion.** Closes Q-006.
- **DEC-014 — Current-region display in `describe`/`get services` human output is deferred** (live region is still read for diff/apply). Closes Q-010.
- **DEC-015 — A multi-region collapse is hard-refused unless `--force`.** If the live `multiRegionConfig` has >1 region and a single-region write would collapse it, `update service` and `apply` MUST refuse (naming the dropped regions) unless `--force`/`apply --force`. Requires reading the full live map (DEC per REQ-API-005). Closes Critical-3 of REVIEW-001.
- **DEC-016 — `--region` alone preserves the service's current scale on update; defaults to 1 only on create (or when live is unknown).** Preserving means: the target region's live count if already placed there, else (for a single-region service moving to a new region) that region's count, else the flat count when default-placed, else 1. Prevents a silent scale-down when moving a running service. Closes Major-3. *Refines DEC-010; wording widened during implementation (Task 3) after the original "target region only" phrasing was found to scale a single-region move down to 1.*
- **DEC-017 — The write method gains a `region *string` parameter with the DEC-007 branching rule.** Interface, hand-written mock, and all four call sites move to the new signature. Closes Critical-1.
- **DEC-018 — `apply` resolves the full `(region, numReplicas)` tuple from desired config + live state before writing, and any managed/desired region forces the `multiRegionConfig` path regardless of which sub-field drifted.** The region write is driven from `configMap[name].Deploy` + live state, not solely from the per-field ChangeSet. Closes Critical-2.
- **DEC-019 — A bare replica change against a region-placed service is routed through `multiRegionConfig` for its existing region, not the flat field.** railctl reads live placement first. Closes Major-4.
- **DEC-020 — Declarative `deploy.region` with omitted `replicas` defaults the count to live-if-known, else 1 (never 0).** Mirrors DEC-016 for the manifest path. Closes Major-5.
- **DEC-021 — The apply volume/region pre-flight runs in `cmd/apply.go` after `fetchLiveState` and before `apply.Apply`, using the volume list already fetched there.** Makes the atomic guarantee feasible. Closes Major-6.
- **DEC-022 — No-op requires both region and effective `numReplicas` to equal the live single-entry `multiRegionConfig`.** Default-placed services (empty live map) are always treated as a change (one write). Avoids needing to resolve the default region's name. Closes Major-1, Major-2.
- **DEC-023 — On update, the no-op check is evaluated before the volume block**, so an already-satisfied request on a volume-bound service succeeds instead of erroring. Closes Gap-4.
- **DEC-024 — The live `multiRegionConfig` map is threaded through the diff/compare/pre-flight path, not just the write path.** `LiveService`/`LiveDeployConfig` carry `MultiRegion map[string]int` (from `fetchLiveState`); `compareDeployConfig` derives the effective replica count for a region-placed service from `MultiRegion[Region]` (not flat `numReplicas`); the apply pre-flight and collapse guard read the same map. Without this, region-placed services show perpetual replica drift and never reach a no-op. Closes NEW-CRIT-1 and NEW-MAJ-1 of REVIEW-002; finally closes REVIEW-001 Minor-2.
- **DEC-025 — For a `--force` multi-region collapse to region X with no explicit `--replicas`, the effective N is X's live count when X is among the live regions, else 1** (never the sum or another region's count). Extends DEC-016's "preserve what you can" to the multi-region case; the manifest path (DEC-020) follows the same rule. Closes NEW-MAJ-2.

## Appendix B — Open questions

*(all closed)*

- Q-001 → DEC-010/DEC-016. Q-002 (manifest field placement) → DEC-002 / REQ-CFG-001 (`deploy.region`).
  Q-003 → DEC-008. Q-004 → DEC-009. Q-005 → DEC-012. Q-006 → DEC-013.
- Q-007 (diff rendering): resolved by REQ-APL-003 (path-agnostic renderer, no new code).
- Q-008 → DEC-011. Q-009 (which doc sections) → REQ-DOC-001/002/003. Q-010 → DEC-014.
- REVIEW-001 items: Critical-1→DEC-017, Critical-2→DEC-018, Critical-3→DEC-015; Major-1/2→DEC-022,
  Major-3→DEC-016, Major-4→DEC-019, Major-5→DEC-020, Major-6→DEC-021; Gap-1→REQ-APL-009,
  Gap-2→REQ-CMD-001+verification, Gap-3→REQ-CMD-003, Gap-4→DEC-023; XC-1→Appendix E, XC-2→REQ-API-005.
- REVIEW-002 items: NEW-CRIT-1→DEC-024/REQ-APL-010, NEW-MAJ-1→DEC-024/REQ-APL-004+008/Appendix E,
  NEW-MAJ-2→DEC-025/REQ-CMD-002/REQ-CFG-005; NEW-MIN-1→REQ-API-003 (reworded caller-enforced),
  NEW-MIN-2→REQ-APL-009 (scope note), NEW-MIN-3→REQ-VOL-001 (force ≠ volume bypass).

## Appendix E — Resource / field reference

**`types.Region`** (new): `Name string`, `Country string`, `Location string`, `Metal bool`.

**`config.DeployConfig.Region`** (new): `Region string \`yaml:"region,omitempty"\`` — canonical
region name; empty = unmanaged (leave live alone). No `$env()` expansion. Not name-validated
offline.

**Live-state fields** (new; XC-1 of REVIEW-001):
- `types.ServiceDetail.MultiRegion map[string]int` — full live placement (region → numReplicas);
  empty = default placement.
- `types.ServiceDetail.Region string` — the sole region when `len(MultiRegion)==1`, else empty
  (derived convenience for diff).
- `diff.LiveDeployConfig.Region string` — populated in `cmd/apply.go:fetchLiveState`
  (`ls.Deploy.Region = svc.Region`). Diff/compare uses the single-region string for the
  `deploy.region` field diff.
- `diff.LiveDeployConfig.MultiRegion map[string]int` (new; REQ-APL-010) — populated in
  `fetchLiveState` (`ls.Deploy.MultiRegion = svc.MultiRegion`). Used by `compareDeployConfig`
  to derive the effective per-region replica count (`MultiRegion[Region]`) instead of the flat
  `numReplicas`, and by the apply pre-flight/collapse guard (REQ-APL-004/008) to read map
  cardinality before any mutation.

**New flag**: `--force` on `update service` and `apply` — overrides the multi-region collapse
guard (DEC-015) only. No other effect.

**`multiRegionConfig` write shape** (single region): `{ "<name>": { "numReplicas": <N≥1> } }`.

**Touch list** (from integration map): `config.DeployConfig` → `config.Validate` →
`diff.LiveDeployConfig` + `diff.compareDeployConfig`/`deployCreateFields`/`buildDeleteChange`
→ `apply.buildDeployConfigFromConfig`/`buildDeployConfigUpdate` (+ call sites) →
`cmd/apply.go:fetchLiveState` → `api.UpdateServiceInstanceConfig` + `APIClient` iface +
hand-written `MockClient` → service GraphQL queries + `serviceInstanceNode` + `toServiceDetail`
+ `types.ServiceDetail` → `cmd/create_service.go`/`update_service.go` (+ `hasDeployConfigFlags`)
→ new `cmd/get_regions.go` + `api/regions.go` → `docs/*` + `make gen`.
