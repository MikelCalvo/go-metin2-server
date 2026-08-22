# Proximity Suppress After Death-Floor `/restart_here` — 2026-08-22

## Objective

Prove that owner death-floor engagement release seeds the same leave/re-enter
proximity suppress already owned by `TARGET(0)` and mob death/respawn, so a
same-socket `/restart_here` while still inside aggro radius does not instantly
re-lock the still-live practice mob.

## Contract frozen by this slice

1. Proximity-armed delayed retaliation may reach owner HP `0` without inventing
   selected-target ownership.
2. That floor releases aggro-lite engagement and seeds proximity suppress for the
   still-inside owner (via the existing subject clear / suppress-mark path).
3. Accepted `/restart_here` while still inside `DefaultSpawnAggroRadius` keeps the
   recovered owner suppressed through later pending-frame flushes / delayed
   retaliation windows.
4. Only an explicit leave of aggro radius `200` and re-enter clears suppress and
   allows fresh proximity acquisition.

## Focused coverage

- `TestGameRuntimeProximityAggroSuppressesReacquireUntilLeaveAndReenterAfterOwnerDeathFloorRestartHere`

```bash
go test ./internal/minimal -run 'TestGameRuntimeProximityAggroSuppressesReacquireUntilLeaveAndReenterAfterOwnerDeathFloorRestartHere$' -count=1
```

## What this is not yet

- broader corpse / revive menus
- skill / ranged / PvP runtime policy
- weighted/random loot tables
- inventing a second suppress store beyond the existing leave/re-enter set
