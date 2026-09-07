# Combined Last-Hit `/phase_select` / Reconnect Kill-Reward Rematerialize — 2026-09-07

## Objective

Close the remaining playable kill → drop → leave → recover gap on the combined
last-hit: after one accepted practice-mob normal `ATTACK` kills the dummy,
registers an authored drop, and floors the engaged owner, same-socket
`/phase_select` re-entry and abrupt reconnect must keep that pending exclusive
handle in the world, then `/restart_here` must rematerialize it with self-only
`ITEM_GROUND_ADD` + `ITEM_OWNERSHIP` so ordinary owner pickup succeeds while the
dummy is still dead.

## Why now

- Combined last-hit already owns dummy death first, then reward frames, then
  the owner-floor suffix
  (`TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerEmitsRewardsBeforeOwnerFloor`).
- Same-socket `/restart_here` already rematerializes a still-pending kill-reward
  handle after still-dead dummy catch-up
  (`TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerRestartHereRematerializesKillRewardDrop`).
- `/restart_town` already keeps the source-map handle registered across transfer
  (not Leave) and rematerializes on relocate-back
  (`TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerRestartTownRematerializesKillRewardDropOnSourceMapReselect`).
- Combined last-hit `/phase_select` / reconnect already keep the still-dead
  dummy skipped while the owner is at the floor, then reuse `/restart_here`
  dummy catch-up
  (`TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerPhaseSelectRestartHereCatchesUpStillDeadDummy`,
  `TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerReconnectRestartHereCatchesUpStillDeadDummy`).
- Daemon restart already parks exclusive handles at `OwnerID = 0` and rebinds
  on matching owner `Join`
  (`TestGameRuntimePracticeMobKillRewardDropRematerializesAcrossDaemonRestart`).
- Graceful shared-world `Leave` still deletes `OwnerID`-matched live handles
  (`docs/plans/2026-08-22-ground-item-leave-persist-owned-deletion.md`), so the
  drop vanishes on `/phase_select` / reconnect even though dummy catch-up now
  works.

## Contract owned by this slice

1. Combined last-hit still emits dummy death, then EXP/gold/ground/ownership,
   then the owner-floor suffix. Ground registration still happens against the
   live killer snapshot.
2. When that floored owner later leaves the shared world (`/phase_select`,
   abrupt close, or stale reclaim of a last-known floor snapshot), currently
   `OwnerID`-matched pending ground handles are **parked**, not deleted:
   - live `GroundItemExists(vid)` stays true
   - process-local `OwnerID` is cleared to `0`
   - durable owner identity, exclusive window, and despawn timers stay intact
   - living visible peers receive the ordinary owner `CHARACTER_DEL` and do
     **not** receive `ITEM_GROUND_DEL` for the parked handle
3. Live (above-floor) owner `Leave` / stale reclaim still delete
   `OwnerID`-matched handles with `ITEM_GROUND_DEL` fanout and FileStore
   persist, as already frozen by the items-lane leave-deletion contract.
4. Same-socket `/phase_select` → `SELECT` / `ENTERGAME` and abrupt reconnect
   before the dummy respawn delay expires rebuild the persisted owner floor:
   - ordinary self bootstrap plus self `PLAYER_POINT_CHANGE` at `0` and self
     `GC DEAD(owner_vid)`
   - living visible peer entry
   - no dummy add/info/update, no dummy `DEAD`, and no ground rematerialize,
     because the owner is still at the zero-HP recipient skip
5. Matching owner `Join` rebinds process-local `OwnerID` onto parked exclusive
   handles via the existing identity-keyed daemon-restart seam. Peers stay
   fail-closed while exclusive ownership is active.
6. Same-socket `/restart_here` after either re-entry returns:
   - ordinary self bootstrap at race create MaxHP
   - then one still-dead dummy catch-up:
     `CHARACTER_DEL(dummy_vid)` -> add/info/update -> trailing `GC DEAD(dummy_vid)`
   - then one self-only kill-reward ground rematerialize:
     `ITEM_GROUND_ADD` + `ITEM_OWNERSHIP` for the same pending handle
7. Ordinary owner `ITEM_PICKUP` of the rematerialized handle succeeds while
   the dummy is still dead (`ITEM_GROUND_DEL` + `ITEM_SET` + `ITEM_GET`) and
   persists recovered MaxHP plus the picked inventory.
8. A visible live peer still receives owner leave / re-entry / alive-again
   frames only; dummy still-dead catch-up and ground rematerialize stay
   self-only for the recovered owner. Pickup still queues one peer
   `ITEM_GROUND_DEL`.

## Focused coverage

- `TestSharedWorldRegistryLeaveAtHPFloorParksOwnedGroundItems`
- `TestSharedWorldRegistryJoinReclaimAtHPFloorParksOwnedGroundItems`
- `TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerPhaseSelectRestartHereRematerializesKillRewardDrop`
- `TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerReconnectRestartHereRematerializesKillRewardDrop`

```bash
go test ./internal/minimal -run 'TestSharedWorldRegistry(LeaveAtHPFloor|JoinReclaimAtHPFloor)ParksOwnedGroundItems$|TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwner(PhaseSelect|Reconnect)RestartHereRematerializesKillRewardDrop$' -count=1
```

## What this is not yet

- inverting live-owner Leave deletion for ordinary player drops
- party share, random loot tables, or level-up choreography
- a second ownership model besides daemon-restart `OwnerID = 0` parking + Join
  rebind
