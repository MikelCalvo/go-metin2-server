# Ops Docs Tip Sync After Catalog Tips 0027/0028/0029 — 2026-08-31

## Objective

Close the remaining Track E operator-docs gap after GREEN
`0027_character_item_instance_attributes`,
`0028_character_safebox_item_instance_attributes`, and
`0029_bootstrap_ground_item_instance_attributes` (plus the already-landed
seeded tip-`0003`+`0027` / tip-`0015`+`0028` hermetic drills): make
development / debugging / migration runbook prose tell the same story as the
catalog tip and harness, without new schema, upsert, remote admin, or claiming
and (now landed) tip-`0010`+`0029` seeded drill ownership noted as a later follow-on at write time.

## Why now

- Additive `0027` / `0028` / `0029` schema + SQL import preflight already landed
  on `main` / `lane/persistence`.
- Seeded hermetic tip-`0003`+`0027` and tip-`0015`+`0028` attribute drills already
  landed (`feat(db): seed tip-0003…`, `feat(db): seed tip-0015…`).
- Tip-`0010`+`0029` seeded hermetic drill was still deferred on `origin/lane/items`
  (`08154242`) when this ops tip-sync was authored and was **not** claimed here;
  it later landed via [seeded ground-item instance-attributes tip sync](2026-08-31-seeded-ground-item-instance-attributes-import-export-drill.md).
- Operator docs still lag:
  - `docs/development.md` seeded-drill inventory still ends at sockets-only
    tip-`0003`+`0024` / tip-`0015`+`0025` / tip-`0010`+`0026`.
  - `docs/development.md` DB-preflight summary still names safebox/ground import
    companions as sockets-only (`0025` / `0026`) even though schema-preflight for
    ground already requires `26`+`29`.
  - `docs/debugging-and-profiling.md` tip-`0015` quarantine SQL-import sentence
    still requires only additive ledger `25`.
  - `docs/workflow/migration-apply-runbook.md` ground-item harness clause still
    names only additive `0026` sockets beside tip-`0010`.

Those contradictions are production-ops hazards for export → quarantine →
import runbooks after catalog tip `29`.

## Contract frozen by this slice

1. Operator docs name tip-`0003`+`0027` and tip-`0015`+`0028` presence-aware
   instance attributes in the seeded hermetic import-export-drill inventory
   beside the already-owned socket seeds. Tip-`0010`+`0029` seeded drill stayed
   explicitly deferred in this ops tip-sync slice (later owned separately).
2. Loopback tip-`0015` quarantine docs state that SQL import still requires the
   export tip plus additive ledger `25` / sockets **and** `28` / attributes
   before INSERT, and the quarantine contract names presence-aware attributes.
3. Migration apply runbook ground-item harness clause names additive `0026`
   sockets **and** additive `0029` attributes; see-also lists include the
   attributes SQL / seeded tip-sync plans already on tree.
4. Development DB-preflight summary clauses for safebox/ground import match the
   full ledger preflight (`0015`+`0025`+`0028`, `0010`+`0026`+`0029`).
5. Soft catalog sentences for standalone `0025` / `0026` import-preflight align
   with the later `0028` / `0029` full-preflight sentences.
6. No new migration tip, no retip of export identities `3` / `10` / `15`, no
   upsert / stock production driver / remote admin / README churn; tip-`0010`+`0029`
   seeded drill ownership stayed out of this ops tip-sync slice and later landed via
   [seeded ground-item instance-attributes tip sync](2026-08-31-seeded-ground-item-instance-attributes-import-export-drill.md).

## What this is not yet

- tip-`0010`+`0029` seeded hermetic import-export-drill (out of scope for this
  ops tip-sync slice; later owned by
  [seeded ground-item instance-attributes tip sync](2026-08-31-seeded-ground-item-instance-attributes-import-export-drill.md))
- upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- DB-backed live inventory / safebox / ground repositories
- `ITEM_GROUND_ADD` wire attributes
- remote admin / daemon mutation route / secrets in git

## Likely files to change

- `docs/development.md`
- `docs/debugging-and-profiling.md`
- `docs/workflow/migration-apply-runbook.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md` (Track E tip pointer)
- `docs/plans/2026-08-09-db-migration-contract.md` (next-slice pointer)
- `docs/plans/2026-08-30-ops-docs-0026-ground-sockets-tip-sync.md` (status cross-link)
- `docs/plans/2026-08-31-bootstrap-ground-item-instance-attributes-sql-additive.md`
  (follow-on pointer: seeded drill still deferred; ops tip-sync owned here)
- this plan

## TDD and validation

Docs-only tip sync (no Go production changes):

```bash
git diff --check
# optional sanity if local sqlite_harness is available:
go test -tags=sqlite_harness ./internal/migratecli -run 'ImportExportDrillSQLite' -count=1
```

## Status

GREEN on `lane/persistence`: operator docs + Track E / migration-contract
pointers now match catalog tips `0027`/`0028`/`0029` and the landed tip-`0003`+`0027`
/ tip-`0015`+`0028` seeded drills. Tip-`0010`+`0029` seeded drill was deferred in this
slice and later landed via [seeded ground-item instance-attributes tip sync](2026-08-31-seeded-ground-item-instance-attributes-import-export-drill.md).
