# Ops Docs Tip Sync After Catalog Tip 0026 — 2026-08-30

## Objective

Close the remaining Track E operator-docs gap after GREEN
`0026_bootstrap_ground_item_instance_sockets` and the seeded hermetic
tip-`0010`+`0026` import-export drill: make development / debugging / migration
runbook prose tell the same story as the catalog tip and harness, without new
schema, upsert, or attributes-on-instance RED.

## Why now

- Package-level SQL import already fail-closes when tip-`0010` is present without
  additive ledger `26`, and the seeded drill proves non-zero pending ground
  sockets through the printed PATH + tip-order SQLite path — see
  [seeded ground-item instance-sockets tip sync](2026-08-30-seeded-ground-item-instance-sockets-import-export-drill.md).
- `docs/development.md` seeded-drill inventory still lists tip-`0023`,
  tip-`0003`+`0024`, tip-`0009`+`0021`/`0022`, and tip-`0015`+`0025` but omits
  tip-`0010`+`0026`.
- `docs/debugging-and-profiling.md` tip-`0009` quarantine already names
  “SQL import still requires tip + additive ledger …”; tip-`0003` / tip-`0010` /
  tip-`0015` quarantine prose describe presence-aware sockets but lack that
  fail-closed import preflight sentence.
- `docs/workflow/migration-apply-runbook.md` cites the seeded item/safebox tip
  sync plans beside the `0010` harness text but does not point at the seeded
  ground sockets tip-sync plan.

Those contradictions are production-ops hazards for export → quarantine →
import runbooks after catalog tip `26`.

## Contract frozen by this slice

1. Operator docs name tip-`0010`+`0026` presence-aware pending ground sockets in
   the seeded hermetic import-export-drill inventory beside the already-owned
   tip-`0003`+`0024` / tip-`0015`+`0025` / tip-`0009`+`0021`/`0022` seeds.
2. Loopback quarantine docs for tip-`0003`, tip-`0010`, and tip-`0015` state that
   SQL import still requires the export tip plus the matching additive ledger
   (`24` / `26` / `25`) before INSERT.
3. Migration apply runbook see-also list includes the seeded ground sockets tip
   sync plan.
4. Track E / migration-contract next-slice pointers mark this ops tip-sync done.
   FileStore-first attributes-on-instance GREEN is owned on `lane/items`; durable
   ground/safebox attribute rematerialize and tip SQL companions stay deferred.
5. No new migration tip, no retip of export identities `3` / `10` / `15`, no
   upsert / stock production driver / remote admin / README churn.

## What this is not yet

- durable ground/safebox attributes-on-instance rematerialize / tip SQL companions
  (FileStore-first encode GREEN already owned — see
  [attributes-on-instance FileStore + encode GREEN](2026-08-30-attributes-on-instance-filestore-encode-green.md))
- upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- DB-backed live ground / inventory / safebox repositories
- `ITEM_GROUND_ADD` wire sockets
- remote admin / daemon mutation route / secrets in git

## Likely files to change

- `docs/development.md`
- `docs/debugging-and-profiling.md`
- `docs/workflow/migration-apply-runbook.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md` (Track E tip pointer)
- `docs/plans/2026-08-09-db-migration-contract.md` (next-slice pointer)
- `docs/plans/2026-08-30-seeded-ground-item-instance-sockets-import-export-drill.md`
  (status cross-link)
- this plan

## TDD and validation

Docs-only tip sync (no Go production changes):

```bash
git diff --check
# optional sanity: confirm seeded drill still green if local sqlite_harness is available
go test -tags=sqlite_harness ./internal/migratecli -run 'ImportExportDrillSQLite' -count=1
```

## Status

Docs tip-sync on `lane/persistence`. Schema / seeded drill / import gates remain
already GREEN. FileStore-first attributes-on-instance GREEN plus additive
`0027`/`0028`/`0029` SQL companions are now owned; operator-docs tip-sync for
those attribute companions (without claiming tip-`0010`+`0029` seeded drill) is
owned by
[ops docs tip sync after catalog tips 0027/0028/0029](2026-08-31-ops-docs-0027-0028-0029-attributes-tip-sync.md).
