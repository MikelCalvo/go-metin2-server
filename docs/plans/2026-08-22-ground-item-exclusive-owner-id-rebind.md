# Ground-Item Exclusive OwnerID Rebind On Rejoin — 2026-08-22

## Objective

When an exclusive pending ground item/gold handle was rematerialized from the durable FileStore with process-local `OwnerID = 0`, rebind that handle to the fresh shared-world entity id as soon as the matching owner identity rejoins, so live exclusive ownership follows the same `OwnerID` path as online drops instead of staying identity-only forever.

## Contract to own

1. On successful shared-world `Join`, for each still-exclusive pending ground handle whose process-local `OwnerID` is `0` and whose durable owner identity (`OwnerCharacterID` / `OwnerVID` / `OwnerName`) matches the joining character, set `OwnerID` to the new entity id.
2. Absolute `ownership_expires_at` / `despawn_at` timers, durable FileStore rows, and peer mid-window fail-closed pickup stay unchanged by the rebind itself (`OwnerID` remains process-local and is still omitted from the durable snapshot).
3. After rebind, owner pickup continues to succeed and peer pickup continues to fail closed while the exclusive window is active.
4. Non-matching joins, public (non-exclusive) handles, and already-bound (`OwnerID != 0`) handles are left untouched.
5. Existing owner-leave cleanup that deletes `OwnerID`-matched live handles remains unchanged; rematerialized `OwnerID = 0` handles still survive leave until rebound.

## What this is not yet

- changing graceful Leave to persist owned-ground deletion into the FileStore (crash vs leave durability semantics stay as today)
- SQL import/backfill from quarantined `0010` exports
- party-shaped owner-delivery notices
- partner-side open player-shop / cube busy rejects
- dual-sided id-collision / restriction finalize reject chat

## TDD and validation

Focused coverage:

- `go test ./internal/minimal -run 'SharedWorldRegistryRebindsExclusiveGroundOwnerIDOnMatchingJoin|SharedWorldRegistryDoesNotRebindExclusiveGroundOwnerIDForNonMatchingJoin|PendingGroundItemExclusiveOwnerIDRebindsOnOwnerRejoin' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Optionally persist owned-ground Leave/reclaim deletion into the FileStore once operators choose crash-vs-graceful-leave durability semantics.
2. Keep SQL import/backfill deferred.
3. Keep party-shaped owner-delivery deferred.
4. Docs-only: name empire anti-flags beside the already-enforced pickup restriction matrix if QA still reads that list as incomplete.

## Status

Shipped: rematerialized exclusive pending ground handles rebind process-local `OwnerID` to the fresh shared-world entity id on matching owner `Join`, while peers stay fail-closed mid-window and the durable FileStore snapshot continues to omit `OwnerID`. Graceful Leave→FileStore deletion, SQL import/backfill, and party-shaped owner-delivery stay deferred.
