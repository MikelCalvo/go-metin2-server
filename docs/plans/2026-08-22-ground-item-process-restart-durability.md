# Ground-Item Process-Restart Durability — 2026-08-22

## Status

Landed on `lane/persistence` as the Track E.4 pending ground-item / ground-gold
process-restart rematerialize slice. Follow-ups below remain open.

Pending bootstrap ground item/gold handles now rematerialize from a dedicated
FileStore with absolute ownership/despawn timers and an offline restore path that
does **not** require a live owner. Inventory/gold/quest/equipment already had
FileStore rematerialize; this slice closes the matching gap for live ground
handles that previously lived only in `sharedWorldRegistry.groundItemsByVID`.

## Contract frozen by this slice

1. New durable FileStore at `config.Service.GroundItemStorePath`
   (`METIN2_GROUND_ITEM_STORE_PATH` / `METIN2_GAMED_GROUND_ITEM_STORE_PATH`),
   default `$TMP/go-metin2-server-ground-items/ground-items.json`, with a
   dedicated parent rejected by `ValidatePersistenceConfig` when shared.
2. Durable snapshot shape (extends 0010; not equal to the migration export):
   - 0010 fields: `vid`, `vnum`, `item_count` XOR `gold_amount`, owner identity,
     map/x/y/z, `pickup_range`
   - runtime extras: `item_id` (item-shaped only), `ownership_exclusive`,
     absolute UTC `ownership_expires_at` (omitempty when public), absolute UTC
     `despawn_at` (required)
3. Persist the full pending set after successful register, remove/pickup, and
   ownership/despawn flush. Atomic temp + rename + fsync, crash-temp prefix
   `.ground-items-*.json`.
4. On `gameRuntime` construction, after shared-world create:
   - `Load()`; missing snapshot → empty OK
   - drop rows with `despawn_at <= now`
   - if exclusive and `ownership_expires_at <= now` → force public
   - insert via `RestorePersistedGroundItem(s)` that does **not** call live
     `RegisterGroundItem*` / does **not** require a live owner
   - do **not** persist process-local `OwnerID` / `OwnerHPPoint`
5. Exclusive ownership after rematerialize is identity-keyed
   (`OwnerLogin` / `OwnerCharacterID` / `OwnerVID` / `OwnerName`) when
   `OwnerID == 0`, so peers cannot loot during the exclusive window just because
   the process-local entity id is gone.
6. `GET /local/runtime-config` and `GET /local/persistence/status` report the
   ground-item store path / validity / summary. Backup/restore drill accepts the
   new runtime-config field but still does **not** cover ground handles.
7. Focused proof:
   - `TestGameRuntimePendingGroundItemAndGoldRematerializeAcrossDaemonRestart`
   - ownership mid-window rematerialize stays exclusive for the owner identity
     and blocks a peer until expiry

## What this is not yet

- SQL import/backfill from quarantined `0010` exports
- ~~adding ground handles to `backup-restore-drill` / BackupTo / RestoreFrom~~ Done: see [ground-item file-store backup/restore](2026-08-22-ground-item-file-store-backup-restore.md).
- replacing live online `Register*` with restore semantics
- persisting process-local `OwnerID` / `OwnerHPPoint` as durable truth
- claiming `MemoryGroundItemStore` is restart-durable
- remote admin auth

## TDD and validation

- `go test ./internal/worldruntime -run 'GroundItemFileStore|DurableGroundItem' -count=1`
- `go test ./internal/config -run 'ValidatePersistenceConfig|LoadServiceUsesBootstrapPersistenceDefaults|GroundItem' -count=1`
- `go test ./internal/minimal -run 'PendingGroundItemAndGoldRematerialize|GroundItemOwnershipRematerialize|GameRuntimeConfigSnapshotReportsPersistence|NewGameRuntimeRejectsSharedFileStoreParent' -count=1`
- `go test ./internal/migratecli -run 'BackupRestoreDrill' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep SQL import/backfill deferred until a driver-backed harness exists.
2. ~~Optional later: include ground-item FileStore in backup-restore drill once
   BackupTo/RestoreFrom exist.~~ Done: see [ground-item file-store backup/restore](2026-08-22-ground-item-file-store-backup-restore.md).
3. Optional later: rebind process-local `OwnerID` when the exclusive owner
   rejoins the shared world.
4. Keep automatic artifact GC deletion deferred.
