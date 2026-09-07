# MOVE / SYNC_POSITION Transfer-Trigger Homeward After Chase — 2026-09-07

## Objective

Close the remaining client-origin exact-position transfer coverage gap beside
operator relocate and authored warp `INTERACT`: after a hit-armed chase has
left an authored practice mob `within_radius`, a player-driven `MOVE` or
`SYNC_POSITION` transfer trigger must release that engagement, clear the
pending chase deadline, and arm the already-owned homeward recovery.

## Status

Done for bootstrap scope as ordinary GREEN twin coverage:

- `TestGameRuntimeMoveTransferTriggerClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace`
- `TestGameRuntimeSyncPositionTransferTriggerClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace`

## Why now

- `MOVE`, `SYNC_POSITION`, and authored warp `INTERACT` all call the existing
  `applySelectedCharacterTransfer(..., rebootstrap = true)` helper.
- Operator `RelocateCharacter` and warp `INTERACT` already own focused
  chase/homeward coverage, but a regression at either position-packet ingress
  could have skipped that helper's engagement snapshot/re-sync behavior.
- The client-visible transfer contract is already frozen: an accepted trigger
  returns the ordinary self rebootstrap response, including one `TARGET(0, 0)`
  clear for the abandoned selected target.

## Focused coverage

```bash
go test ./internal/minimal -run 'TestGameRuntime(MoveTransferTrigger|SyncPositionTransferTrigger)ClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace$' -count=1
```

Each twin proves:

1. An accepted hit arms chase, the due chase beat moves the actor from authored
   home `(1700,2800)` to `(1800,2800)`, and leash state is `within_radius`.
2. The packet-specific exact-position trigger transfers the owner from map `42`
   to `(43,5000,5000)` and returns self `TARGET(0, 0)` in the rebootstrap
   response.
3. The abandoned actor loses its pending chase inspection row and gains one
   within-radius homeward deadline.
4. After the owned one-second delay, a retained watcher receives one `MOVE`
   back to authored home and the actor is `at_home` with no remaining homeward
   deadline.

## What this is not

- absolute chase / return / homeward due-at rematerialization across daemon
  restart (cancelled for Track A bootstrap as re-arm-from-now)
- cross-map mob `MOVE` / `GC WARP`, pack AI, pathfinding, or target switching
- a new transfer scheduler or position-packet protocol
- any new RED: this only proves already-live shared helper behavior at both
  remaining client ingress paths
