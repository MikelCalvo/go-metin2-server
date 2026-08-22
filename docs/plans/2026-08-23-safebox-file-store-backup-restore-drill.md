# Safebox File-Store Backup/Restore Drill Fold-In — 2026-08-23

## Objective

Fold the already-shipped durable safebox FileStore (`safeboxstore` + loopback
`/local/safebox-store/*`) into the combined `metin2-migrate backup-restore-drill`
printer and operator runbooks so reconnect/restart operators can preserve
same-account warehouse cells beside the seven manifested bootstrap stores —
without inventing password/money/mall, SQL import, or a remote admin API.

Durable safebox rematerialize across process restart already landed on `main`
(`feat(items): rematerialize durable safebox cells across restart`). The store
owns `BackupTo` / `ValidateBackupFrom` / `RestoreFrom` / crash-temp cleanup, and
`gamed` already registers loopback validate / cleanup / backup / backup-validate /
restore endpoints. The drill printer still decoded `GET /local/runtime-config`
with `DisallowUnknownFields` **without** `persistence.safebox_store_path`, so a
live retained runtime-config from current `gamed` would fail closed before any
curl was printed. This slice closes that ops gap.

## Contract frozen by this slice

1. `backupRestorePersistenceConfig` accepts and requires absolute
   `safebox_store_path` beside the existing seven persistence fields.
2. Shared-parent fail-closed checks include `filepath.Dir(safebox_store_path)`
   with the other file-path stores.
3. Printed drill script exports `SAFEBOX_STORE_PATH`, creates `$BASE/safebox`,
   and sequences safebox validate / crash-temp cleanup / backup / backup-validate
   / aside-rename / restore through `/local/safebox-store/*`.
4. Backup order places safebox after ground items (eighth store). Restore order
   places safebox after ground items and before account/login-ticket replacement.
5. Lab topology, file-store drill runbook, migration-apply runbook, and
   development docs name the eighth store and the dedicated
   `/var/metin2/data/safebox/safebox.json` parent layout.
6. Password / money / mall / SQL safebox schema / remote admin remain deferred.

## What this is not yet

- safebox password challenge / money / mall
- SQL import/backfill of safebox cells
- daemon startup auto-restore beyond already-owned rematerialize
- automatic artifact GC deletion
- remote admin authentication

## TDD and validation

- `go test ./internal/migratecli -run 'BackupRestoreDrill' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep SQL safebox schema / import deferred until a driver-backed harness exists.
2. Keep password / money / mall deferred on the items lane.
3. Optional later: hermetic end-to-end HTTP drill against a live drained `gamed`
   (printer remains read-only).
