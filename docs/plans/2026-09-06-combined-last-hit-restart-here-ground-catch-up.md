# Combined Last-Hit `/restart_here` Ground Catch-Up — 2026-09-06

## Objective

Close the remaining playable kill → drop → recover gap on the combined last-hit:
after one accepted practice-mob normal `ATTACK` kills the dummy, registers an
authored drop, and floors the engaged owner, same-socket `/restart_here` must
rematerialize that still-pending ground handle and let ordinary owner pickup
succeed while the dummy is still dead.

## Why now

- Combined last-hit already owns dummy death first, then reward frames, then
  the owner-floor suffix
  (`TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerEmitsRewardsBeforeOwnerFloor`).
- Combined last-hit `/restart_here` already catch-up-refreshes the still-dead
  dummy with trailing `GC DEAD`
  (`TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerRestartHereCatchesUpStillDeadDummy`).
- EnterGame / transfer already rematerialize pending ground handles with
  `ITEM_GROUND_ADD` + `ITEM_OWNERSHIP` via `VisibleGroundItemFrames`.
- Same-socket `/restart_here` rebuilt static/mob visibility but skipped that
  ground carrier, so the recovered owner could not see or pick up the drop
  that the killing hit had already registered.

## Contract owned by this slice

1. Combined last-hit still emits dummy death, then EXP/gold/ground/ownership,
   then the owner-floor suffix.
2. Same-socket `/restart_here` before the dummy respawn delay expires returns:
   - ordinary self bootstrap (`CHARACTER_ADD` -> `CHAR_ADDITIONAL_INFO` ->
     `CHARACTER_UPDATE` -> `PLAYER_POINT_CHANGE` at race create MaxHP)
   - then one still-dead dummy catch-up:
     `CHARACTER_DEL(dummy_vid)` -> add/info/update -> trailing `GC DEAD(dummy_vid)`
   - then one self-only kill-reward ground rematerialize:
     `ITEM_GROUND_ADD` + `ITEM_OWNERSHIP` for the same pending handle
3. Fresh `TARGET` / `ATTACK` against that dummy still fail closed until the
   dummy respawns.
4. Ordinary owner `ITEM_PICKUP` of the rematerialized handle succeeds while
   the dummy is still dead (`ITEM_GROUND_DEL` + `ITEM_SET` + `ITEM_GET`) and
   persists recovered MaxHP plus the picked inventory.
5. A visible live peer still receives only the owner alive-again refresh from
   `/restart_here`; dummy still-dead catch-up and ground rematerialize stay
   self-only for the recovered owner. Pickup still queues one peer
   `ITEM_GROUND_DEL`.

## Focused coverage

- `TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerRestartHereRematerializesKillRewardDrop`

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerRestartHereRematerializesKillRewardDrop$' -count=1
```

## What this is not yet

- party share, random loot tables, or level-up choreography

Reconnect / `/phase_select` still-dead dummy catch-up after combined last-hit
is now owned by
`2026-09-07-combined-last-hit-phase-select-reconnect-still-dead-catch-up.md`.

Combined last-hit `/phase_select` / reconnect kill-reward rematerialize is now
owned by
`2026-09-07-combined-last-hit-phase-select-reconnect-kill-reward-rematerialize.md`.

`/restart_town` still-dead dummy teardown plus source-map relocate rematerialize
is now owned by
[combined last-hit `/restart_town` ground catch-up](2026-09-06-combined-last-hit-restart-town-ground-catch-up.md).
