# File-Store Backup/Restore Drill — 2026-08-19

## Objective

Document the combined loopback-only backup → validate → restore sequence for the six bootstrap JSON stores that already own manifested backup primitives, so operators can run reconnect/restart and migration-window drills without improvising per-store ordering or inventing a remote admin API.

## Contract frozen by this slice

`docs/workflow/file-store-backup-restore-drill.md` now owns the operator workflow for:

- account store
- login-ticket store
- item-template store
- interaction-definition store
- static-actor store
- quest-state store

Rules frozen by the runbook:

- confirm active paths with `GET /local/runtime-config` and drain selected-character sessions via `GET /local/persistence/status` before restore;
- backup destinations must be empty and outside live stores;
- every backup is dry-run validated before any live destination is emptied;
- restore remains a replacement into an empty destination and refuses live selected-character sessions;
- file-path stores must use dedicated parent directories because restore empties `filepath.Dir(snapshotPath)`;
- recommended backup order is account → login tickets → item templates → interactions → static actors → quest state;
- recommended restore order is item templates → interactions → static actors → quest state → account → login tickets;
- migration apply still uses the CLI-only path and treats this drill as the file-store half of backup validation.

Supporting doc fixes in the same slice:

- `docs/debugging-and-profiling.md` now documents interaction-store `backup_manifest` and `restore_blocked_by_live_sessions` on `/local/persistence/status`, matching the runtime status shape;
- `docs/workflow/migration-apply-runbook.md` and `docs/development.md` link the combined drill and list all six store validate/backup-validate surfaces.

## What this is not yet

This is not a new endpoint, not daemon startup auto-restore, not DB-backed repositories, not ground-item durability, and not a remote admin API. It also does not automate aside-rename/restore orchestration beyond the documented curl and filesystem steps.

## TDD and validation

Docs-only slice. Validation:

- review endpoint inventory against `cmd/gamed/main.go` registrations and current backup/restore docs;
- confirm interaction status fields against `InteractionStoreStatus` in `internal/minimal/factory.go`;
- `git diff --check` on touched docs.

## Follow-up options

1. ~~Add a small operator script or hermetic dry-run helper that prints the drill commands from `/local/runtime-config` without performing restore.~~ Done: `metin2-migrate backup-restore-drill`.
2. Keep ground-item / ground-gold restart durability deferred until a real world-state repository exists.
3. Extract repository seams only after offline quarantine + loopback quarantine both prove the export boundary.
