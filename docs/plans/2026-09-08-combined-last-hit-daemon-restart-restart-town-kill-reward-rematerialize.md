# Combined Last-Hit Daemon-Restart `/restart_town` Kill-Reward Rematerialize — 2026-09-08

## Objective

Close the remaining playable kill → drop → crash → town-return gap on the
combined last-hit: after one accepted practice-mob normal `ATTACK` kills the
dummy, registers an authored drop, and floors the engaged owner, a `gamed`
process restart must rematerialize the still-dead dummy plus the pending
exclusive handle from FileStore. Fresh EnterGame while the owner is still at
the floor must skip dummy and ground rematerialize, then `/restart_town` must
keep that parked source-map handle registered, tear dummy occupancy and ground
visibility down from the town map, and rematerialize still-dead dummy trailing
`GC DEAD` plus `ITEM_GROUND_ADD` + `ITEM_OWNERSHIP` only after source-map
relocate-back so ordinary owner pickup succeeds while the dummy is still dead.

## Why now

- Combined last-hit `gamed` process restart already rematerializes the
  still-dead dummy plus parked exclusive handle through `/restart_here`
  (`TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerDaemonRestartRestartHereRematerializesKillRewardDrop`).
- Combined last-hit `/restart_town` already rematerializes that pending drop
  after source-map relocate-back without process restart, including the
  `/phase_select` and reconnect Leave twins
  (`TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerRestartTownRematerializesKillRewardDropOnSourceMapReselect`,
  `TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerPhaseSelectRestartTownRematerializesKillRewardDropOnSourceMapReselect`,
  `TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerReconnectRestartTownRematerializesKillRewardDropOnSourceMapReselect`).
- The parked last-hit crash/restart plan still named `/restart_here` as the
  only post-restart recovery path. Town return after FileStore rematerialize
  is the missing composition.

## Contract owned by this slice

1. Combined last-hit still emits dummy death, then EXP/gold/ground/ownership,
   then the owner-floor suffix. Ground registration still happens against the
   live killer snapshot and persists the exclusive handle to the dedicated
   ground-item FileStore.
2. The accepted dummy death also persists spawn-backed `combat_current_hp=0`
   plus absolute `respawn_ready_at` so process restart keeps that authored
   `spawn_group_ref` still-dead through the remaining delay.
3. The owner-floor suffix still persists points[1]=0. A rebuilt runtime from
   the same FileStore paths rematerializes that dead snapshot on EnterGame.
4. Crash/restart rematerializes the pending exclusive handle with
   `OwnerID = 0`, absolute ownership/despawn timers intact, and durable owner
   identity intact. Matching owner `Join` rebinds process-local `OwnerID`.
   Living peers stay fail-closed while exclusive ownership is active.
5. Still-dead owner EnterGame after that restart rebuilds the persisted floor
   and skips dummy add/info/update, dummy `DEAD`, and ground rematerialize
   because the owner is still at the zero-HP recipient skip.
6. A living visible peer that EnterGames the rematerialized world first
   receives ordinary still-dead dummy add/info/update plus trailing
   `GC DEAD(dummy_vid)`, then exclusive `ITEM_GROUND_ADD` + `ITEM_OWNERSHIP`
   for the parked handle. Pickup stays fail-closed for that peer while
   exclusive ownership is active. The later still-dead owner re-entry still
   skips dummy and ground rematerialize.
7. Same-socket `/restart_town` after that restart, before the dummy respawn
   delay expires, returns:
   - ordinary self bootstrap at the owned empire town-return position
   - then ordinary source-map transfer teardown:
     source peer `CHARACTER_DEL`, still-dead dummy occupancy `CHARACTER_DEL`,
     and source kill-reward `ITEM_GROUND_DEL`
   - no town-map dummy `DEAD` replay and no town-map `ITEM_GROUND_ADD` /
     `ITEM_OWNERSHIP` rematerialize
8. Relocate-back into source-map visibility rematerializes the still-dead
   dummy plus the same parked kill-reward handle, then ordinary owner
   `ITEM_PICKUP` succeeds while the dummy is still dead.

## Focused coverage

- `TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerDaemonRestartRestartTownRematerializesKillRewardDropOnSourceMapReselect`

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerDaemonRestartRestartTownRematerializesKillRewardDropOnSourceMapReselect$' -count=1
```

## What this is not yet

- party share, random loot tables, or level-up choreography
- inverting live-owner Leave deletion for ordinary player drops
- a second ownership model besides FileStore `OwnerID = 0` parking + Join rebind
