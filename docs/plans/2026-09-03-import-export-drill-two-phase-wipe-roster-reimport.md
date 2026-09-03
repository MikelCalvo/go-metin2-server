# Import-export-drill two-phase wipe→roster→reimport — 2026-09-03

## Objective

Close the deferred Track E operator gap after hermetic opt-in scoped-replace:
seeded full-tree re-backfill that includes tip-`0002` roster must be printable
as a confirmation-gated `/bin/sh` script without inventing cascade delete inside
any `Import*` primitive and without registering a stock production driver.

## Why now

- Tip vocabulary scoped-replace GREEN is complete.
- Opt-in scoped-replace drill printer + hermetic SQLite proofs are owned
  ([import-export-drill opt-in scoped-replace](2026-09-03-import-export-drill-opt-in-scoped-replace.md),
  [hermetic import-export-drill opt-in scoped-replace SQLite](2026-09-03-hermetic-import-export-drill-opt-in-scoped-replace-sqlite.md)).
- Those proofs honestly froze that a **seeded single-pass** including tip-`0002`
  still fails closed while character-FK child tip rows remain, because child tip
  scoped replace re-inserts FK dependents before roster delete.
- Operators currently hand-edit / omit roster. That is the remaining lab
  re-backfill friction before production-engine selection.

## Contract frozen by this slice

### 1. Offline wipe-scope synthesizer

```bash
metin2-migrate synthesize-wipe-export \
  --kind <kind> \
  --export <quarantine.json|->
```

Supported `--kind` values only:

- `character-item-state`
- `character-point-state`
- `character-myshop-unit-prices`
- `character-quest-state`
- `character-safebox-state`
- `bootstrap-ground-item-state`

Behavior:

1. Offline only: never opens a database, never embeds a DSN, never mutates the
   retained source file.
2. Accepts either:
   - bare migration-shaped export JSON, or
   - wrapped `quarantine-export` stdout (`{"summary":...,"export":...}`).
3. Re-runs the existing `Quarantine*` contract, then emits a **bare** wipe export
   with the same migration_version/name, sorted declared scope ids
   (`character_ids` or `vids`), and empty row arrays.
4. Fail-closed when the derived wipe scope is empty (empty-scope replace would
   be a silent no-op and leave FK dependents in place).
5. Reject unsupported kinds (roster / ticket / template / static) as usage.

### 2. import-export accepts wrapped quarantine JSON

`metin2-migrate import-export --export ...` must accept the same bare-or-wrapped
shapes as the synthesizer so retained trees produced by
`export-quarantine-drill` → `quarantine-export` are directly importable without
manual `jq '.export'` unwrapping. Bare exports remain the preferred artifact
shape for synthesizer stdout / wipe files.

### 3. Two-phase drill printer flag

```bash
metin2-migrate import-export-drill \
  --export-tree <absolute-retained-tree> \
  --driver <database/sql-driver-name> \
  [--dsn-env METIN2_IMPORT_DSN] \
  --i-confirm-print-sql-import-drill \
  --i-confirm-print-two-phase-wipe-roster-reimport
```

Rules:

1. Requires `--i-confirm-print-sql-import-drill`.
2. Implies scoped-replace printing (does **not** require a separate
   `--i-confirm-print-scoped-replace`, but may be combined with it).
3. Still print-only: never executes imports, never opens a database, never
   embeds a DSN value.
4. Printed `/bin/sh` phases, in order:
   1. **Synthesize wipe artifacts** for each phase-1 kind into
      `$EXPORT_TREE/<kind>/wipe-quarantine.json`.
   2. **Wipe** those kinds via
      `import-export ... --export "$EXPORT_TREE/<kind>/wipe-quarantine.json"
      --i-confirm-sql-import --i-confirm-scoped-replace`.
   3. **Roster replace** tip-`0002` from retained
      `$EXPORT_TREE/account-character-roster/quarantine.json`.
   4. **Reimport** every tip kind in `exportQuarantineKinds` **except**
      `account-character-roster` from retained `quarantine.json` with scoped
      replace.
5. Phase-1 wipe kinds / scope keys:
   - `character-item-state` → `character_ids`
   - `character-point-state` → `character_ids`
   - `character-myshop-unit-prices` → `character_ids`
   - `character-quest-state` → `character_ids`
   - `character-safebox-state` → `character_ids`
   - `bootstrap-ground-item-state` → `vids`
6. Ticket / template / static are **not** wiped in phase 1 (no `characters(id)`
   FK). They reimport in phase 4 only.
7. Default insert-only printer and single-pass scoped-replace printer remain
   unchanged.
8. Contrib helper may forward the same opt-in only when
   `METIN2_IMPORT_PRINT_TWO_PHASE_WIPE_ROSTER=YES` (default remains
   insert-only / single-pass scoped-replace when that older gate is set).

## What this is not yet

- automatic / scheduled execution of the printed script
- cascade-delete inside tip-`0002` `ImportAccountCharacterRoster`
- stock production driver registration
- DB-backed runtime repositories / daemon mutation routes
- remote admin, secrets in git, metrics/tracing

## Likely files to change

- `internal/migratecli/synthesize_wipe_export.go` (new)
- `internal/migratecli/synthesize_wipe_export_test.go` (new)
- `internal/migratecli/import_export.go` (wrapped quarantine accept)
- `internal/migratecli/import_export_test.go`
- `internal/migratecli/import_export_drill.go`
- `internal/migratecli/import_export_drill_test.go`
- `internal/migratecli/import_export_drill_sqlite_harness_test.go`
- `internal/migratecli/migratecli.go`
- `contrib/lab-retention-gc/metin2-print-retention-gc.sh`
- `contrib/lab-retention-gc/README.md`
- `contrib/lab-retention-gc/env/metin2-runtime-config.env.sample`
- `docs/development.md`
- `docs/workflow/migration-apply-runbook.md`
- Track E pointers in `docs/plans/2026-08-08-playable-vertical-roadmap.md` /
  `docs/plans/2026-08-09-db-migration-contract.md`
- this plan

## TDD and validation

```bash
go test ./internal/migratecli -run 'SynthesizeWipeExport|ImportExport|ImportExportDrill' -count=1
go test -tags=sqlite_harness ./internal/migratecli -run 'ImportExportDrillSQLite' -count=1
gofmt -l internal/migratecli/*.go
git diff --check
```

## Status

GREEN on `lane/persistence`.

- `metin2-migrate synthesize-wipe-export` emits bare wipe-scope exports for the six character-FK tip kinds and accepts bare or wrapped quarantine JSON.
- `metin2-migrate import-export` unwraps wrapped `quarantine-export` JSON before decode.
- `metin2-migrate import-export-drill --i-confirm-print-two-phase-wipe-roster-reimport` prints synthesize → wipe → roster → omit-roster reimport.
- Contrib helper forwards `METIN2_IMPORT_PRINT_TWO_PHASE_WIPE_ROSTER=YES`.
- Hermetic seeded SQLite proof executes the printed two-phase script green under `go test -tags=sqlite_harness ./internal/migratecli -run ImportExportDrillSQLiteHermeticPrintedScriptTwoPhase`.

## Exit criteria

- synthesizer emits bare wipe exports with non-empty derived scope
- import-export accepts wrapped quarantine JSON without changing Import* seams
- two-phase printer emits synthesize → wipe → roster → omit-roster reimport
- hermetic seeded SQLite proof executes the printed two-phase script green
- existing insert-only and single-pass scoped-replace proofs stay green
- stock binaries remain free of a registered production driver

## Anti-goals / ordering constraints

- Do not invent cascade delete inside roster replace.
- Do not rewrite retained `quarantine.json` in place.
- Do not auto-run printed scripts from CLI / contrib / cron.
- Do not push `origin/main`; push only `origin/lane/persistence`.
