# Character Safebox-State Migration + Export Seam — 2026-08-23

## Objective

Land the first schema-only SQL boundary and narrow repository-style export /
quarantine seam for durable same-account safebox passwords + cells already owned
by the `safeboxstore` FileStore rematerialize / backup-restore path, so operators
and hermetic tests can project migration-shaped artifacts without inventing a
DB-backed runtime or import/backfill execution.

## Why now

- Durable safebox FileStore, restart rematerialize, backup/restore drill fold-in,
  and loopback `/local/safebox-store/*` ops proofs already landed.
- Every other PvE durable surface used by login → map → reward → reconnect
  (`0002`/`0003`/`0004`/`0007`/`0009`/`0010`/`0011` plus static-content tips)
  already has a migration-shaped export + quarantine + named exporter seam.
- Safebox was the remaining FileStore-backed PvE vertical store without a
  schema/export boundary, so retained reconnect/restart evidence could not be
  quarantined beside the other eight bootstrap stores.

## Contract frozen by this slice

1. Embedded catalog adds `0014_character_safebox_state` after
   `0013_static_actor_combat_profile_state` (catalog tip moves to `14`; the
   static-actor content export tip stays at `13`).
2. `up` creates:
   - `character_safebox_passwords` keyed by `character_id` with operator-aid
     `login` and optional `password` (empty means bootstrap default `000000`)
   - `character_safebox_items` keyed by durable item `id`, unique
     `(character_id, cell)` for cells `0..14`
3. `down` drops items then passwords.
4. `safeboxstore.CharacterSafeboxStateExporter` exposes
   `ExportCharacterSafeboxState() (CharacterSafeboxStateExport, error)`.
5. `FileStore` and hermetic `MemoryStore` satisfy that seam. Missing snapshots
   export empty password/item arrays. Backup/restore/crash-temp stay FileStore-only.
6. Quarantine validators fail closed on migration tip mismatch, nil collections,
   invalid login/password/cell/item rows, duplicate keys, and unstable
   `character_id` ↔ `login` mappings.
7. Loopback-only `GET` / `POST` export + quarantine endpoints under
   `/local/safebox-store/exports/character-safebox-state` on `gamed`, plus
   `metin2-migrate quarantine-export --kind character-safebox-state`.
8. Docs tip the catalog / export inventory; no README churn, no SQL import,
   no remote admin, no money/mall widening.

## What this is not yet

- SQL-backed safebox repository at runtime
- INSERT / backfill / restore-from-export into live FileStores or databases
- daemon-local mutating migration apply
- safebox money / mall schema
- automatic artifact GC deletion
- remote admin authentication

## TDD and validation

- `go test ./db/migrations -run 'BuiltInCatalog|CatalogSummaryUsesBuiltIn|PlanUpToLatestUsesBuiltIn' -count=1`
- `go test ./internal/safeboxstore -run 'CharacterSafeboxState|MemoryStore' -count=1`
- `go test ./internal/ops -run 'CharacterSafeboxState|LocalSafeboxStore' -count=1`
- `go test ./internal/minimal -run 'ExportsCharacterSafeboxStateThroughMemoryStoreSeam' -count=1`
- `go test ./internal/migratecli -run 'QuarantineExport' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep SQL import/backfill deferred until a driver-backed harness exists.
2. Keep money / mall schema deferred on the items lane.
3. Keep automatic / scheduled artifact GC deletion deferred.
4. ~~Optional later: print-only systemd/unit samples for retention / GC printers.~~ Done: see [print-only retention / GC unit samples](2026-08-23-print-only-retention-gc-unit-samples.md) and [lab retention / GC print-only unit samples](../workflow/lab-retention-gc-unit-samples.md).
