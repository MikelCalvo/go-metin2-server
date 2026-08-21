# Reward Drop Ownership Public Release — 2026-08-21

## Objective

Freeze that kill-reward ground handles reuse the already-owned bootstrap exclusive-ownership timer, blank public `GC::ITEM_OWNERSHIP` release, and ordinary collector pickup path from `item-drop-pickup-bootstrap.md`, instead of leaving the reward spec claiming ownership expiry / public loot release are unowned.

## Contract frozen by this slice

1. Reward ground registration stamps the same in-memory exclusive ownership window (`30` seconds) and destroy deadline (`300` seconds) used by player drops.
2. While exclusive ownership is active, a living non-owner visible collector's `ITEM_PICKUP` fails closed with no frames and leaves the reward handle pending.
3. After the exclusive ownership timer elapses, living visible sessions receive one blank `GC::ITEM_OWNERSHIP`, and the same collector may reclaim the reward through ordinary collector-side pickup (`ITEM_GROUND_DEL`, `ITEM_SET`, normal/self `ITEM_GET`).
4. Spec/QA name this shared ownership/public-release contract for reward drops beside player drops; restart-restored ownership/despawn timer state remains deferred.

## What this is not yet

- restart-restored ownership / despawn timers from quarantined `0010` exports
- party-shaped owner-delivery notices / real party membership
- corpse interaction beyond the shared ground-item ownership path
- weighted/random loot tables

## TDD and validation

Focused coverage:

- `go test ./internal/minimal -run 'RewardDropPublicReleaseAllowsLivingCollectorPickup|ItemPickupAllowsPublicCollectorAfterOwnershipRelease' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep restart-restored ownership/despawn timers deferred until operators decide quarantined `0010` exports should drive recovery.
2. Optional commit-time exchange busy reject chat is now owned; keep accepted safebox password/load/placement/money deferred.
3. Keep partner-side open player-shop / cube busy-window exchange rejects deferred until those presentation seams exist.
