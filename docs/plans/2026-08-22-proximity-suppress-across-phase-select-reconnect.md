# Proximity Suppress Across `/phase_select` / Reconnect — 2026-08-22

## Objective

Prove that proximity leave/re-enter suppress seeded by owner death-floor release
survives Leave → fresh Join identity changes (`/phase_select` and abrupt
disconnect/reconnect) so a later same-socket `/restart_here` while still inside
aggro radius does not instantly re-lock the still-live practice mob.

## Contract frozen by this slice

1. Live proximity suppress stays keyed by subject entity ID (no permanent
   name/VID-keyed suppress store).
2. On Leave / stale reclaim, subject suppress membership is detached from the
   departing entity ID and parked under the character VID when known.
3. On Join, pending suppress parked under that VID rematerializes onto the new
   subject entity ID.
4. Actor-side suppress clear also drops pending VID park entries for that actor
   (anti-leak).
5. Accepted `/restart_here` after `/phase_select` or reconnect, while still inside
   `DefaultSpawnAggroRadius`, keeps the recovered owner suppressed until an
   explicit leave/re-enter of the effective aggro radius.
6. Transfer / same-entity-ID paths are unchanged; content-bundle replacement and
   daemon restart continue to fail-close suppress (not remapped).

## Focused coverage

- `TestGameRuntimeProximityAggroSuppressesReacquireUntilLeaveAndReenterAfterOwnerDeathFloorPhaseSelectRestartHere`
- `TestGameRuntimeProximityAggroSuppressesReacquireUntilLeaveAndReenterAfterOwnerDeathFloorReconnectRestartHere`

```bash
go test ./internal/minimal -run 'TestGameRuntimeProximityAggroSuppressesReacquireUntilLeaveAndReenterAfterOwnerDeathFloor(PhaseSelect|Reconnect)RestartHere$' -count=1
```

## What this is not yet

- inventing cross-map return MOVE / `GC WARP` choreography (frozen as delete/readd /
  direct-home rebuild in `spawn-leash-bootstrap.md`)
- inventing a second permanent suppress store keyed by name/VID
- remapping suppress across content-bundle replacement or daemon restart
- broader corpse / revive menus
