# Combined Last-Hit `/phase_select` / Reconnect `/restart_town` Ground Catch-Up — 2026-09-07

## Objective

Close the remaining playable kill → drop → floor-leave → town-return gap on
the combined last-hit: after one accepted practice-mob normal `ATTACK` kills
the dummy, registers an authored drop, and floors the engaged owner,
`/phase_select` or abrupt reconnect must park that pending source-map handle
instead of deleting it. Later `/restart_town` keeps the parked handle
registered, tears dummy occupancy and ground visibility down from the town
map, and rematerializes still-dead dummy trailing `GC DEAD` plus
`ITEM_GROUND_ADD` + `ITEM_OWNERSHIP` only after source-map relocate-back so
ordinary owner pickup succeeds while the dummy is still dead.

## Why now

- Combined last-hit `/restart_town` already rematerializes the pending drop
  after source-map relocate-back without an intervening Leave
  (`TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerRestartTownRematerializesKillRewardDropOnSourceMapReselect`).
- Combined last-hit `/phase_select` / reconnect already park the handle and
  rematerialize it through `/restart_here`
  (`TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerPhaseSelectRestartHereRematerializesKillRewardDrop`,
  `TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerReconnectRestartHereRematerializesKillRewardDrop`).
- Existing `/phase_select` / reconnect `/restart_town` proofs cover a
  still-live dummy with no kill-reward drop. The parked last-hit town twin
  does not.

## Contract owned by this slice

1. Combined last-hit still emits dummy death, then EXP/gold/ground/ownership,
   then the owner-floor suffix.
2. Same-socket `/phase_select` or abrupt reconnect while the dummy is still
   dead parks the pending kill-reward handle instead of deleting it.
3. Still-dead re-entry skips dummy add/info/update, dummy `DEAD`, and ground
   rematerialize because the owner is still at the zero-HP recipient skip.
4. Later `/restart_town` before the dummy respawn delay expires returns:
   - ordinary self bootstrap at the owned empire town-return position
   - then ordinary source-map transfer teardown:
     source peer `CHARACTER_DEL`, still-dead dummy occupancy `CHARACTER_DEL`,
     and source kill-reward `ITEM_GROUND_DEL`
   - no town-map dummy `DEAD` replay and no town-map `ITEM_GROUND_ADD` /
     `ITEM_OWNERSHIP` rematerialize
5. Relocate-back into source-map visibility rematerializes the still-dead
   dummy plus the same parked kill-reward handle, then ordinary owner
   `ITEM_PICKUP` succeeds while the dummy is still dead.

## Focused coverage

- `TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerPhaseSelectRestartTownRematerializesKillRewardDropOnSourceMapReselect`
- `TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerReconnectRestartTownRematerializesKillRewardDropOnSourceMapReselect`

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwner(PhaseSelect|Reconnect)RestartTownRematerializesKillRewardDropOnSourceMapReselect$' -count=1
```

## What this is not yet

- party share, random loot tables, or level-up choreography

Follow-up landed in
`docs/plans/2026-09-07-combined-last-hit-daemon-restart-kill-reward-rematerialize.md`:
combined last-hit `gamed` process restart rematerializes the still-dead dummy
plus parked exclusive handle, skips dummy/ground rematerialize while the
owner is still at the floor, then `/restart_here` catch-up lets ordinary
owner pickup succeed.
