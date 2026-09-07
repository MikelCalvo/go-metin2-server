# Combined Last-Hit Daemon-Restart Kill-Reward Rematerialize — 2026-09-07

## Objective

Close the remaining playable kill → drop → crash → recover gap on the combined
last-hit: after one accepted practice-mob normal `ATTACK` kills the dummy,
registers an authored drop, and floors the engaged owner, a `gamed` process
restart must rematerialize the still-dead dummy plus the pending exclusive
handle from FileStore. Fresh EnterGame while the owner is still at the floor
must skip dummy and ground rematerialize, then `/restart_here` must catch up
still-dead trailing `GC DEAD` plus self-only `ITEM_GROUND_ADD` +
`ITEM_OWNERSHIP` so ordinary owner pickup succeeds while the dummy is still
dead.

## Why now

- Combined last-hit `/phase_select` / reconnect already park the handle and
  rematerialize it through `/restart_here` without process restart
  (`TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerPhaseSelectRestartHereRematerializesKillRewardDrop`,
  `TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerReconnectRestartHereRematerializesKillRewardDrop`).
- Combined last-hit `/restart_town` after those parked Leave paths rematerializes
  on source-map relocate-back
  (`TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerPhaseSelectRestartTownRematerializesKillRewardDropOnSourceMapReselect`,
  `TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerReconnectRestartTownRematerializesKillRewardDropOnSourceMapReselect`).
- Live-owner kill-reward FileStore rematerialize already exists, but that proof
  never floors the owner or keeps the dummy still-dead
  (`TestGameRuntimePracticeMobKillRewardDropRematerializesAcrossDaemonRestart`).
- Still-dead spawn-group timer persistence and owner-floor rematerialize already
  exist on separate seams
  (`TestGameRuntimeContentSpawnGroupStillDeadPersistsAcrossDaemonRestart`,
  `TestGameRuntimePlayerDeathFloorRematerializesAcrossDaemonRestart`).
- The parked last-hit town-return plan still named this crash/restart twin as
  not-yet.

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
7. Same-socket `/restart_here` after that restart, before the dummy respawn
   delay expires, returns:
   - ordinary self bootstrap at race create MaxHP
   - then one still-dead dummy catch-up:
     `CHARACTER_DEL(dummy_vid)` -> add/info/update -> trailing `GC DEAD(dummy_vid)`
   - then one self-only kill-reward ground rematerialize:
     `ITEM_GROUND_ADD` + `ITEM_OWNERSHIP` for the same pending handle
8. Ordinary owner `ITEM_PICKUP` of the rematerialized handle succeeds while
   the dummy is still dead (`ITEM_GROUND_DEL` + `ITEM_SET` + `ITEM_GET`) and
   persists recovered MaxHP plus the picked inventory.

## Focused coverage

- `TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerDaemonRestartRestartHereRematerializesKillRewardDrop`

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerDaemonRestartRestartHereRematerializesKillRewardDrop$' -count=1
```

## What this is not yet

- party share, random loot tables, or level-up choreography
- inverting live-owner Leave deletion for ordinary player drops
- a second ownership model besides FileStore `OwnerID = 0` parking + Join rebind
