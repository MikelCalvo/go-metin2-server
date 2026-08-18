# Login Ticket Backup/Restore — 2026-08-18

## Objective

Give the one-shot authd-to-gamed login-ticket handoff store the same manifest-closed backup, dry-run validation, and replacement restore surface already owned by the account and item-template stores, so operators can preserve and recover pending login keys during local PvE reconnect/restart drills without inventing a remote admin API.

## Contract frozen by this slice

`loginticket.FileStore` now owns:

- `BackupTo(dstDir)`
- `ValidateBackupFrom(srcDir)`
- `RestoreFrom(srcDir)`

Rules frozen by tests:

- destination directories must be empty and outside the live store (lexical and symlink-resolved);
- only committed canonical ticket snapshots are copied;
- hidden `.ticket-*.json` crash temps are ignored as payload and reported by dry-run validation;
- backups write `login-ticket-backup-manifest.json` (`go-metin2-login-ticket-backup-v1`) with deterministic summary, filenames, login keys, sizes, and SHA-256 checksums;
- restore/preflight require that manifest and reject checksum drift, untracked visible entries, and symlinked manifests/snapshots;
- restore refuses non-empty destinations and destinations nested under the backup source;
- `Issue`, `Consume`, and successful stale-ticket cleanup remove a stale restored manifest so live mutation does not keep obsolete backup metadata;
- `Validate()` fails closed when an active restored manifest no longer matches committed tickets.

`gamed` exposes the surface through loopback-only:

- `POST /local/login-tickets/backup`
- `POST /local/login-tickets/backup/validate`
- `POST /local/login-tickets/restore`

Runtime restore also refuses live selected-character sessions. `/local/persistence/status` now reports login-ticket `backup_manifest` and `restore_blocked_by_live_sessions`.

## What this is not yet

This is not database-backed ticket storage, not daemon startup auto-restore, not a remote admin API, and not backup/restore for static actors, interactions, or quest state.

## TDD and validation

Focused coverage:

- `go test ./internal/loginticket -run 'TestFileStore(BackupTo|ValidateBackupFrom|RestoreFrom|IssueRemovesStaleBackupManifest|ValidateRejectsStaleBackupManifest)' -count=1`
- `go test ./internal/ops -run 'TestLocalLoginTicketStore(Backup|Restore)' -count=1`
- `go test ./internal/minimal -run 'TestGameRuntime(Backup|Validate|Restore)LoginTicketStore' -count=1`

## Follow-up options

1. Add the same backup/restore posture for quest-state and authored static/interaction stores.
2. Combined multi-store backup/restore sequencing is now documented in `docs/workflow/file-store-backup-restore-drill.md`.
3. Keep DB ticket repositories deferred until the schema-shaped export/import quarantine path exists.
