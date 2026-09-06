# Combined Last-Hit `/restart_town` Ground Catch-Up — 2026-09-06

## Objective

Close the remaining playable kill → drop → town-return gap on the combined
last-hit: after one accepted practice-mob normal `ATTACK` kills the dummy,
registers an authored drop, and floors the engaged owner, same-socket
`/restart_town` must keep that still-pending source-map handle registered,
tear it down from the recovered owner's town visibility, and rematerialize it
only after relocate-back into source-map visibility so ordinary owner pickup
succeeds while the dummy is still dead.

## Why now

- Combined last-hit already owns dummy death first, then reward frames, then
  the owner-floor suffix
  (`TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerEmitsRewardsBeforeOwnerFloor`).
- Combined last-hit `/restart_here` already rematerializes the pending drop
  in place
  (`TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerRestartHereRematerializesKillRewardDrop`).
- `/restart_town` is a transfer, not Leave, so owned source-map ground handles
  must survive the town-return instead of being deleted by owner-leave cleanup.
- Existing `/restart_town` source-map reselect proofs cover a still-live dummy
  with no kill-reward drop. The combined last-hit town twin does not.

## Contract owned by this slice

1. Combined last-hit still emits dummy death, then EXP/gold/ground/ownership,
   then the owner-floor suffix.
2. Same-socket `/restart_town` before the dummy respawn delay expires returns:
   - ordinary self bootstrap at the owned empire town-return position
     (`CHARACTER_ADD` -> `CHAR_ADDITIONAL_INFO` -> `CHARACTER_UPDATE` ->
     `PLAYER_POINT_CHANGE` at race create MaxHP)
   - then ordinary source-map transfer teardown:
     source peer `CHARACTER_DEL`, still-dead dummy `CHARACTER_DEL`, and
     source kill-reward `ITEM_GROUND_DEL`
   - no town-map dummy `DEAD` replay and no town-map `ITEM_GROUND_ADD` /
     `ITEM_OWNERSHIP` rematerialize, because the recovered owner left that
     source visibility
3. The pending source-map handle stays registered after that transfer. Town-side
   `TARGET` / `ATTACK` / `ITEM_PICKUP` against the source dummy or drop fail
   closed until relocate-back.
4. Relocate-back into source-map visibility rematerializes:
   - ordinary source peer add/info/update
   - still-dead dummy add/info/update plus trailing `GC DEAD(dummy_vid)`
   - self-only kill-reward `ITEM_GROUND_ADD` + `ITEM_OWNERSHIP` for the same
     pending handle
5. Fresh `TARGET` / `ATTACK` against that dummy still fail closed until the
   dummy respawns.
6. Ordinary owner `ITEM_PICKUP` of the rematerialized handle succeeds while
   the dummy is still dead (`ITEM_GROUND_DEL` + `ITEM_SET` + `ITEM_GET`) and
   persists recovered MaxHP plus the picked inventory at the source-map
   relocate coordinates.
7. A visible live source peer still receives only the owner delete on
   `/restart_town` and the ordinary owner re-entry on relocate-back; dummy
   still-dead rematerialize and ground rematerialize stay self-only for the
   recovered owner. Pickup still queues one peer `ITEM_GROUND_DEL`.

## Focused coverage

- `TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerRestartTownRematerializesKillRewardDropOnSourceMapReselect`

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerRestartTownRematerializesKillRewardDropOnSourceMapReselect$' -count=1
```

## What this is not yet

- reconnect / `/phase_select` still-dead dummy catch-up or kill-reward
  rematerialize after combined last-hit (Leave still deletes currently owned
  ground handles)
- party share, random loot tables, or level-up choreography
