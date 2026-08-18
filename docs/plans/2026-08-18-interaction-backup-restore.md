# Interaction Backup/Restore — 2026-08-18

## Objective

Give the authored interaction-definition snapshot store the same manifest-closed backup, dry-run validation, and replacement restore surface already owned by the account, login-ticket, item-template, and quest-state stores, so operators can preserve and recover NPC / service interaction content during local reconnect/restart drills without inventing a remote admin API.

## Contract frozen by this slice

`interactionstore.FileStore` now owns:

- `BackupTo(dstDir)`
- `ValidateBackupFrom(srcDir)`
- `RestoreFrom(srcDir)`

Rules frozen by tests:

- destination directories must be empty and outside the live store (lexical and symlink-resolved);
- only the committed `interaction-definitions.json` snapshot is copied when present;
- missing committed snapshots backup/restore as an empty store with no synthetic snapshot file;
- hidden `.interaction-definitions-*.json` crash temps are ignored as payload and reported by dry-run validation;
- backups write `interaction-backup-manifest.json` (`go-metin2-interaction-backup-v1`) with deterministic summary, filename, size, and SHA-256 checksum;
- restore/preflight require that manifest and reject checksum drift, untracked visible entries, and symlinked manifests/snapshots;
- restore refuses non-empty destinations and destinations nested under the backup source;
- successful `Save` removes a stale restored manifest so live mutation does not keep obsolete backup metadata;
- `Validate()` fails closed when an active restored manifest no longer matches the committed snapshot.

`gamed` exposes the surface through loopback-only:

- `POST /local/interaction-store/backup`
- `POST /local/interaction-store/backup/validate`
- `POST /local/interaction-store/restore`

Runtime restore also refuses live selected-character sessions and reloads the restored snapshot into the live interaction-definition index when sessions are drained. `/local/persistence/status` now reports interaction-store `backup_manifest` and `restore_blocked_by_live_sessions`.

## What this is not yet

This is not database-backed interaction storage, not daemon startup auto-restore, not a remote admin API, and not backup/restore for static actors. It also does not yet document a combined multi-store backup drill.

## TDD and validation

Focused coverage:

- `go test ./internal/interactionstore -run 'TestFileStore(BackupTo|ValidateBackupFrom|RestoreFrom|SaveRemovesStaleBackupManifest|ValidateRejectsStaleBackupManifest)' -count=1`
- `go test ./internal/ops -run 'TestLocalInteractionStore(Backup|Restore)' -count=1`
- `go test ./internal/minimal -run 'TestGameRuntime(Backup|Validate|Restore)InteractionStore' -count=1`

## Follow-up options

1. Add the same backup/restore posture for authored static actors (content lane already has a pending slice).
2. Document a combined file-store backup drill that sequences account, login-ticket, item-template, quest-state, interaction, and static-actor restore under drained live sessions.
3. Keep DB interaction repositories deferred until the schema-shaped export/import quarantine path exists.
