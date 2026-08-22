# Ground-Item Leave Persist Owned Deletion — 2026-08-22

## Objective

When graceful shared-world `Leave` / stale reclaim deletes `OwnerID`-matched pending ground item/gold handles, also fire the already-owned `onGroundItemsChanged` persist hook so the durable FileStore drops those rows instead of rematerializing deleted litter after the next `gamed` restart.

## Contract to own

1. `removeOwnedGroundItemsLocked` (used by `Leave` and stale reclaim) persists the resulting pending-ground snapshot through the existing `SetGroundItemsChangedHook` / `persistPendingGroundItems` path whenever at least one owned handle was removed.
2. After owner leave with a live exclusive handle, `GroundItemFileStore.Load()` no longer contains that `vid`.
3. Peer still receives the already-owned `GC::ITEM_GROUND_DEL` fanout; no inventory/gold/account mutation is introduced.
4. Rematerialized exclusive handles that still have `OwnerID = 0` (owner not yet rebound) remain untouched by Leave of a different entity and still survive FileStore rematerialize until rebound + later leave, or until ownership/despawn timers expire.
5. Spec/QA name graceful Leave→FileStore deletion beside the owned rematerialize / OwnerID-rebind contracts.

## What this is not yet

- changing ownership/despawn absolute-timer semantics
- SQL import/backfill from quarantined `0010` exports
- party-shaped owner-delivery notices
- partner-side open player-shop / cube busy rejects
- durable safebox persistence / password / money

## TDD and validation

Focused coverage:

- `go test ./internal/minimal -run 'LeavePersistsOwnedGroundItemDeletion|LeavePersistsOwnedGroundGoldDeletion|StaleReclaimPersistsOwnedGroundItemDeletion' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep SQL import/backfill deferred.
2. Keep party-shaped owner-delivery deferred.
3. Keep partner-side open player-shop / cube busy rejects deferred.

## Status

Shipped: graceful Leave / stale reclaim now fire `onGroundItemsChanged` after owned-ground deletion so the FileStore drops those rows; rematerialize crash proofs abandon without that persist hook. SQL import/backfill and party-shaped owner-delivery stay deferred.
