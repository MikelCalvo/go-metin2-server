# Hermetic Import-Export Drill SQLite Seeded Tree — 2026-08-27

## Objective

Extend the already-shipped empty-payload hermetic
`import-export-drill` SQLite proof with one cohesive non-empty seeded retained
export tree so the printed `/bin/sh` script is proven to drive confirmation-gated
`import-export` for every tip kind against real FK-linked durable rows — without
inventing upsert policy, registering a stock production driver, auto-running the
printer from CLI/contrib/cron, or exposing a daemon mutation route.

## Why now

- [Hermetic import-export drill SQLite execution proof](2026-08-27-hermetic-import-export-drill-sqlite-execution-proof.md)
  explicitly deferred “non-empty seeded import payloads in this hermetic drill”.
- Per-package `SQLiteHarness*Import` tests already own seeded inserts, but they
  do not exercise the printed PATH wiring + tip-kind ordering + DSN-env
  indirection together on one shared database.
- The PvE durable-state / migration-window vertical needs confidence that a
  retained quarantine tree with real roster → item/point/quest/safebox/ground
  parent FKs (plus independent ticket/template/static tips) survives the
  operator runbook script end-to-end.

## Contract frozen by this slice

1. Keep the existing empty-payload hermetic proof untouched.
2. Add a second build-tagged test under `internal/migratecli`
   (`//go:build sqlite_harness`) that:
   - builds `./cmd/metin2-migrate` with `-tags=sqlite_harness`;
   - materializes one retained export tree with shape-valid non-empty
     `quarantine.json` for every `exportQuarantineKinds` entry, using public
     `Export*` helpers (or equivalent canonical JSON) and shared IDs;
   - applies the embedded catalog to tip on a temp SQLite file;
   - prints `import-export-drill` with `--driver sqlite`, absolute
     `--export-tree`, and `--i-confirm-print-sql-import-drill`;
   - executes the printed script under `/bin/sh` with `PATH` including the
     tagged binary and `METIN2_IMPORT_DSN` set to the temp DB DSN.
3. Shared seed identity (one account / one character is enough):
   - login `Alpha` / character id `11` / name `AlphaWar`;
   - account id = `stableRosterAccountID("alpha")` (`776349473104011307`);
   - child tip rows reference character `11` where FK-required;
   - tickets / templates / static-actor content remain independent of roster
     rows except for intentional shared identity fields.
4. After execution every tip kind subdirectory contains `import-result.json`
   with non-zero expected count markers, for example:
   - roster `account_count: 1`
   - item `inventory_item_count: 1`
   - points `point_row_count: 255`
   - myshop unit-prices `price_row_count: 2`
   - quest `flag_count: 1`
   - safebox `password_count: 1`
   - ticket `ticket_count: 1`
   - template `template_count: 1`
   - static `static_actor_count: 1`
   - ground `ground_item_count: 1`
5. Focused `SELECT` assertions prove durable rows landed (accounts/characters,
   inventory/equipment/quickslots, points, myshop unit-prices, quest flags,
   safebox password/item, ticket, template, interaction/static actor, ground
   item). Non-empty tip-`0023` seeding is owned by [seeded myshop unit-prices
   tip sync](2026-08-29-seeded-myshop-unit-prices-import-export-drill.md).
   Non-empty tip-`0003`+`0024` instance-socket seeding is owned by [seeded item
   instance-sockets tip sync](2026-08-30-seeded-item-instance-sockets-import-export-drill.md).
   Non-empty tip-`0003`+`0027` instance-attribute seeding is owned by [seeded item
   instance-attributes tip sync](2026-08-31-seeded-item-instance-attributes-import-export-drill.md).
   Non-empty tip-`0009`+`0021`/`0022` refine keep-on-fail / fail-result-vnum
   seeding is owned by [seeded item-template refine fields tip
   sync](2026-08-30-seeded-item-template-refine-fields-import-export-drill.md).
   Non-empty tip-`0015`+`0025` safebox cell instance-socket seeding is owned by
   [seeded safebox cell instance-sockets tip
   sync](2026-08-30-seeded-safebox-cell-instance-sockets-import-export-drill.md).
   Non-empty tip-`0010`+`0026` pending ground instance-socket seeding is owned by
   [seeded ground-item instance-sockets tip
   sync](2026-08-30-seeded-ground-item-instance-sockets-import-export-drill.md).
6. Printed script and import-result bodies still omit concrete DSN embedding
   beyond env-var indirection.
7. Untagged `go test ./internal/migratecli` stays free of the SQLite dependency.
8. Docs mark the deferred seeded-tree follow-up done on the empty hermetic plan
   and Track E / migration-contract tips; upsert / auto-run / stock driver remain
   explicitly deferred.

## What this is not yet

- automatic / scheduled execution of the printed import script
- upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- DB-backed runtime repositories replacing FileStores
- loopback ops mutation endpoint
- remote admin, secrets in git, metrics/tracing

## Likely files to change

- `internal/migratecli/import_export_drill_sqlite_harness_test.go`
- `docs/plans/2026-08-27-hermetic-import-export-drill-sqlite-execution-proof.md`
- `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md` (Track E tip)
- `docs/development.md` (brief pointer)
- this plan

## TDD and validation

Focused coverage under `//go:build sqlite_harness`:

- seeded retained tree + printed-script `/bin/sh` execution
- all nine tip kinds write non-zero `import-result.json` markers
- FK-linked child tips succeed after roster import order
- apply reaches catalog tip before imports
- printer stdout never embeds a concrete DSN value
- SELECT round-trip of shared seed rows

Validation for this slice:

```bash
go test -tags=sqlite_harness ./internal/migratecli -run 'ImportExportDrillSQLite' -count=1
gofmt -l internal/migratecli/import_export_drill_sqlite_harness_test.go
git diff --check
```

Also keep untagged package green:

```bash
go test ./internal/migratecli -run 'ImportExportDrill|ContribLabRetentionGC' -count=1
```

## Exit criteria

- seeded hermetic printed-script SQLite proof is green under
  `-tags=sqlite_harness`
- prior deferred seeded-tree follow-up marked done
- empty hermetic proof remains green and unchanged in intent
- stock binaries remain free of a registered production driver
- matching hermetic `import-result-status.json` redirect proof is owned by [hermetic import-export status SQLite proof](2026-08-28-hermetic-import-export-status-sqlite.md)
- upsert / auto-run remain explicitly deferred

## Anti-goals / ordering constraints

- Do not auto-run printed import commands from CLI / contrib / cron.
- Do not embed DSN values in printer stdout or contrib notes.
- Do not register a production driver in stock binaries.
- Do not invent upsert / merge policy.
- Do not push `origin/main`; push only `origin/lane/persistence`.
