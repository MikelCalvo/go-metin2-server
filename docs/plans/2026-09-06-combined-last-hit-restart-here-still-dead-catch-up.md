# Combined Last-Hit `/restart_here` Still-Dead Catch-Up — 2026-09-06

## Objective

Prove the already-owned `/restart_here` static-actor catch-up on the combined
last-hit: after one accepted practice-mob normal `ATTACK` kills the dummy and
floors the engaged owner, same-socket recovery while the dummy is still inside
its server-owned dead interval must replay trailing `GC DEAD(dummy_vid)` instead
of silently presenting a live dummy.

## Why now

- Combined last-hit already owns dummy death first, then the owner-floor suffix
  (`TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerEmitsCombinedDeathBurst`).
- Combined last-hit clocks are pinned before the dummy respawn delay so later
  watcher flushes do not rebuild the dummy early.
- `/restart_here` already refreshes currently visible static actors with
  delete-plus-state frames, and that encoder already appends trailing `DEAD`
  while HP is `0`.
- Existing `/restart_here` proofs cover a still-live dummy, or a dummy whose
  respawn is already due. Fresh EnterGame already replays still-dead trailing
  `DEAD`. The same-socket recovery path after combined last-hit does not.

## Contract owned by this slice

1. Combined last-hit still emits dummy death first, then the owner-floor suffix.
2. Same-socket `/restart_here` before the dummy respawn delay expires returns:
   - ordinary self bootstrap (`CHARACTER_ADD` -> `CHAR_ADDITIONAL_INFO` ->
     `CHARACTER_UPDATE` -> `PLAYER_POINT_CHANGE` at race create MaxHP)
   - then one still-dead dummy catch-up:
     `CHARACTER_DEL(dummy_vid)` -> add/info/update -> trailing `GC DEAD(dummy_vid)`
3. Fresh `TARGET` / `ATTACK` against that dummy fail closed until the dummy
   respawns.
4. After the owned respawn delay, the recovered owner receives the ordinary
   live rebuild (`CHARACTER_DEL` + add/info/update, no trailing `DEAD`) and a
   later fresh `TARGET` succeeds at full HP.
5. A visible live peer still receives only the owner alive-again refresh from
   `/restart_here`; dummy still-dead catch-up stays self-only for the recovered
   owner.

## Focused coverage

- `TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerRestartHereCatchesUpStillDeadDummy`

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerRestartHereCatchesUpStillDeadDummy$' -count=1
```

## What this is not yet

- kill-reward rematerialize after `/phase_select` / reconnect (Leave still
  deletes currently owned ground handles)

Reconnect / `/phase_select` still-dead dummy catch-up after combined last-hit
is now owned by
`2026-09-07-combined-last-hit-phase-select-reconnect-still-dead-catch-up.md`.

Kill-reward ground-item catch-up on `/restart_here` is now owned by
`2026-09-06-combined-last-hit-restart-here-ground-catch-up.md`.
`/restart_town` still-dead dummy teardown plus source-map relocate rematerialize
is now owned by
`2026-09-06-combined-last-hit-restart-town-ground-catch-up.md`.
