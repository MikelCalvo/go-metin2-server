# Character Safebox Money Migration + Export Tip — 2026-08-23

## Objective

Tip the durable safebox migration/export/quarantine seam from password+cells
(`0014_character_safebox_state`) to include warehouse gold already owned by the
`safeboxstore` FileStore (`money` on each character row), so reconnect/restart
operators can project the same PvE warehouse-money state that open-burst
`SAFEBOX_MONEY_CHANGE` and `/safebox_money_save` / `/safebox_money_withdraw`
already rematerialize — without inventing SQL import, mall, or a DB-backed
runtime.

## Why now

- Items-lane already landed durable optional `money`, open-burst emission, and
  slash deposit/withdraw with FileStore rematerialize across restart.
- `0014` / `ExportCharacterSafeboxState` / quarantine still project only
  passwords + cells, so retained migration-shaped exports silently drop
  warehouse gold that the eighth manifested store already persists.
- Track E / migration-contract follow-ups still list money schema as deferred
  while the FileStore already owns it; that contradiction is a production-ops
  hazard for quarantine/preflight after deposit/withdraw enters the PvE loop.
- Additive tip-bump pattern is already proven (`0006` safebox-reject column,
  `0009` refine-info tables, `0012`/`0013` static-actor content tips).

## Contract frozen by this slice

1. Embedded catalog adds `0015_character_safebox_money` after
   `0014_character_safebox_state` (catalog tip moves to `15`; static-actor
   content tip stays `0013`).
2. `up` adds `money INTEGER NOT NULL DEFAULT 0` on
   `character_safebox_passwords` with
   `CHECK (money >= 0 AND money <= 2147483647)` (signed int32 wire bound).
3. `down` drops the `money` column.
4. `safeboxstore.CharacterSafeboxStateMigrationVersion` / `...Name` tip to
   `15` / `character_safebox_money`.
5. `CharacterSafeboxPasswordRow` carries optional `money` (omitted / zero means
   `0`); export projects `row.Money`; quarantine validates the same
   non-negative / `<= MaxInt32` bound and canonicalizes zero as `0`.
6. Loopback `/local/safebox-store/exports/character-safebox-state` (+quarantine)
   and `metin2-migrate quarantine-export --kind character-safebox-state` keep
   the same paths/kind; retained tip-`14` payloads fail closed until re-exported.
7. Docs tip catalog / quarantine inventory / Track E through `0015`; mall /
   SQL import / remote admin remain deferred.

## What this is not yet

- SQL import/backfill from quarantined exports into FileStores or databases
- DB-backed safebox repository at runtime
- mall money / mall open-checkout
- TMP4 CG `SAFEBOX_MONEY` request packet
- automatic artifact GC deletion
- remote admin authentication
- README churn beyond operator docs already required

## TDD and validation

- `go test ./db/migrations -run 'BuiltInCatalog|CatalogSummaryUsesBuiltIn|PlanUpToLatestUsesBuiltIn' -count=1`
- `go test ./internal/safeboxstore -run 'CharacterSafeboxState|MemoryStore' -count=1`
- `go test ./internal/ops -run 'CharacterSafeboxState' -count=1`
- `go test ./internal/migratecli -run 'QuarantineExport' -count=1`
- `go test ./internal/minimal -run 'MigrationCatalog|PlanUpToLatestUsesBuiltIn|ExportsCharacterSafeboxState' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep SQL import/backfill deferred until a driver-backed harness exists.
2. Keep mall schema / TMP4 CG `SAFEBOX_MONEY` deferred on the items lane.
3. Keep automatic / scheduled artifact GC deletion deferred.
4. Optional later: print-only systemd/unit samples for retention / GC printers.
5. ~~Prove warehouse-money rematerialize across full `gamed` process restart beside same-session reopen.~~ Done: see [safebox money process-restart rematerialize](2026-08-23-safebox-money-process-restart-rematerialize.md).
