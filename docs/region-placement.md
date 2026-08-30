# Region Placement

## Overview

railctl can pin a Railway service to a specific region, imperatively and
declaratively, and list the available regions.

```bash
railctl get regions                                              # list region names
railctl create service api --image node:20 --region europe-west4 -p my-project
railctl update service api --region us-west2 -p my-project -e production
```

Under the hood, region placement is Railway's `multiRegionConfig` (a map of
region ID → replica count). railctl writes it with the `environmentPatchCommit`
mutation and reads current placement from the latest deployment's metadata. For
project tokens the required `project-access-token` header is used automatically.

## Regions

`railctl get regions` prints the short region **names** you pass to `--region` /
`deploy.region`. The list is shipped with the CLI (Railway's `regions` query is
not available to project tokens), so it works under any token scope:

| Name (use this)   | Full region ID           | Location               |
| ----------------- | ------------------------ | ---------------------- |
| `us-west2`        | `us-west2`               | California, USA        |
| `us-east4`        | `us-east4-eqdc4a`        | Virginia, USA          |
| `europe-west4`    | `europe-west4-drams3a`   | Amsterdam, Netherlands |
| `asia-southeast1` | `asia-southeast1-eqsg3a` | Singapore              |

`--region` accepts the short name or the full ID; the full ID is what's used on
the wire (the `multiRegionConfig` key). Output formats: `table` (NAME, LOCATION),
`wide` (+ COUNTRY), `json`, `yaml`. If Railway adds a region, the CLI's shipped
list must be updated.

## Imperative: create / update service

- `--region <name>` on `create service` and `update service`. The value (short
  name or full ID) is validated against the region list; an unknown value errors
  with the valid names.
- **create** places the service in exactly the target region (defaults to 1
  replica, or `--replicas`) — railctl removes Railway's implicit default so the
  service is single-region from the start.
- **update** reads the live placement and writes a **single region** (target set,
  every other current region removed), preserving the current replica count
  unless `--replicas` is given. It triggers a redeploy (unless `--skip-deployment`)
  and is a **no-op** when the service is already in the target region with the
  same replica count.
- A bare `--replicas` on a region-placed service updates that region's count.

### Default region for new services

`create service` falls back to the `RAILCTL_REGION` environment variable when
`--region` is omitted (flag wins). Create-only — `update service` always requires
an explicit `--region`.

```bash
export RAILCTL_REGION=europe-west4
railctl create service api --image node:20 -p my-project
```

## Declarative: `deploy.region`

```yaml
services:
  - name: api
    image: node:20-alpine
    deploy:
      region: us-west2   # short name or full region ID; pin to one region
      replicas: 2
```

- Omitted `deploy.region` is unmanaged — `apply` leaves live placement alone.
- Region values are literal (no `$env()`); short name or full ID, validated
  against the shipped list and always written as the full ID (a literal short
  name would hit Railway's legacy non-metal region and break volume
  migrations). `apply`/`diff` reconcile placement (read from deployment meta)
  and stay idempotent.
- `RAILCTL_REGION` does not affect `apply` (the manifest is the source of truth).

## Volumes: migration on region change

Railway auto-migrates an attached volume when its service changes region: the
deployment is held until the volume is ready in the new region, and the service
is **down** while the migration runs (longer for larger volumes). To protect
against accidental downtime, `update service --region` and `apply` refuse the
change on a volume-carrying service unless `--force` acknowledges the migration.
(Railway does not support migrating between metal and non-metal regions.)

## Multi-region services and `--force`

A service can be placed in more than one region (via the dashboard/API, or by a
prior create). Setting a single `--region` would drop the others, so railctl
refuses unless `--force` (`update service --force` / `apply --force`), naming the
regions being dropped. One `--force` acknowledges both consequences (collapse
and volume migration) when they coincide.

## Not supported

- Multi-region fan-out in one command (single region only).
- A live `regions` query (project tokens can't; the list is shipped).
- A current-region column in `describe` / `get services` human output (placement
  is still read for diff/apply/no-op).

## Under the hood

| Concern | Mechanism |
| --- | --- |
| List regions | Shipped list (`internal/api/regions.go`) |
| Write placement | `environmentPatchCommit` → `services.<id>.deploy.multiRegionConfig` (region ID → `{numReplicas}`; `null` removes) |
| Read placement | latest deployment `meta.serviceManifest.deploy.multiRegionConfig` |
| Project-token auth | `project-access-token` header (automatic on token-type detection) |

## Testing

```bash
go test ./internal/...
go vet -tags e2e ./tests/e2e/...
# live (project token): RAILWAY_PROJECT_TOKEN=<tok> go test -tags e2e -run 'TestRegions|TestApplyDiff_Region' ./tests/e2e/project/...
```
