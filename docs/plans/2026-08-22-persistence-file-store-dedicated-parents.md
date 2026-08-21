# Persistence File-Store Dedicated Parents — 2026-08-22

## Objective

Fail closed at `gamed` startup when bootstrap file-backed stores share a parent directory, and make zero-config defaults / lab topology examples use dedicated parents — matching the restore contract already owned by `metin2-migrate backup-restore-drill` and `docs/workflow/file-store-backup-restore-drill.md`.

File-path restore empties `filepath.Dir(snapshotPath)`. Shared parents are therefore hostile to multi-store restore: restoring one store would wipe sibling snapshots/manifests. The CLI drill printer already rejects that layout; daemon startup and lab docs still allow / document it.

## Contract frozen by this slice

1. `config.ValidatePersistenceConfig` rejects any pair of file-role stores whose cleaned absolute parent directories collide on either the symlink-resolved path or the lexical absolute path.
2. New sentinel: `ErrPersistencePathSharedParent`.
3. Zero-config defaults move under dedicated temp parents:
   - `.../go-metin2-server-static-actors/static-actors.json`
   - `.../go-metin2-server-interaction-definitions/interaction-definitions.json`
   - `.../go-metin2-server-item-templates/item-templates.json`
   - `.../go-metin2-server-quest-state/quest-state.json`
4. Lab topology absolute-path examples and env exports use dedicated parents under `/var/metin2/data/.../`.
5. Developer / debugging docs state that shared file-store parents fail startup the same way overlapping directory trees already do.
6. Directory-store overlap / role / symlink rules are unchanged. Authd handoff validation is unchanged (no file stores).

## What this is not yet

- automatic migration of existing shared-parent lab trees
- ground-item restart durability / `0010` recovery
- SQL import/backfill
- remote admin APIs
- changing restore semantics themselves

## TDD and validation

- `go test ./internal/config -run 'ValidatePersistenceConfig|LoadServiceUsesBootstrapPersistenceDefaults' -count=1`
- `go test ./internal/minimal -run 'NewGameRuntimeRejects|GameRuntimeConfigSnapshotReportsPersistence|AuthSessionFactoryWithValidatedConfigRejects' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep ground-item restart durability deferred until operators decide quarantined `0010` exports should drive recovery.
2. Keep SQL import/backfill deferred until a driver-backed harness and backup policy exist.
3. Optional Docker `LABEL` workflow-run metadata remains deferred.
