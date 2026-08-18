# Quest-State Backup/Restore — 2026-08-18

## Objective

Give the standalone bootstrap quest-state snapshot store the same manifest-closed backup, dry-run validation, and replacement restore surface already owned by the account, login-ticket, and item-template stores, so operators can preserve and recover quest flags during local PvE reconnect/restart drills without inventing a remote admin API.

## Contract frozen by this slice

`queststate.FileStore` now owns:

- `BackupTo(dstDir)`
- `ValidateBackupFrom(srcDir)`
- `RestoreFrom(srcDir)`

Rules frozen by tests:

- destination directories must be empty and outside the live store (lexical and symlink-resolved);
- only the committed `quest-state.json` snapshot is copied when present;
- missing committed snapshots backup/restore as an empty store with no synthetic snapshot file;
- hidden `.quest-state-*.json` crash temps are ignored as payload and reported by dry-run validation;
- backups write `quest-state-backup-manifest.json` (`go-metin2-quest-state-backup-v1`) with deterministic summary, filename, size, and SHA-256 checksum;
- restore/preflight require that manifest and reject checksum drift, untracked visible entries, and symlinked manifests/snapshots;
- restore refuses non-empty destinations and destinations nested under the backup source;
- successful `Save` / transition apply removes a stale restored manifest so live mutation does not keep obsolete backup metadata;
- `Validate()` fails closed when an active restored manifest no longer matches the committed snapshot.

`gamed` exposes the surface through loopback-only:

- `POST /local/quest-state/backup`
- `POST /local/quest-state/backup/validate`
- `POST /local/quest-state/restore`

Runtime restore also refuses live selected-character sessions. `/local/persistence/status` now reports quest-state `backup_manifest` and `restore_blocked_by_live_sessions`.

## What this is not yet

This is not database-backed quest storage, not daemon startup auto-restore, not a remote admin API, and not backup/restore for static actors or interaction definitions. It also does not yet document a combined multi-store backup drill.

## TDD and validation

Focused coverage:

- `go test ./internal/queststate -run 'TestFileStore(BackupTo|ValidateBackupFrom|RestoreFrom|SaveRemovesStaleBackupManifest|ValidateRejectsStaleBackupManifest)' -count=1`
- `go test ./internal/ops -run 'TestLocalQuestStateStore(Backup|Restore)' -count=1`
- `go test ./internal/minimal -run 'TestGameRuntime(Backup|Validate|Restore)QuestStateStore' -count=1`

## Follow-up options

1. Add the same backup/restore posture for authored static/interaction stores.
2. Document a combined file-store backup drill that sequences account, login-ticket, item-template, and quest-state restore under drained live sessions.
3. Keep DB quest repositories deferred until the schema-shaped export/import quarantine path exists.
