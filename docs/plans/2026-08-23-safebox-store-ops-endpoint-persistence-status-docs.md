# Safebox Store Ops Endpoint + Persistence-Status Docs Proof — 2026-08-23

## Objective

Close the remaining ops-lane gap after the durable safebox FileStore and
`metin2-migrate backup-restore-drill` fold-in landed: prove the already-registered
loopback `/local/safebox-store/*` handlers and `gameRuntime` backup/restore/
crash-temp/status seams with focused tests, and sync
`docs/debugging-and-profiling.md` so operator triage no longer omits the eighth
store from runtime-config / persistence-status / endpoint inventory.

No password/money/mall, SQL import, remote admin, or README churn is added.

## Why now

- `feat(items): rematerialize durable safebox cells across restart` and
  `feat(ops): fold safebox into backup-restore drill` already ship the FileStore,
  `gamed` endpoint registration, drill printer, apply-runbook preflight, and lab
  topology layout.
- `internal/ops` still had no `LocalSafeboxStore*` coverage beside the ground-item
  mirror, so the loopback-only HTTP contract was unproven for the eighth store.
- `internal/minimal` still lacked focused runtime
  Backup/ValidateBackup/Restore/Cleanup/live-session-guard proofs for safebox,
  even though the same surface already existed for ground items.
- `docs/debugging-and-profiling.md` still listed seven persistence paths under
  `GET /local/runtime-config` / `GET /local/persistence/status` and omitted the
  `/local/safebox-store/*` endpoint sections, which is a production-ops hazard
  for reconnect/restart triage after the drill already prints those curls.

## Contract frozen by this slice

1. Focused `internal/ops` coverage proves loopback success and non-loopback `403`
   for `/local/safebox-store/validate`, `/crash-temps/cleanup`, `/backup`,
   `/backup/validate`, and `/restore`, plus `409` on backup callback failure.
2. Focused `internal/minimal` coverage proves
   `BackupSafeboxStore` / `ValidateSafeboxStoreBackup` / `RestoreSafeboxStore` /
   `CleanupSafeboxStoreCrashTempFiles`, including live-session restore fail-closed
   without mutating the target store and persistence-status
   `backup_manifest` / `restore_blocked_by_live_sessions` reporting.
3. `docs/debugging-and-profiling.md` documents the five `/local/safebox-store/*`
   endpoints, lists `persistence.safebox_store_path` in runtime-config, and
   reports `safebox_store` under persistence status beside the seven older
   bootstrap stores.
4. Password / money / mall / SQL safebox schema / remote admin remain deferred.

## What this is not yet

- safebox password challenge / money / mall
- SQL import/backfill of safebox cells
- daemon startup auto-restore beyond already-owned rematerialize
- automatic artifact GC deletion
- systemd/unit samples that auto-run retention / GC printers
- remote admin authentication

## TDD and validation

- `go test ./internal/ops -run 'LocalSafeboxStore' -count=1`
- `go test ./internal/minimal -run 'SafeboxStore(Backup|Validate|Restore|Cleanup)|RestoreSafeboxStore|BackupSafeboxStore|CleanupSafeboxStore' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep SQL safebox schema / import deferred until a driver-backed harness exists.
2. Keep password / money / mall deferred on the items lane.
3. Keep automatic / scheduled artifact GC deletion deferred.
4. ~~Optional later: systemd/unit samples that only print (never auto-run)
   retention / GC triage scripts.~~ Done: see [print-only retention / GC unit samples](2026-08-23-print-only-retention-gc-unit-samples.md) and [lab retention / GC print-only unit samples](../workflow/lab-retention-gc-unit-samples.md).
5. Optional later: hermetic end-to-end HTTP drill against a live drained `gamed`
   (printer remains read-only).
