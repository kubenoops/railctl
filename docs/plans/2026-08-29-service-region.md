# Plan: service region placement

**Design:** `docs/designs/2026-08-29-service-region.md` (Status: Ready, iteration 5)
**Branch:** `feat/service-region`
**Execution:** per task → implement → compile/verify → unit-test → review. Tasks are ordered
bottom-up (types/API → config → diff → apply → commands → docs → e2e); each leaves the tree
building and the unit suite green. Live e2e (real tokens) runs after the e2e task, by the
orchestrator. RES-1 of REVIEW-003 is folded into Task 4.

## Verification commands

```bash
cd /home/omar/src/github.com/kubenoops/railctl
make build                       # every task
go test ./internal/...           # unit/integration suite stays green
go vet -tags e2e ./tests/e2e/... # after the e2e task
make gen-check                   # after the docs task (embedded skill copy in sync)
```

Each task lists the REQs it satisfies (see SPEC §5) so review is "does this match REQ-X?".

---

## Task 1 — types + API read/write core  (REQ-API-001..005, DEC-001/007/017)

- `internal/types/project.go`: add `Region` type (`Name, Country, Location string; Metal bool`);
  add `ServiceDetail.MultiRegion map[string]int` and derived `ServiceDetail.Region string`
  (sole key when `len==1`, else "").
- `internal/api/regions.go` (new): `ListRegions() ([]types.Region, error)` — the `regions`
  query (name/country/location/railwayMetal).
- `internal/api/services.go`:
  - Widen `UpdateServiceInstanceConfig(..., replicas *int, ..., region *string)`. `region!=nil`
    ⇒ `input["multiRegionConfig"] = {*region: {numReplicas: <resolved N>}}` and **omit**
    top-level `numReplicas`; `region==nil` ⇒ today's flat path (REQ-API-002/003/004).
  - Add `multiRegionConfig { region numReplicas }` selection to `listServicesQuery` and
    `getServiceQuery`; add to `serviceInstanceNode`; parse in `toServiceDetail` → populate
    `MultiRegion` + derived `Region` (REQ-API-005).
- `internal/api/interface.go`: add `ListRegions`; update `UpdateServiceInstanceConfig` signature.
- `internal/api/mock.go` (hand-written): add `ListRegionsFunc` + method; update
  `UpdateServiceInstanceConfigFunc` type + dispatch to the new signature.
- Tests (`internal/api/*_test.go`): REQ-API-001 (regions parse), 002 (multiRegionConfig body,
  no flat numReplicas), 003 (flat path when region nil), 005 (2-entry map → MultiRegion both,
  Region ""; 1-entry → Region set). httptest capture pattern.
- **Verify:** `make build && go test ./internal/api/...`

## Task 2 — `get regions` command  (REQ-CMD-007, DEC-004)

- `internal/cmd/get_regions.go` (new): mirror `get_replicas.go`; workspace/project-scoped
  (no `NeedService`); table (NAME, LOCATION) / wide (+COUNTRY, METAL) / json / yaml; register
  on `getCmd`.
- Test: REQ-CMD-007 (json valid, ≥1 region; table headers).
- **Verify:** `make build && go test ./internal/cmd/... && ./railctl get regions --help`

## Task 3 — imperative `create`/`update service --region` + guards  (REQ-CMD-001..009, REQ-VOL-001/002, DEC-003/005/010/015/016/019/022/023/025)

- Shared helpers (e.g. `internal/cmd/region.go`): `resolveRegion(client, value)` (validate
  vs `ListRegions`, case-insensitive → canonical; unknown → error listing names — REQ-CMD-003);
  `ensureNoVolumeForRegionChange(...)` (REQ-VOL-001); `resolveEffectiveReplicas(...)` and the
  collapse-guard check reading `ServiceDetail.MultiRegion` (REQ-CMD-009/DEC-025).
- `create_service.go`: `--region` flag; `RAILCTL_REGION` fallback (REQ-CMD-005); validate
  before create (REQ-CMD-004); add region to `hasDeployConfigFlags` + `applyDeployConfig`;
  effective N defaults to 1 (REQ-CMD-002). No volume check (REQ-VOL-002).
- `update_service.go`: `--region` + `--force` flags. Order: read live placement → **no-op
  check** (region+effective N equal → return, DEC-022/023) → **volume block** (REQ-VOL-001;
  `--force` never bypasses) → **collapse guard** (>1 live region → refuse unless `--force`,
  REQ-CMD-009) → resolve effective N (preserve live for target region, REQ-CMD-002/DEC-016) →
  write via `multiRegionConfig` → redeploy unless `--skip-deployment` (REQ-CMD-006). Bare
  `--replicas` on a region-placed service routes through the map (REQ-CMD-008/DEC-019). Print
  result line.
- Tests: REQ-CMD-001..009, REQ-VOL-001/002 (mock-based integration; assert write shape,
  no-op, guard errors, force behaviour, env fallback, fail-fast on unknown region).
- **Verify:** `make build && go test ./internal/cmd/...`

## Task 4 — config manifest field  (REQ-CFG-001..005, DEC-013/020/025; RES-1 groundwork)

- `internal/config/config.go`: add `DeployConfig.Region string \`yaml:"region,omitempty"\``.
  `Validate` stays API-free (format-only; no name check — REQ-CFG-002). No `$env()` expansion
  for region (REQ-CFG-003). Legacy conversion leaves it unset (REQ-CFG-004). Effective-N
  default for `region` set + `replicas` omitted resolved at apply time (REQ-CFG-005/DEC-020) —
  config only carries the literal.
- `config_test.go`: REQ-CFG-001..004 (+ the `$env` literal case).
- **Verify:** `go test ./internal/config/...`

## Task 5 — diff / compare  (REQ-APL-002/003/009/010, DEC-008/024, RES-1)

- `internal/diff/diff.go`: `LiveDeployConfig` gains `Region string` + `MultiRegion map[string]int`.
  `compareDeployConfig`: add a `deploy.region` string diff (zero-value = unmanaged → DEC-008);
  derive the effective replica compare from `MultiRegion[Region]` for a single-region live
  service (REQ-APL-010/DEC-024); when live has >1 region and desired sets no region, emit **no**
  `deploy.replicas` drift (RES-1). Add region to `deployCreateFields` and `buildDeleteChange`
  (REQ-APL-009, single-region scope). Renderer is path-agnostic (REQ-APL-003 — no change).
- `diff_test.go`: REQ-APL-002 (omit → no diff), 003 (region drift renders), 009 (delete diff),
  010 (per-region replica compare → idempotent; >1-region no spurious drift).
- **Verify:** `go test ./internal/diff/...`

## Task 6 — apply (tuple resolution + atomic pre-flight)  (REQ-APL-001/004/005/006/007/008, DEC-018/021)

- `internal/cmd/apply.go`: `fetchLiveState` copies `Region` + `MultiRegion` into
  `LiveDeployConfig`. Add the **pre-flight** after `fetchLiveState`, before `apply.Apply`:
  scan every `ResourceChange` touching `deploy.region` against (a) the already-listed volume set
  → volume block (REQ-APL-004), (b) live `MultiRegion` cardinality → collapse guard unless
  `--force` (REQ-APL-008). Fail the whole apply before any mutation.
- `internal/apply/apply.go`: `buildDeployConfigFromConfig`/`buildDeployConfigUpdate` gain a
  region return; resolve the `(region, numReplicas)` tuple from `configMap[name].Deploy` + live
  state, forcing the map path for any managed region even on a one-field ChangeSet
  (REQ-APL-007/DEC-018). Widened `UpdateServiceInstanceConfig` call sites. Region change →
  deploy (REQ-APL-005). `RAILCTL_REGION` not consulted (REQ-APL-006). `apply --force` flag.
- `apply_test.go`: REQ-APL-001/004/005/006/007/008.
- **Verify:** `go test ./internal/apply/... && go test ./internal/...`

## Task 7 — docs + skill + gen  (REQ-DOC-001/002/003)

- `docs/declarative-config.md`: `deploy.region` row + example (REQ-DOC-001).
- `docs/railctl-skill.md`: `--region`, `get regions`, volume constraint, `--force` collapse
  (REQ-DOC-002); then `make gen` to refresh `internal/skill/railctl-skill.md`.
- `docs/region-placement.md` (new): behaviour, volume constraint, `--force`, `RAILCTL_REGION`
  scope (REQ-DOC-003).
- **Verify:** `make gen-check` (no diff), review.

## Task 8 — e2e (project-scoped)  (REQ-CMD-007/009, REQ-VOL-001, REQ-APL-001/004/005/008)

- `tests/e2e/project/`: `create_with_region` in `services_test.go`; `update_region` (+ no-op
  re-run) in `update_service_test.go`; `deploy.region` create/update/idempotent in
  `apply_diff_test.go`; a `get regions` test (mirror `replicas_test.go`); volume-block and
  collapse-guard/`--force` cases. Discover a real region name via `get regions -o json`.
- **Verify:** `go vet -tags e2e ./tests/e2e/...` (compile); live run by orchestrator.

## Live verification (orchestrator, after Task 8)

Against real tokens: `get regions`; create a throwaway service with `--region`; move it
(`update --region`, confirm redeploy) and re-run to confirm no-op; attach a volume then confirm
`--region` is refused; declaratively set `deploy.region` via `apply`, then `diff` shows clean.
Clean up throwaway resources.

## Out of plan (SPEC non-goals)

Multi-region fan-out, volume data migration, current-region display column (DEC-002/014).
