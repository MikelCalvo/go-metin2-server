# Safebox cell instance-sockets SQL additive — 2026-08-30

## Objective

Close the migration/export/import gap after items-lane owned FileStore
presence-aware safebox cell instance sockets: add additive catalog migration
`0025_character_safebox_item_instance_sockets`, project those sockets through
tip-`0015` export/quarantine/import, and fail closed before SQL INSERT when the
ledger owns tip-`0015` but not additive `0025`.

## Why now

- Durable safebox FileStore / runtime already round-trip and honor cell instance
  sockets (including explicit zero) through check-in → restart → reopen /
  checkout (`docs/plans/2026-08-30-safebox-cell-instance-sockets-durable.md`).
- Migration-shaped tip-`0015` export/import still omit sockets after that GREEN,
  so quarantined SQL backfill silently drops authoritative safebox cell instance
  sockets (including deactivated auto-potion `socket0 = 0`).
- Track E prefers explicit additive schema + import preflight over opaque
  driver `no such column` errors (same pattern as tip-`0003` + `0024`).
- Safer than inventing a new tip identity: sockets extend existing
  `character_safebox_items` rows while export identity stays tip `15`.

## Contract frozen by this slice

1. Embedded catalog adds `0025_character_safebox_item_instance_sockets` after
   `0024_character_item_instance_sockets` (catalog tip moves to `25`).
2. `up` adds `has_sockets` + `socket0`/`socket1`/`socket2` on
   `character_safebox_items` with CHECKs mirroring `0024`:
   - `has_sockets IN (0, 1)`
   - each socket in signed int32 range
   - when `has_sockets = 0`, all sockets must be `0`
3. `down` drops those columns (dependent `socket2` first).
4. Keep tip-`0015` / `character_safebox_money` as the export / quarantine /
   import-result migration identity (do **not** retip to `25`).
5. `CharacterSafeboxItemRow` carries optional `has_sockets` +
   `socket0`/`socket1`/`socket2`; export maps:
   - `HasSockets == false` / omitted → omitted / `has_sockets=false`, sockets `0`
   - `HasSockets == true` (including all-zero) → `has_sockets=true` + values
6. Quarantine rejects non-zero sockets when `has_sockets` is false.
7. `ImportCharacterSafeboxState` inserts the new columns and requires tip-`0015`
   plus additive `0025` before any INSERT
   (`ErrCharacterSafeboxStateImportSchemaRequired` when either boundary is
   missing).
8. Mall / TMP4 CG `SAFEBOX_MONEY` request header / client
   `SAFEBOX_CHANGE_PASSWORD` packets / attributes-on-instance remain deferred.

## What this is not yet

- retipping safebox-state exports to `migration_version=25`
- DB-backed live safebox repositories
- remote admin / daemon mutation route / secrets in git
- mall open/checkout / GD/DB myshop

## Likely files to change (GREEN follow-on)

- `db/migrations/0025_character_safebox_item_instance_sockets.{up,down}.sql`
- `db/migrations/migrations.manifest.json`
- `db/migrations/catalog_test.go` / `plan_test.go`
- `internal/safeboxstore/export.go`
- `internal/safeboxstore/export_quarantine.go`
- `internal/safeboxstore/safebox_state_import.go`
- `internal/safeboxstore/*_test.go` (+ sqlite harness)
- `internal/migratecli` / `internal/ops` / `internal/minimal` migration tips
- `docs/development.md` / migration contract / roadmap / QA checklist
- this plan

## TDD and validation (after GREEN opens)

- `go test ./db/migrations -run 'BuiltInCatalog|CatalogSummaryUsesBuiltIn|PlanUpToLatestUsesBuiltIn' -count=1`
- `go test ./internal/safeboxstore -run 'ExportCharacterSafebox|ValidateCharacterSafebox|QuarantineCharacterSafebox|ImportCharacterSafebox|InstanceSockets' -count=1`
- `go test -tags=sqlite_harness ./internal/safeboxstore -run SQLiteHarness -count=1`
- `go test ./internal/migratecli -run 'QuarantineExport|ImportExport' -count=1`
- `go test ./internal/ops -run 'LocalMigrationStatus|CharacterSafebox' -count=1`
- `go test ./internal/minimal -run 'MigrationStatus|MigrationCatalog|RegisterGamedMigration' -count=1`
- `gofmt` on touched Go files
- `git diff --check`
- `go test ./...` and `go vet ./...`

## Status

GREEN shipped on `lane/persistence` (this run): additive catalog tip `0025`,
tip-`0015` export/quarantine/import projection of presence-aware safebox cell
sockets, and import preflight requiring tip-`0015` plus additive `0025`.
FileStore rematerialize for safebox cell sockets was already shipped
(`ea033bb6`).

Follow-on tip sync: seeded hermetic tip-`0015`+`0025` safebox cell sockets in
the shared import-export-drill plus operator loopback docs — see
[seeded safebox cell instance-sockets tip sync](2026-08-30-seeded-safebox-cell-instance-sockets-import-export-drill.md).
