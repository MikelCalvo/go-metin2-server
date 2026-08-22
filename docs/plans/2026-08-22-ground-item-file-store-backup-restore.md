# Ground-Item File-Store Backup/Restore — 2026-08-22

## Objective

Give the durable pending ground-item / ground-gold FileStore the same
manifest-closed backup, dry-run validation, and replacement restore surface
already owned by the six bootstrap JSON stores, then fold it into the combined
loopback backup/restore drill so reconnect/restart operators can preserve live
ground handles after the process-restart rematerialize slice landed.

Pending ground handles already rematerialize across `gamed` restart from
`GroundItemStorePath`. Backup/restore drill still accepted the runtime-config
field but did **not** cover BackupTo / RestoreFrom or drill curls. This slice
closes that deferred ops gap without inventing a remote admin API or SQL import.

## Contract frozen by this slice

`worldruntime.FileStore` (ground-item) now owns:

- `BackupTo(dstDir)`
- `ValidateBackupFrom(srcDir)`
- `RestoreFrom(srcDir)`
- `CleanupCrashTempFiles()`

Rules frozen by tests:

1. Destination directories must be empty and outside the live store (lexical and
   symlink-resolved).
2. Only the committed `ground-items.json` snapshot is copied when present.
3. Missing committed snapshots backup/restore as an empty store with no
   synthetic snapshot file.
4. Hidden `.ground-items-*.json` crash temps are ignored as payload and reported
   by dry-run validation / `Validate()`.
5. Backups write `ground-item-backup-manifest.json`
   (`go-metin2-ground-item-backup-v1`) with deterministic summary, filename,
   size, and SHA-256 checksum.
6. Restore/preflight require that manifest and reject checksum drift, untracked
   visible entries, and symlinked manifests/snapshots.
7. Restore refuses non-empty destinations and destinations nested under the
   backup source.
8. Successful `Save` removes a stale restored manifest so live mutation does not
   keep obsolete backup metadata.
9. `Validate()` fails closed when an active restored manifest no longer matches
   the committed snapshot.

`gamed` exposes the surface through loopback-only routes under the
`ground-item-store` prefix (avoids colliding with live
`GET /local/ground-items` / by-VID / migration-export routes):

- `POST /local/ground-item-store/validate`
- `POST /local/ground-item-store/crash-temps/cleanup`
- `POST /local/ground-item-store/backup`
- `POST /local/ground-item-store/backup/validate`
- `POST /local/ground-item-store/restore`

Runtime restore:

- refuses live selected-character sessions
- replaces the empty FileStore destination
- clears in-memory pending ground handles and rematerializes from the restored
  snapshot (including empty restore → empty live set)
- filters despawned / expired-exclusive rows with the same restore filter used
  at process restart

`/local/persistence/status` reports ground-item `backup_manifest` and
`restore_blocked_by_live_sessions` beside the existing path/valid/summary.

`metin2-migrate backup-restore-drill` and
`docs/workflow/file-store-backup-restore-drill.md` include the seventh store in
validate / backup / backup-validate / empty-destination / restore sequencing
(after quest-state for backup; after quest-state and before account for restore).

## What this is not yet

- SQL import/backfill from quarantined `0010` exports
- daemon startup auto-restore beyond the already-owned rematerialize path
- remote admin authentication
- claiming `MemoryGroundItemStore` is restart-durable or backup-capable
- replacing live online `Register*` with restore semantics
- persisting process-local `OwnerID` / `OwnerHPPoint` as durable truth

## TDD and validation

- `go test ./internal/worldruntime -run 'GroundItemFileStore|FileStoreBackup|FileStoreRestore|FileStoreValidate|CleanupCrashTemp' -count=1`
- `go test ./internal/ops -run 'LocalGroundItemStore' -count=1`
- `go test ./internal/minimal -run 'GroundItemStore(Backup|Validate|Restore|Cleanup)|PersistenceStatusReports.*GroundItem' -count=1`
- `go test ./internal/migratecli -run 'BackupRestoreDrill' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep SQL import/backfill deferred until a driver-backed harness exists.
2. Keep automatic artifact GC deferred.
3. Optional later: rebind process-local `OwnerID` when the exclusive owner rejoins.
